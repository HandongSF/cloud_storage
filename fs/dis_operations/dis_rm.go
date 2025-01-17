package dis_operations

import (
	"context"
	"fmt"
	"sync"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

func Dis_rm(arg []string) (err error) {
	//일단 list에 존재하는지 확인
	fmt.Printf(arg[0] + "\n")
	listOfFiles, err := GetDistributedFile()
	if err != nil {
		return fmt.Errorf("GetDistributedFile failed %v", err)
	}
	fmt.Printf("number of files: %d\n", len(listOfFiles))

	// print list of files (for debug)
	for _, name := range listOfFiles {
		fmt.Printf("check: " + name + "\n")
	}

	for _, name := range listOfFiles {
		fmt.Printf("name: " + name + " " + arg[0] + "\n")
		if name == arg[0] {
			DistributedFileInfos, err := GetDistributedFileStruct(arg[0])
			if err != nil {
				return fmt.Errorf("Failed to get Distributed File Info: %v", err)
			}

			var wg sync.WaitGroup
			errCh := make(chan error, len(DistributedFileInfos))

			remoteDirectory := "Distribution"
			for _, info := range DistributedFileInfos {
				wg.Add(1)
				go func(info DistributedFile) {
					defer wg.Done()
					//arg인자에 Info.Remote.Name:Distribution/info.DistributedFile
					remotePath := fmt.Sprintf("%s:%s/%s", info.Remote.Name, remoteDirectory, info.DistributedFile)
					fmt.Printf("Deleting file on remote: %s\n", remotePath)

					if err := remoteCallDeleteFile([]string{remotePath}); err != nil {
						errCh <- fmt.Errorf("failed to delete %s on remote %s: %w", info.DistributedFile, info.Remote.Name, err)
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
				return fmt.Errorf("Errors occurred while deleting files: %v", deleteErrs)
			}

			err = RemoveFileFromMetadata(arg[0])
			if err != nil {
				return fmt.Errorf("Failed to remove file from metadata: %v", err)
			}

			fmt.Printf("Successfully deleted all parts of %s and updated metadata.\n", arg[0])

			break
		}
	}

	return nil

}

func remoteCallDeleteFile(args []string) (err error) {
	fmt.Printf("Calling remoteCallDeleteFile with args: %v\n", args)

	deleteFileCommand := *deleteFileDefinition
	deleteFileCommand.SetArgs(args)

	err = deleteFileCommand.Execute()
	if err != nil {
		return fmt.Errorf("error executing copyCommand: %w", err)
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
				return fmt.Errorf("%s is a directory or doesn't exist: %w", args[0], fs.ErrorObjectNotFound)
			}
			fileObj, err := f.NewObject(context.Background(), fileName)
			if err != nil {
				return err
			}
			return operations.DeleteFile(context.Background(), fileObj)
		}, true)
	},
}
