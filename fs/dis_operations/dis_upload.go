package dis_operations

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/operations"
	rsync "github.com/rclone/rclone/fs/sync"
	"github.com/rclone/rclone/reedsolomon"
	"github.com/spf13/cobra"
)

func Dis_Upload(args []string, reSignal bool) error {
	absolutePath, err := dis_init(args[0])
	if err != nil {
		return err
	}

	originalFileName := filepath.Base(args[0])
	var distributedFileArray []DistributedFile
	hashedNamesMap := make(map[string]string)

	if reSignal {
		tempDistributedFileArray, err := GetDistributedFileStruct(originalFileName)
		if err != nil {
			return err
		}

		for _, dFile := range tempDistributedFileArray {
			if !dFile.Check {
				distributedFileArray = append(distributedFileArray, dFile)
				hashVal, err := CalculateHash(dFile.DistributedFile)
				if err != nil {
					return err
				}
				hashedNamesMap[dFile.DistributedFile] = hashVal
			}
		}
	} else {
		fmt.Printf("else name : %s\n", args[0])
		isDuplicate, err := DoesFileStructExist(originalFileName)
		fmt.Printf("isDuplicate: %t\n", isDuplicate)
		if err != nil {
			return err
		}

		if isDuplicate && !ShowDescription(originalFileName) {
			return nil
		}

		hashedNamesMap, distributedFileArray, err = prepareUpload(originalFileName, absolutePath)
		if err != nil {
			return err
		}
	}

	if err := startUploadFileGoroutine(originalFileName, hashedNamesMap, distributedFileArray); err != nil {
		return err
	}

	if err := ResetCheckFlag(originalFileName); err != nil {
		return err
	}

	fmt.Println("Completed Dis_Upload!")
	return nil
}

func createHashNames(distributedFileArray []DistributedFile) (hashNameMap map[string]string, errors []error) {
	hashNameMap = make(map[string]string)
	var errs []error
	for _, DFile := range distributedFileArray {
		hashedFileName, err := ConvertFileNameForUP(DFile.DistributedFile)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to convert file name %v", err))
			continue // Skip this iteration on error
		}

		hashNameMap[DFile.DistributedFile] = hashedFileName
	}
	return hashNameMap, errs
}

func prepareUpload(fileName, absolutePath string) (hashNameMap map[string]string, distributedFileInfos []DistributedFile, err error) {
	dis_names, checksums, shardSize, padding := reedsolomon.DoEncode(absolutePath)

	remotes := config.GetRemotes()

	err = MakeDistributionDir(remotes)
	if err != nil {
		return nil, nil, err
	}

	// get Distributed info	해야함
	for idx, source := range dis_names {
		dis_fileName := filepath.Base(source)

		remote, err := LoadBalancer_RoundRobin()
		if err != nil {
			return nil, nil, err
		}
		// Get the distributed info
		distributionFile, err := GetDistributedInfo(dis_fileName, Remote{remote.Name, remote.Type}, checksums[idx])
		if err != nil {
			return nil, nil, err
		}

		distributedFileInfos = append(distributedFileInfos, distributionFile)

	}

	hashNameMap, errs := createHashNames(distributedFileInfos)
	if len(errs) > 0 {
		return nil, nil, fmt.Errorf("errors occurred during hashing: %v", errs)
	}

	// Get the full path for the original file
	originalFileFullPath, err := getAbsolutePath(fileName)
	if err != nil {
		return nil, nil, err
	}

	err = MakeDataMap(originalFileFullPath, distributedFileInfos, shardSize, padding)
	if err != nil {
		return nil, nil, err
	}

	return hashNameMap, distributedFileInfos, nil
}

