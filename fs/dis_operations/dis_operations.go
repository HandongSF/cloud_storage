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
	// 성주 코드
	err = dis_init(args[0])
	if err != nil {
		return fmt.Errorf("error in dis_init: %w", err)
	}
	// Reed solomon에게 요청
	//dis_fi, num := reedsolomon.DoEncode(args[0])

	// 결과로 받은 파일들 리모트에 올리기
	// 올리면서 config 파일 업데이트 할 정보 저장

	// 다 올리고 config 파일 최종적으로 업데이트
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

func dis_init(arg string) error {
	path, err := os.Getwd()
	if err != nil {
		return err
	}

	fullPath := filepath.Join(path, arg)
	// 존재하지 않는 파일이라면 cmd창에 에러 메세지 출력
	fi, err := os.Open(fullPath)
	if err != nil {
		return err
	}
	// 존재한다면 ok 메세지 cmd창에 출력
	fmt.Println("Success to find file : ", fi)

	// 유저가 현재 위치한 로컬 디렉토리에(path) 파일이름으로 디렉토리 생성
	fileBase := strings.TrimSuffix(arg, filepath.Ext(arg))
	dirPath := filepath.Join(path, fileBase+"_dir")

	err = os.Mkdir(dirPath, 0755)
	if err != nil {
		return err
	}
	fmt.Println("Directory created successfully at: ", dirPath)

	return nil
}
