package dis_operations

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/reedsolomon"
	"github.com/spf13/cobra"
)

func Dis_Download(args []string) (err error) {
	distributedFileNames, err := GetDistributedFile()
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

	for _, disFileStruct := range distributedFileInfos {
		source := fmt.Sprintf("%s:%s/%s", disFileStruct.Remote.Name, remoteDirectory, disFileStruct.DistributedFile)
		fmt.Printf("Downloading shard %s to %s of size %d\n", source, shardDir, disFileStruct.DisFileSize)

		wg.Add(1)
		go func(source, shardDir string) {
			defer wg.Done()
			if err := remoteCallCopyforDown([]string{source, shardDir}); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("error in remoteCallCopy for file %s: %v", source, err))
				mu.Unlock()
			}
		}(source, shardDir)
	}

	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("errors occurred during download: %v", errs)
	}

	// Send to erasure coding to recover
	modFileName := fmt.Sprintf("%s", args[0])

	absolutePath, err := getAbsolutePath(args[1])
	if err != nil {
		return err
	}

	// Move downloaded file to destination
	reedsolomon.DoDecode(modFileName, absolutePath)

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
