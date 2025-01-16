package dis_operations

import (
	"context"
	"fmt"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

func Dis_Remove(arg []string) (err error) {
	//일단 list에 존재하는지 확인
	fmt.Printf(arg[0] + "\n")
	listOfFiles, err := GetDistributedFile()
	if err != nil {
		return fmt.Errorf("GetDistributedFile failed %v", err)
	}
	fmt.Printf("number of files: %d\n", len(listOfFiles))

	check := false

	// for _, name := range listOfFiles {
	// 	fmt.Printf("check: " + name + "\n")
	// }

	for _, name := range listOfFiles {
		fmt.Printf("name: "+name, "\n")
		if name == arg[0] {
			check = true
		}
	}

	if !check {
		return fmt.Errorf("%s not found", arg[0])
	} else {
		remoteCallDeleteFile(arg)
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
		cmd.Run(true, false, command, func() error {
			if fileName == "" {
				return fmt.Errorf("%s is a directory or doesn't exist: %w", args[0], fs.ErrorObjectNotFound)
			}
			fileObj, err := f.NewObject(context.Background(), fileName)
			if err != nil {
				return err
			}
			return operations.DeleteFile(context.Background(), fileObj)
		})
	},
}
