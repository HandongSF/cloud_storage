package dis_operations

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/rclone/rclone/reedsolomon"
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
			if err := remoteCallCopy([]string{source, shardDir}); err != nil {
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
	fileInfo, err := GetFileInfoStruct(args[0])
	if err != nil {
		return err
	}

	reedsolomon.DoDecode(modFileName, absolutePath, fileInfo.Padding)

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
