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
	//rclonePath := GetRcloneDirPath()

	//remote->local sync
	//err = dis_config.SyncAnyRemoteToLocal(rclonePath)
	//if err != nil {
	//	return err
	//}

	originalFileName := arg[0]
	var distributedFileArray []DistributedFile

	_, err = GetFileInfoStruct(originalFileName)
	if err != nil {
		return err
	}

	// if re-rm (due to previous failure)
	if reSignal {
		distributedFileArray, err = GetUncompletedFileInfo(originalFileName)
		if err != nil {
			return err
		}

	} else {
		err = UpdateFileFlag(originalFileName, "rm")
		if err != nil {
			return err
		}
		distributedFileArray, err = GetDistributedFileStruct(originalFileName)
		if err != nil {
			return err
		}
	}

	start := time.Now()

	if err := startRmFileGoroutine(originalFileName, distributedFileArray); err != nil {
		return err
	}

	elapsed := time.Since(start)
	fmt.Printf("Time taken for dis_rm: %s\n", elapsed)

	err = ResetCheckFlag(originalFileName)
	if err != nil {
		return err
	}
	err = RemoveFileFromMetadata(originalFileName)
	if err != nil {
		return fmt.Errorf("failed to remove file from metadata: %v", err)
	}

	fmt.Printf("Successfully deleted all parts of %s and updated metadata.\n", originalFileName)
	//err = dis_config.SyncAllLocalToRemote(rclonePath)
	//if err != nil {
	//	return err
	//}

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
		if info.Remote.String() == "|" {
			fmt.Printf("Empty Remote\n")
			err = UpdateDistributedFile_CheckFlag(originalFileName, info.DistributedFile, true)
			if err != nil {
				fmt.Printf("UpdateDistributedFile_CheckFlag 에러 : %v\n", err)
			}
			continue
		}

		wg.Add(1)
		go func(info DistributedFile) {
			defer wg.Done()

			// Info.Remote.Name:Distribution/info.DistributedFile
			hashedFileName, err := CalculateHash(info.DistributedFile)
			if err != nil {
				errCh <- fmt.Errorf("failed to calculate hash %v", err)
			}

			remotePath := fmt.Sprintf("%s:%s/%s", info.Remote.Name, remoteDirectory, hashedFileName)

			if err := remoteCallDeleteFile([]string{remotePath}); err != nil {
				errCh <- fmt.Errorf("failed to delete %s on remote %s: %w", info.DistributedFile, info.Remote.Name, err)
			}

			// Update flags
			err = UpdateDistributedFile_CheckFlag(originalFileName, info.DistributedFile, true)
			if err != nil {
				fmt.Printf("UpdateDistributedFile_CheckFlag 에러 : %v\n", err)
			}

			if err != nil {
				errCh <- fmt.Errorf("error updating remote info: %v", err)
			}
		}(info)
	}

	wg.Wait()
	close(errCh)

	var deleteErrs []error
	for err := range errCh {
		deleteErrs = append(deleteErrs, err)
	}

	if len(deleteErrs) > 0 {
		return fmt.Errorf("errors occurred while deleting files: %v", deleteErrs)
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
