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
	Short: "saving rclone config file at rclone dir",
	Long: `saving rclone config file that is saved user's driver at rclone dir

Example:

    $ rclone dis_config path

This command calls the internal Config_upload function to perform the process.`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		cmd.Run(true, true, command, func() error {
			err := dis_config.Config_upload(args)
			if err != nil {
				return fmt.Errorf("error during dis_upload: %v", err)
			}
			return nil
		})
	},
}
