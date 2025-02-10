package dis_operations

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

func Dis_rm(arg []string, reSignal bool) (err error) {
	//일단 list에 존재하는지 확인
	fmt.Printf(arg[0] + "\n")

	originalFileName := arg[0]
	var distributedFileArray []DistributedFile

	listOfFiles, err := Dis_ls() //Dis_ls로 목록 체크
	if err != nil {
		return fmt.Errorf("GetDistributedFile failed %v", err)
	}
	fmt.Printf("number of files: %d\n", len(listOfFiles))
	start := time.Now() // 타이머 시작

	// reRm 인 경우
	if reSignal {
		distributedFileArray, err = GetUncompletedFileInfo(originalFileName)

	} else { // reRm이 아닌경우
		for _, name := range listOfFiles {
			fmt.Printf("name: " + name + " " + arg[0] + "\n")
			// Dis_ls로 일치하는 이름을 찾았다면
			if name == arg[0] {
				distributedFileArray, err = GetDistributedFileStruct(arg[0])
				if err != nil {
					return fmt.Errorf("Failed to get Distributed File Info: %v", err)
				}

				return nil
			}
		}
	}
	// rm 로직
	if err := startRmFileGoroutine(originalFileName, distributedFileArray); err != nil {
		return err
	}

	elapsed := time.Since(start)
	fmt.Printf("Time taken for dis_rm: %s\n", elapsed)

	// 모든 것이 성공했다면 flag 초기화
	err = ResetCheckFlag(arg[0])
	if err == nil {
		err = RemoveFileFromMetadata(arg[0])
		if err != nil {
			return fmt.Errorf("Failed to remove file from metadata: %v", err)
		}
		fmt.Printf("Successfully deleted all parts of %s and updated metadata.\n", arg[0])
	} else {
		//??
	}

	return fmt.Errorf("file %s does not exist on remote.\n", arg[0])
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

	err = RemoveFileFromMetadata(originalFileName)
	if err != nil {
		return fmt.Errorf("Failed to remove file from metadata: %v", err)
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
