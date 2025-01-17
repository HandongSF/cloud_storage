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

func Dis_Upload(args []string) (err error) {
	// Check if file exists
	err = dis_init(args[0])
	if err != nil {
		return err
	}

	dis_names, shardSize := reedsolomon.DoEncode(args[0])
	remotes := config.GetRemotes()
	distributedFileArray := make([]DistributedFile, len(dis_names))
	rr_counter := 0 // Round Robin

	err = MakeDistributionDir(remotes)
	if err != nil {
		return err
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	var errs []error

	for i, source := range dis_names {
		// Prepare destination for the file upload
		dest := fmt.Sprintf("%s:%s", remotes[rr_counter].Name, remoteDirectory)
		fmt.Printf("Uploading file %s to %s of size %d\n", source, dest, shardSize)

		wg.Add(1)

		go func(i int, source string, dest string) {
			defer wg.Done()

			// Perform the upload (Via API Call)
			err := remoteCallCopy([]string{source, dest})
			if err != nil {
				errs = append(errs, fmt.Errorf("error in Dis_Upload for file %s: %w", source, err))
				return
			}

			// Get the full path of the shard
			shardFullPath, err := GetFullPath(source)
			if err != nil {
				errs = append(errs, fmt.Errorf("error getting full path for %s: %w", source, err))
				return
			}

			// Get the distributed info for the shard
			distributionFile, err := GetDistributedInfo(shardFullPath, Remote{remotes[rr_counter].Name, remotes[rr_counter].Type})
			if err != nil {
				errs = append(errs, fmt.Errorf("error in GetDistributedInfo for %s: %w", source, err))
				return
			}

			mu.Lock()
			distributedFileArray[i] = distributionFile
			mu.Unlock()
		}(i, source, dest)

		mu.Lock()
		rr_counter = (rr_counter + 1) % len(remotes)
		mu.Unlock()
	}

	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("errors occurred: %v", errs)
	}

	// Get the full path for the original file
	originalFileFullPath, err := GetFullPath(args[0])
	if err != nil {
		return err
	}

	// Make the data map using the distributed files
	MakeDataMap(originalFileFullPath, distributedFileArray)
	fmt.Printf("Completed Dis_Upload!\n")
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

func GetFullPath(source string) (string, error) {
	// Get the current working directory
	currentDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %v", err)
	}

	// Join the current directory with the source file path
	fullPath := filepath.Join(currentDir, source)

	return fullPath, nil
}

func dis_init(arg string) (err error) {
	path, err := os.Getwd()
	if err != nil {
		fmt.Println("error to get current directory path: ", err)
		return err
	}

	fullPath := filepath.Join(path, arg)
	// 존재하지 않는 파일이라면 cmd창에 에러 메세지 출력
	fi, err := os.Open(fullPath)
	if err != nil {
		fmt.Println("file does not exit", err)
		return err
	}
	// 존재한다면 ok 메세지 cmd창에 출력
	fmt.Println("Success to find file : ", fi)

	// 해당 코드 필요 없을 수 있음. Reedsolomon에서 생성하는 shard file에 shard 생성
	// 유저가 현재 위치한 로컬 디렉토리에(path) 파일이름으로 디렉토리 생성
	//fileBase := strings.TrimSuffix(arg, filepath.Ext(arg))
	//dirPath := filepath.Join(path, fileBase+"_dir")

	//err = os.Mkdir(dirPath, 0755)
	//if err != nil {
	//	fmt.Println("Error creating directory: ", err)
	//	return err
	//}
	//fmt.Println("Directory created successfully at: ", dirPath)

	return nil
}
