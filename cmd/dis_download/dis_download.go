package dis_download

import (
	"strings"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/dis_operations"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
}

var commandDefinition = &cobra.Command{
	Use:   "dis_download target:name destination:path",
	Short: `Download distributed file to destination path.`,
	Long: strings.ReplaceAll(
		`Download distributed file to destination path. Target file must be
requested in full name, meaning that it must be followed with its extension.

eg

	rclone dis_download test.txt local:path


Note that during this process, distributed binary files stored remote will be 
requeted from the remotes and decoded in the process to be downloaded in the 
destination path. If some of the partitioned files have been lost or damaged
during this process, an automatic recovery process will be used to restore the 
original file. Erasure Coding is using Reed Solomon is used during this process
and parity blocks are used to restore the file. If the damage goes over a
threshhold, recovery of the file can be difficult.

Downloading the file does not erase the distributed binary files in the remote.
To erase the files, use the dis_rm command instead.

[dis_rm] (/commands/dis_rm/).`, "|", "`"),
	Annotations: map[string]string{
		"groups": "Copy,Filter,Listing,Important",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(2, 2, command, args)
		cmd.Run(true, true, command, func() error {
			sameCommand, err := dis_operations.CheckState("download", args, dis_operations.None) // use default lb, its not going to be used anyways
			if err != nil {
				return err
			}
			if !sameCommand {
				return dis_operations.Dis_Download(args, false)
			}
			return nil
		})
	},
}
