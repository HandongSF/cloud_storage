package dis_operations

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/reedsolomon"
	"github.com/spf13/cobra"
)

func Dis_Download(args []string) (err error) {
	distributedFileNames, err := Dis_ls()
	if err != nil {
		return err
	}

	if !contains(distributedFileNames, args[0]) {
		return fmt.Errorf("file not found in remote")
	}

	// Get Distribution list
	distributedFileInfos, err := GetDistributedFileStruct(args[0])
	if err != nil {
		return err
	}

	// Get shards  via API call
	shardDir, err := reedsolomon.GetShardDir()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	start := time.Now()
	for _, disFileStruct := range distributedFileInfos {
		hashedFileName, err := CalculateHash(disFileStruct.DistributedFile)
		if err != nil {
			errs = append(errs, fmt.Errorf("error in CalculateHash: %v", err))
		}
		source := fmt.Sprintf("%s:%s/%s", disFileStruct.Remote.Name, remoteDirectory, hashedFileName)
		fmt.Printf("Downloading shard %s to %s of size %d\n", source, shardDir, disFileStruct.DisFileSize)

		wg.Add(1)
		go func(source, shardDir, hashedFileName, originalFileName string) {
			defer wg.Done()
			if err := remoteCallCopyforDown([]string{source, shardDir}); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("error in remoteCallCopy for file %s: %v", source, err))
				mu.Unlock()
			}
			err := ConvertFileNameForDo(hashedFileName, originalFileName)
			if err != nil {
				errs = append(errs, fmt.Errorf("error in convertFileNameFordo: %v", err))
			}
		}(source, shardDir, hashedFileName, disFileStruct.DistributedFile)
	}

	wg.Wait()

	elapsed := time.Since(start)
	fmt.Printf("Time taken for dis_download: %s\n", elapsed)

	if len(errs) > 0 {
		return fmt.Errorf("errors occurred during download: %v", errs)
	}

	absolutePath, err := getAbsolutePath(args[1])
	if err != nil {
		return err
	}

	// Move downloaded file to destination
	fileInfo, err := GetFileInfoStruct(args[0])
	if err != nil {
		return err
	}

	disFileInfos, err := GetDistributedFileStruct(args[0])
	if err != nil {
		return err
	}

	var checksums []string

	for _, each := range disFileInfos {
		checksums = append(checksums, each.Checksum)
	}

	err = reedsolomon.DoDecode(args[0], absolutePath, fileInfo.Padding, checksums)
	if err != nil {
		result := ShowDescription_RemoveFile(args[0])
		if result {
			err = Dis_rm([]string{args[0]})
			if err != nil {
				return err
			}
		}
		return nil
	}

	//check checksum
	//if checksum error -> delete file
	filePathAndName := path.Join(absolutePath, args[0])
	newChecksum, err := calculateChecksum(filePathAndName)
	if err != nil {
		return err
	}
	if newChecksum == GetChecksum(args[0]) {
		fmt.Printf("checksum is same\n")
	} else {
		fmt.Printf("checksum is different\n")
		//delete file
		err = os.Remove(filePathAndName)
		if err != nil {
			fmt.Printf("Failed to delete file: %v\n", err)
			return
		}
		return fmt.Errorf("checksum is different! so can't download file")
	}

	// Need fix, not deleting shards correctly
	//reedsolomon.DeleteShardDir()

	fmt.Printf("File successfully downloaded to %s\n", absolutePath)

	return nil
}

func getAbsolutePath(arg string) (string, error) {
	// Check if the path is absolute
	if filepath.IsAbs(arg) {
		// Return the cleaned absolute path
		return filepath.Clean(arg), nil
	}

	// If it's not absolute, resolve relative to the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %v", err)
	}

	// Join and clean the path to get the absolute version
	destinationPath := filepath.Join(cwd, arg)
	return filepath.Clean(destinationPath), nil
}

func contains(slice []string, str string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
}

func remoteCallCopyforDown(args []string) (err error) {
	fmt.Printf("Calling remoteCallCopy with args: %v\n", args)

	// Fetch the copy command
	copyCommand := *copyCommandDefinitionForDown
	copyCommand.SetArgs(args)

	err = copyCommand.Execute()
	if err != nil {
		return fmt.Errorf("error executing copyCommand: %w", err)
	}

	return nil
}

var copyCommandDefinitionForDown = &cobra.Command{
	Use: "copy source:path dest:path",
	Annotations: map[string]string{
		"groups": "Copy,Filter,Listing,Important",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(2, 2, command, args)
		fsrc, srcFileName, fdst := cmd.NewFsSrcFileDst(args)
		cmd.RunWithSustainOS(true, true, command, func() error {
			if srcFileName == "" {
				fmt.Printf("%s is a directory or doesn't exist\n", args[0])
				return nil
			}
			return operations.CopyFile(context.Background(), fdst, fsrc, srcFileName, srcFileName)
		}, true)
	},
}
