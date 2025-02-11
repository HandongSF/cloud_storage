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

func Dis_Download(args []string, reSignal bool) (err error) {

	_, err = GetFileInfoStruct(args[0])
	if err != nil {
		return err
	}

	var fileNames []string
	var distributedFileInfos []DistributedFile

	if reSignal {
		//Get Distribution list(Check 읽어서 false인 것만 들고 오기)
		distributedFileInfos, err = GetUncompletedFileInfo(args[0])
		if err != nil {
			return err
		}

	} else {
		//state 변경
		err = UpdateFileFlag(args[0], "download")
		if err != nil {
			return err
		}
		distributedFileInfos, err = GetDistributedFileStruct(args[0])
		if err != nil {
			return err
		}
	}

	start := time.Now()
	startDownloadFileGoroutine(distributedFileInfos)
	elapsed := time.Since(start)
	fmt.Printf("Time taken for dis_download: %s\n", elapsed)

	absolutePath, err := getAbsolutePath(args[1])
	if err != nil {
		return err
	}

	// Move downloaded file to destination
	fileInfo, err := GetFileInfoStruct(args[0])
	if err != nil {
		return err
	}

	var checksums []string

	for _, each := range distributedFileInfos {
		checksums = append(checksums, each.Checksum)
	}

	err = reedsolomon.DoDecode(args[0], absolutePath, fileInfo.Padding, checksums)
	if err != nil {
		result := ShowDescription_RemoveFile(args[0])
		if result {
			err = Dis_rm([]string{args[0]}, false)
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

	// change Flag and Check to false
	err = ResetCheckFlag(args[0])
	if err != nil {
		return err
	}

	fmt.Printf("File successfully downloaded to %s\n", absolutePath)

	// Erase Temp Shard
	reedsolomon.DeleteShardWithFileNames(fileNames)

	return nil
}

func startDownloadFileGoroutine(distributedFileInfos []DistributedFile) (err error) {

	shardDir, err := reedsolomon.GetShardDir()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	for _, fileInfo := range distributedFileInfos {

		wg.Add(1)
		go func(fileInfo DistributedFile) {
			defer wg.Done()

			hashedFileName, err := CalculateHash(fileInfo.DistributedFile)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("CalculateHash for %s: %w", fileInfo.DistributedFile, err))
				mu.Unlock()
				return
			}

			source := fmt.Sprintf("%s:%s/%s", fileInfo.Remote.Name, remoteDirectory, hashedFileName)
			fmt.Printf("Downloading shard %s to %s\n", source, shardDir)
			downloadedFilePath := path.Join(shardDir, hashedFileName)

			if err := remoteCallCopyforDown([]string{source, shardDir}); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("remoteCallCopyforDown for %s: %w", fileInfo.DistributedFile, err))
				mu.Unlock()
				return
			}

			// Check if the file exists before renaming
			if _, err := os.Stat(downloadedFilePath); os.IsNotExist(err) {
				mu.Lock()
				errs = append(errs, fmt.Errorf("downloaded file %s does not exist", downloadedFilePath))
				mu.Unlock()
				return
			}

			if err := ConvertFileNameForDo(hashedFileName, fileInfo.DistributedFile); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("ConvertFileNameForDo for %s: %w", fileInfo.DistributedFile, err))
				mu.Unlock()
				return
			}

			if err := UpdateDistributedFileCheckFlag(fileInfo.DistributedFile, fileInfo.DistributedFile, true); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("UpdateDistributedFileCheckFlag for %s: %w", fileInfo.DistributedFile, err))
				mu.Unlock()
				return
			}

		}(fileInfo)
	}

	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("errors occurred during download: %v", errs)
	}

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
