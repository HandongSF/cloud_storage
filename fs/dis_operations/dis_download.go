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

func Dis_Download(args []string, reSignal bool) (err error) {

	//rclonePath := GetRcloneDirPath()

	//remote->local sync
	// err = dis_config.SyncAnyRemoteToLocal(rclonePath)
	// if err != nil {
	// 	return err
	// }

	originalFileName := filepath.Base(args[0])
	_, err = GetFileInfoStruct(originalFileName)
	if err != nil {
		return err
	}

	var fileNames []string
	var distributedFileInfos []DistributedFile

	if reSignal {
		//Get Distribution list(Check 읽어서 false인 것만 들고 오기)
		distributedFileInfos, err = GetUncompletedFileInfo(originalFileName)
		if err != nil {
			return err
		}

	} else {
		//state 변경
		err = UpdateFileFlag(originalFileName, "download")
		if err != nil {
			return err
		}
		distributedFileInfos, err = GetDistributedFileStruct(originalFileName)
		if err != nil {
			return err
		}
	}

	start := time.Now()
	if err := startDownloadFileGoroutine_Worker(distributedFileInfos, originalFileName, 32); err != nil {
		return err
	}

	elapsed := time.Since(start)
	fmt.Println("Current Time:", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("Time taken for dis_download: %s\n", elapsed)

	absolutePath, err := getAbsolutePath(args[1])
	if err != nil {
		return err
	}

	// Move downloaded file to destination
	fileInfo, err := GetFileInfoStruct(originalFileName)
	if err != nil {
		return err
	}

	checksums := make(map[string]string)
	for _, each := range distributedFileInfos {
		checksums[each.DistributedFile] = each.Checksum
	}

	err = reedsolomon.DoDecode(originalFileName, absolutePath, fileInfo.Padding, checksums, fileInfo.Shard, fileInfo.Parity, tryGetPassword())
	if err != nil {
		result := ShowDescription_RemoveFile(originalFileName, err)
		if result {
			err = Dis_rm([]string{originalFileName}, false)
			if err != nil {
				return err
			}
		}
		return nil
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

func startDownloadFileGoroutine_Worker(distributedFileInfos []DistributedFile, originalFileName string, workerCount int) (err error) {
	shardDir, err := reedsolomon.GetShardDir()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	jobs := make(chan DistributedFile, len(distributedFileInfos))

	// Worker function
	downloader := func() {
		for fileInfo := range jobs {
			if err := downloadFile(fileInfo, shardDir, originalFileName, &mu, &errs); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
			wg.Done()
		}
	}

	// Start worker goroutines
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			downloader()
			wg.Done()
		}()
	}

	// Send jobs to workers
	for _, fileInfo := range distributedFileInfos {
		wg.Add(1)
		jobs <- fileInfo
	}

	close(jobs) // Close channel to signal workers
	wg.Wait()   // Wait for all workers to finish

	// if len(errs) > 0 {
	// 	return fmt.Errorf("errors occurred during download: %v", errs)
	// }

	return nil
}

func startDownloadFileGoroutine(distributedFileInfos []DistributedFile, originalFileName string) (err error) {
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
			if err := downloadFile(fileInfo, shardDir, originalFileName, &mu, &errs); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}(fileInfo)
	}

	wg.Wait()

	// if len(errs) > 0 {
	// 	return fmt.Errorf("errors occurred during download: %v", errs)
	// }

	return nil
}

func downloadFile(fileInfo DistributedFile, shardDir, originalFileName string, mu *sync.Mutex, errs *[]error) error {
	startTime := time.Now()

	hashedFileName, err := CalculateHash(fileInfo.DistributedFile)
	if err != nil {
		mu.Lock()
		*errs = append(*errs, fmt.Errorf("CalculateHash for %s: %w", fileInfo.DistributedFile, err))
		mu.Unlock()
		return err
	}

	source := fmt.Sprintf("%s:%s/%s", fileInfo.Remote.Name, remoteDirectory, hashedFileName)
	fmt.Printf("Downloading shard %s to %s\n", source, shardDir)
	downloadedFilePath := path.Join(shardDir, hashedFileName)

	if err := remoteCallCopyforDown([]string{source, shardDir}); err != nil {
		mu.Lock()
		*errs = append(*errs, fmt.Errorf("remoteCallCopyforDown for %s: %w", fileInfo.DistributedFile, err))
		mu.Unlock()
		return err
	}

	elapsedTime := time.Since(startTime)
	downloadedFile, err := os.Stat(downloadedFilePath)
	if err != nil {
		mu.Lock()
		*errs = append(*errs, fmt.Errorf("downloaded file %s does not exist", downloadedFilePath))
		mu.Unlock()
		return err
	}

	// Calculate throughput
	throughput := float64(downloadedFile.Size()) / elapsedTime.Seconds()
	throughputKbps := throughput * 8 / 1e3

	if err := ConvertFileNameForDo(hashedFileName, fileInfo.DistributedFile); err != nil {
		return fmt.Errorf("ConvertFileNameForDo for %s: %w", fileInfo.DistributedFile, err)
	}

	// Update remote info
	err = updateRemoteInfo_Down(originalFileName, fileInfo, throughputKbps, mu)
	if err != nil {
		return err
	}

	return nil
}

func updateRemoteInfo_Down(originalFileName string, shardInfo DistributedFile, throughputKbps float64, mu *sync.Mutex) error {
	mu.Lock()
	err := UpdateDistributedFile_CheckFlag(originalFileName, shardInfo.DistributedFile, true)
	if err != nil {
		mu.Unlock()
		return fmt.Errorf("UpdateDistributedFileCheckFlag error: %v", err)
	}
	err = UpdateRemoteInfo(shardInfo.Remote, func(b *RemoteInfo) {
		b.UpdateThroughput(throughputKbps, 1)
	})
	mu.Unlock()
	if err != nil {
		return err
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
