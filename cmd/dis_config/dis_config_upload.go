package dis_config

import (
	"fmt"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/dis_config"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
}

var commandDefinition = &cobra.Command{
	Use:   "dis_config",
	Short: "Upload the distributed config files after sync",
	Long: `Performs remote→local sync and then executes the upload operation,
followed by local→remote sync to update changes on the remote.

Example:

    $ rclone dis_upload

This command calls the internal Config_upload function to perform the process.`,
	Run: func(command *cobra.Command, args []string) {
		cmd.Run(true, true, command, func() error {
			err := dis_config.Config_upload()
			if err != nil {
				return fmt.Errorf("error during dis_upload: %v", err)
			}
			return nil
		})
	},
}
