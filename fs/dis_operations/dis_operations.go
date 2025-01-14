package dis_operations

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

func dis_init(args []string) {
	path, err := os.Getwd()
	if err != nil {
		fmt.Println("error to get current directory path: ", err)
		return
	}

	fullPath := filepath.Join(path, args[0])
	// 존재하지 않는 파일이라면 cmd창에 에러 메세지 출력
	fi, err := os.Open(fullPath)
	if err != nil {
		fmt.Println("file does not exit", err)
		return
	}
	// 존재한다면 ok 메세지 cmd창에 출력
	fmt.Println("Success to find file : ", fi)

	// 유저가 현재 위치한 로컬 디렉토리에(path) 파일이름으로 디렉토리 생성
	fileBase := strings.TrimSuffix(args[0], filepath.Ext(args[0]))
	dirPath := filepath.Join(path, fileBase+"_dir")

	err = os.Mkdir(dirPath, 0755)
	if err != nil {
		fmt.Println("Error creating directory: ", err)
		return
	}
	fmt.Println("Directory created successfully at: ", dirPath)
}
