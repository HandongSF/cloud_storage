package dis_remove

import (
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/dis_operations"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
}

var commandDefinition = &cobra.Command{
	Use:   "dis_rm fileName",
	Short: `remove distributed file on registered remotes.`,
	Long:  `Remove distributed file on registered remotes. `,
	Annotations: map[string]string{
		"groups": "Important",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		cmd.Run(true, true, command, func() error {
			dis_operations.CheckState()
			return dis_operations.Dis_rm(args)
		})
	},
}