func startUploadFileGoroutine(originalFileName string, hashedFileNameMap map[string]string, distributedFileArray []DistributedFile) (err error) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	var errs []error
	dir := GetShardPath()

	for _, shardInfo := range distributedFileArray {
		wg.Add(1)

		go func(shardInfo DistributedFile) {
			defer wg.Done()
			dest := fmt.Sprintf("%s:%s", shardInfo.Remote.Name, remoteDirectory)
			fmt.Printf("Uploading file %s to %s\n", hashedFileNameMap[shardInfo.DistributedFile], dest)

			source := filepath.Join(dir, hashedFileNameMap[shardInfo.DistributedFile])

			err = remoteCallCopy([]string{source, dest})
			if err != nil {
				errs = append(errs, fmt.Errorf("error in remoteCallCopy for file %s: %w", source, err))
				return
			}

			err = UpdateDistributedFileCheckFlag(originalFileName, shardInfo.DistributedFile, true)
			if err != nil {
				fmt.Printf("UpdateDistributedFileCheckFlag 에러 : %v\n", err)
			}

			mu.Lock()
			// Erase Temp Shard
			reedsolomon.DeleteShardWithFileNames([]string{hashedFileNameMap[shardInfo.DistributedFile]})
			mu.Unlock()

		}(shardInfo)
	}

	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("errors occurred: %v", errs)
	}
	fmt.Println("NO errors occured!")
	return nil
}

func MakeDistributionDir(remotes []config.Remote) (err error) {
	var wg sync.WaitGroup
	var errs []error
	for _, remote := range remotes {
		argument := fmt.Sprintf("%s:%s", remote.Name, remoteDirectory)
		wg.Add(1)

		go func(arg string) {
			defer wg.Done()

			err := remoteCallMkdir([]string{arg})
			if err != nil {
				errs = append(errs, fmt.Errorf("error creating directory at %s: %w", arg, err))
				return
			}
		}(argument)
	}

	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("errors occurred: %v", errs)
	}

	return nil
}

func remoteCallCopy(args []string) (err error) {
	fmt.Printf("Calling remoteCallCopy with args: %v\n", args)

	// Fetch the copy command
	copyCommand := *copyCommandDefinition
	copyCommand.SetArgs(args)

	err = copyCommand.Execute()
	if err != nil {
		return fmt.Errorf("error executing copyCommand: %w", err)
	}

	return nil
}

var (
	createEmptySrcDirs = false
)

var copyCommandDefinition = &cobra.Command{
	Use: "copy source:path dest:path",
	Annotations: map[string]string{
		"groups": "Copy,Filter,Listing,Important",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(2, 2, command, args)
		fsrc, srcFileName, fdst := cmd.NewFsSrcFileDst(args)
		cmd.RunWithSustainOS(true, true, command, func() error {
			if srcFileName == "" {
				return rsync.CopyDir(context.Background(), fdst, fsrc, createEmptySrcDirs)
			}
			return operations.CopyFile(context.Background(), fdst, fsrc, srcFileName, srcFileName)
		}, true)
	},
}

func remoteCallMkdir(args []string) (err error) {
	fmt.Printf("Calling remoteCallMkdir with args: %v\n", args)

	// Fetch the copy command
	copyCommand := *mkdirCommandDefinition
	copyCommand.SetArgs(args)

	err = copyCommand.Execute()
	if err != nil {
		return fmt.Errorf("error executing mkdirCommand: %w", err)
	}

	return nil
}

var mkdirCommandDefinition = &cobra.Command{
	Use:   "mkdir remote:path",
	Short: `Make the path if it doesn't already exist.`,
	Annotations: map[string]string{
		"groups": "Important",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fdst := cmd.NewFsDir(args)
		if !fdst.Features().CanHaveEmptyDirectories && strings.Contains(fdst.Root(), "/") {
			fs.Logf(fdst, "Warning: running mkdir on a remote which can't have empty directories does nothing")
		}
		cmd.RunWithSustainOS(true, false, command, func() error {
			return operations.Mkdir(context.Background(), fdst, "")
		}, true)
	},
}

func dis_init(arg string) (string, error) {
	// Use the existing getAbsolutePath function to resolve the absolute path
	absolutePath, err := getAbsolutePath(arg)
	if err != nil {
		fmt.Println("Error resolving the absolute path:", err)
		return "", err
	}

	// Check if the file exists
	if _, err := os.Stat(absolutePath); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("File does not exist:", absolutePath)
			return "", fmt.Errorf("file does not exist: %s", absolutePath)
		}
		// Handle other errors (e.g., permission issues)
		fmt.Println("Error checking file:", err)
		return "", err
	}

	// If the file exists, print success message
	fmt.Println("Success: File found at", absolutePath)
	return absolutePath, nil
}
