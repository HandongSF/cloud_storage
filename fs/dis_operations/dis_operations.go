package dis_operations

import (
	"context"
	"fmt"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/sync"
	"github.com/spf13/cobra"
)

var remoteDirectory = "Distribution"

func Dis_Upload(args []string) (err error) {

	remotes := config.GetRemoteNames()

	for _, remote := range remotes {
		dest := fmt.Sprintf("%s:%s", remote, remoteDirectory)
		tempArgs := []string{args[0], dest}

		fmt.Printf("Uploading to remote: %s\n", dest)
		err = remoteCallCopy(tempArgs)

		if err != nil {
			return fmt.Errorf("error in Dis_Upload: %w", err)
		}
	}

	fmt.Printf("Completed Dis_Upload\n")
	return nil
}

func remoteCallCopy(args []string) (err error) {
	fmt.Printf("Calling remoteCallCopy with args: %v\n", args)

	// Fetch the copy command
	copyCommand := *commandDefinition
	copyCommand.SetArgs(args)

	err = copyCommand.Execute()
	if err != nil {
		return fmt.Errorf("error executing copyCommand: %w", err)
	}

	return nil
}

var (
	createEmptySrcDirs = false
)

var commandDefinition = &cobra.Command{
	Use: "copy source:path dest:path",
	Annotations: map[string]string{
		"groups": "Copy,Filter,Listing,Important",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(2, 2, command, args)
		fsrc, srcFileName, fdst := cmd.NewFsSrcFileDst(args)
		cmd.RunWithSustainOS(true, true, command, func() error {
			if srcFileName == "" {
				return sync.CopyDir(context.Background(), fdst, fsrc, createEmptySrcDirs)
			}
			return operations.CopyFile(context.Background(), fdst, fsrc, srcFileName, srcFileName)
		}, true)
	},
}
