package dis_upload

import (
	"fmt"
	"strings"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/dis_operations"
	"github.com/spf13/cobra"
)

var loadBalancer LoadBalancerFlag

func init() {
	cmd.Root.AddCommand(commandDefinition)
	loadBalancer.Value = dis_operations.RoundRobin // Default value
	commandDefinition.Flags().VarP(&loadBalancer, "loadbalancer", "b", "Load balancing strategy (RoundRobin, LeastConnections,ResourceBased)")
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
			if !loadBalancer.Value.IsValid() {
				return fmt.Errorf("invalid load balancer type: %s (valid: RoundRobin, LeastConnections, Random)", loadBalancer.Value)
			}
			fmt.Printf("Uploading using load balancer: %s\n", loadBalancer.Value)

			_, err := dis_operations.CheckState("upload", args, loadBalancer.Value)
			if err != nil {
				return err
			}
			return dis_operations.Dis_Upload(args, false, loadBalancer.Value)
		})
	},
}

// Custom type to implement flag validation
type LoadBalancerFlag struct {
	Value dis_operations.LoadBalancerType
}

func (l *LoadBalancerFlag) String() string {
	return string(l.Value)
}

func (l *LoadBalancerFlag) Set(value string) error {
	lb := dis_operations.LoadBalancerType(value)
	if !lb.IsValid() {
		return fmt.Errorf("invalid load balancer type: %s (valid: RoundRobin, LeastConnections, Random)", value)
	}
	l.Value = lb
	return nil
}

func (l *LoadBalancerFlag) Type() string {
	return "LoadBalancerType"
}
