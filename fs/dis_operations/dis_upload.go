package dis_operations

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/sync"
	"github.com/rclone/rclone/reedsolomon"
	"github.com/spf13/cobra"
)

func Dis_Upload(args []string) (err error) {
	// Check if file exists, if yes, create directory with same name
	err = dis_init(args[0])
	if err != nil {
		return err
	}

	dis_names, shardSize := reedsolomon.DoEncode(args[0])
	remotes := config.GetRemotes()
	distributedFileArray := make([]DistributedFile, len(dis_names))
	rr_counter := 0 // Round Robin

	for i, source := range dis_names {

		dest := fmt.Sprintf("%s:%s", remotes[rr_counter].Name, remoteDirectory)

		fmt.Printf("Uploading file %s to %s of size %d\n", source, dest, shardSize)

		// Perform the upload
		err = remoteCallCopy([]string{source, dest})
		if err != nil {
			return fmt.Errorf("error in Dis_Upload for file %s: %w", source, err)
		}

		shardFullPath, err := GetFullPath(source)
		if err != nil {
			return err
		}

		distributionFile, err := GetDistributedInfo(shardFullPath, Remote{remotes[rr_counter].Name, remotes[rr_counter].Type})
		if err != nil {
			return fmt.Errorf("error in GetDistributedInfo %s: %w", source, err)
		}

		distributedFileArray[i] = distributionFile
		rr_counter = (rr_counter + 1) % len(remotes)
	}

	originalFileFullPath, err := GetFullPath(args[0])
	if err != nil {
		return err
	}

	MakeDataMap(originalFileFullPath, distributedFileArray)
	fmt.Printf("Completed Dis_Upload!\n")
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

func GetFullPath(source string) (string, error) {
	// Get the current working directory
	currentDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %v", err)
	}

	// Join the current directory with the source file path
	fullPath := filepath.Join(currentDir, source)

	return fullPath, nil
}

func dis_init(arg string) (err error) {
	path, err := os.Getwd()
	if err != nil {
		fmt.Println("error to get current directory path: ", err)
		return err
	}

	fullPath := filepath.Join(path, arg)
	// 존재하지 않는 파일이라면 cmd창에 에러 메세지 출력
	fi, err := os.Open(fullPath)
	if err != nil {
		fmt.Println("file does not exit", err)
		return err
	}
	// 존재한다면 ok 메세지 cmd창에 출력
	fmt.Println("Success to find file : ", fi)

	// 해당 코드 필요 없을 수 있음. Reedsolomon에서 생성하는 shard file에 shard 생성
	// 유저가 현재 위치한 로컬 디렉토리에(path) 파일이름으로 디렉토리 생성
	//fileBase := strings.TrimSuffix(arg, filepath.Ext(arg))
	//dirPath := filepath.Join(path, fileBase+"_dir")

	//err = os.Mkdir(dirPath, 0755)
	//if err != nil {
	//	fmt.Println("Error creating directory: ", err)
	//	return err
	//}
	//fmt.Println("Directory created successfully at: ", dirPath)

	return nil
}
