package dis_upload

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
	Use:   "dis_upload source:path",
	Short: `Upload source file via distributing it to registered remotes.`,
	Long: strings.ReplaceAll(
		`Upload source file via distributing it to registered remotes. This 
means selecting a source file in local path and partioning it to several binary 
files. This is achieved using Erasure Coding so even when some of the partioned 
blocks are lost, parity blocks can be used to restore the original file.

Note that this command is to obtain a Private Cloud Storage where a single
file is completely unreadable, thus staying hidden from the storage service 
provider.

Distributed files will be further encoded and stored in specific directories 
(only for containing distributed data) in their appropriate remote. 
The distribution process will select all remotes accessible at the time of
call and distribute the files using a fair Load Balancing Algorihtm. 

Uploading duplicate files will enact CLI to start an interactive process that
will ask the user whether to overwrite the file or to skip uploading it. 

If you wish to simply copy the file without any distribution, use the 
[copy] (/commands/copy/) command instead.`, "|", "`"),
	Annotations: map[string]string{
		"groups": "Copy,Filter,Listing,Important",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		cmd.Run(true, true, command, func() error {
			dis_operations.CheckState()
			return dis_operations.Dis_Upload(args, false)
		})
	},
}
