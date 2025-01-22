// Package dis_ls provides the ls command.
package dis_ls

import (
	"fmt"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/dis_ls/dis_lshelp"
	"github.com/rclone/rclone/fs/dis_operations"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
}

var commandDefinition = &cobra.Command{
	Use:   "dis_ls",
	Short: `List the distributed objects in the path with its name.`,
	Long: `Lists the distributed objects in the remote storage to standard output in a human
readable format with its name. 

Eg

    $ rclone dis_ls swift:bucket
        testfile_1.txt
        testfile_2.txt

` + dis_lshelp.Help,
	Annotations: map[string]string{
		"groups": "Filter,Listing",
	},
	Run: func(command *cobra.Command, args []string) {
		// command가 "dis_ls" 인지 체크
		if command.Use == "dis_ls" {
			cmd.Run(true, true, command, func() error {
				// command 가 dis_ls라면 GetDistributedFile() 함수 실행
				fileNames, err := dis_operations.Dis_ls()
				if err != nil {
					return fmt.Errorf("error while retrieving distributed files: %v", err)
				}

				// distributed 된 파일 이름 출력
				for _, name := range fileNames {
					fmt.Println(name)
				}
				return nil
			})
		} else {
			// command가 "dis_ls" 가 아니라면, error message 출력
			fmt.Println("Error: Unsupported command. Use 'dis_ls' to list distributed files.")
		}
	},
}
