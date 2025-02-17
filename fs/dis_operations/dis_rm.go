package dis_operations

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/dis_config"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

func Dis_rm(arg []string, reSignal bool) (err error) {
	rclonePath := GetRcloneDirPath()

	//remote->local sync
	err = dis_config.SyncAnyRemoteToLocal(rclonePath)
	if err != nil {
		return err
	}

	//일단 list에 존재하는지 확인
	fmt.Printf("Dis_rm " + arg[0] + "\n")

	originalFileName := arg[0]
	var distributedFileArray []DistributedFile

	_, err = GetFileInfoStruct(originalFileName)
	if err != nil {
		return err
	}

	// reRm 인 경우
	if reSignal {
		distributedFileArray, err = GetUncompletedFileInfo(originalFileName)
		if err != nil {
			return err
		}

	} else { // reRm이 아닌경우
		err = UpdateFileFlag(originalFileName, "rm")
		if err != nil {
			return err
		}
		distributedFileArray, err = GetDistributedFileStruct(originalFileName)
		if err != nil {
			return err
		}
	}

	start := time.Now() // 타이머 시작
	// rm 로직
	if err := startRmFileGoroutine(originalFileName, distributedFileArray); err != nil {
		return err
	}

	elapsed := time.Since(start)
	fmt.Printf("Time taken for dis_rm: %s\n", elapsed)

	// 모든 것이 성공했다면
	// Update Load Balancer Info
	DecrementRemoteConnectionCounter(distributedFileArray)

	// flag 초기화
	err = ResetCheckFlag(arg[0])
	if err != nil {
		return err
	}
	err = RemoveFileFromMetadata(arg[0])
	if err != nil {
		return fmt.Errorf("failed to remove file from metadata: %v", err)
	}

	fmt.Printf("Successfully deleted all parts of %s and updated metadata.\n", arg[0])
	err = dis_config.SyncAllLocalToRemote(rclonePath)
	if err != nil {
		return err
	}
	return nil
}

func remoteCallDeleteFile(args []string) (err error) {
	fmt.Printf("Calling remoteCallDeleteFile with args: %v\n", args)

	deleteFileCommand := *deleteFileDefinition
	deleteFileCommand.SetArgs(args)

	err = deleteFileCommand.Execute()
	if err != nil {
		return fmt.Errorf("error executing deleteCommand: %w", err)
	}

	return nil
}

func startRmFileGoroutine(originalFileName string, distributedFileArray []DistributedFile) (err error) {
	var wg sync.WaitGroup
	errCh := make(chan error, len(distributedFileArray))

	remoteDirectory := "Distribution"
	for _, info := range distributedFileArray {
		wg.Add(1)
		go func(info DistributedFile) {
			defer wg.Done()
			//arg인자에 Info.Remote.Name:Distribution/info.DistributedFile
			hashedFileName, err := CalculateHash(info.DistributedFile)
			if err != nil {
				errCh <- fmt.Errorf("failed to calculate hash %v", err)
			}
			remotePath := fmt.Sprintf("%s:%s/%s", info.Remote.Name, remoteDirectory, hashedFileName)
			fmt.Printf("Deleting file on remote: %s\n", remotePath)

			if err := remoteCallDeleteFile([]string{remotePath}); err != nil {
				errCh <- fmt.Errorf("failed to delete %s on remote %s: %w", info.DistributedFile, info.Remote.Name, err)
			}

			// 삭제했다면 플래그 업데이트
			UpdateDistributedFileCheckFlag(originalFileName, info.DistributedFile, true)
		}(info)
	}

	wg.Wait()
	close(errCh)

	var deleteErrs []error
	for err := range errCh {
		deleteErrs = append(deleteErrs, err)
	}

	if len(deleteErrs) > 0 {
		return fmt.Errorf("Errors occurred while deleting files: %v", deleteErrs)
	}

	return nil
}

var deleteFileDefinition = &cobra.Command{
	Use: "deletefile remote:path",
	Annotations: map[string]string{
		"versionIntroduced": "v1.42",
		"groups":            "Important",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		f, fileName := cmd.NewFsFile(args[0])
		cmd.RunWithSustainOS(true, false, command, func() error {
			if fileName == "" {
				fmt.Printf("%s is a directory or doesn't exist\n", args[0])
				return nil
			}
			fileObj, err := f.NewObject(context.Background(), fileName)
			if err != nil {
				fmt.Printf("%s is a directory or doesn't exist\n", args[0])
				return nil
			}
			return operations.DeleteFile(context.Background(), fileObj)
		}, true)
	},
}
