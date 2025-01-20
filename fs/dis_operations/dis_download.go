package dis_operations

import (
	"fmt"
	"io"
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

	// Move downloaded file to destination
	outputPath := reedsolomon.DoDecode(modFileName)

	// Define the destination path (you may want to use args[1] or a specific destination)
	destinationPath := fmt.Sprintf("%s/%s", args[1], filepath.Base(outputPath))

	// Move the decoded file to the destination
	if err := moveFile(outputPath, destinationPath); err != nil {
		return fmt.Errorf("failed to move file from %s to %s: %v", outputPath, destinationPath, err)
	}

	fmt.Printf("File successfully moved to %s\n", destinationPath)

	return nil
}

func moveFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer output.Close()

	if _, err := io.Copy(output, input); err != nil {
		return err
	}

	// Remove the source file after successful copy
	if err := os.Remove(source); err != nil {
		return err
	}

	return nil
}

func contains(slice []string, str string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
}
