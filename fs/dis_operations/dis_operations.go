package dis_operations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
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
	remotes := config.GetRemoteNames()
	distributedFileArray := make([]DistributedFile, len(dis_names))
	rr_counter := 0 // Round Robin

	for i, source := range dis_names {

		dest := fmt.Sprintf("%s:%s", remotes[i], remoteDirectory)

		fmt.Printf("Uploading file %s to %s of size %d\n", source, dest, shardSize)

		// Perform the upload
		err = remoteCallCopy([]string{source, dest})
		if err != nil {
			return fmt.Errorf("error in Dis_Upload for file %s: %w", source, err)
		}

		filename := filepath.Base(source)
		distributionFile, err := GetDistributedInfo(filename, Remote{remotes[rr_counter], ""})
		distributedFileArray[i] = distributionFile

		if err != nil {
			return fmt.Errorf("error in GetDistributedInfo %s: %w", source, err)
		}
		rr_counter = (rr_counter + 1) % len(remotes)
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

func GetDistributedInfo(fileName string, remote Remote) (DistributedFile, error) {
	if fileName == "" {
		return DistributedFile{}, errors.New("FileName cannot be empty")
	}

	// we don't know yet
	distributedFilePath := "/Users/iyeeun/Desktop/cloud_storage_rclone/erasure/test.jpg.86"

	fileInfo, err := os.Stat(distributedFilePath)

	if err != nil {
		return DistributedFile{}, fmt.Errorf("failed to stat file %s: %v", distributedFilePath, err)
	}

	return DistributedFile{
		DistributedFile: fileName,
		DisFileSize:     fileInfo.Size(),
		remote:          remote,
	}, nil

}

// MakeDataMap makes file info json
func MakeDataMap(originalFilePath string, distributedFile []DistributedFile) error {
	if originalFilePath == "" {
		return errors.New("originalFilePath cannot be empty")
	}
	//you have to change on you side!!!
	jsonFilePath := "/Users/iyeeun/Desktop/datamap.json"

	originalFileName := filepath.Base(originalFilePath)

	originalFileInfo, err := os.Stat(originalFilePath)
	if err != nil {
		return fmt.Errorf("failed to stat original file: %v", err)
	}
	originalFileSize := originalFileInfo.Size()

	checksum, err := calculateChecksum(originalFilePath)
	if err != nil {
		return fmt.Errorf("failed to calculate checksum: %v", err)
	}

	newFileInfo := FileInfo{
		FileName:             originalFileName,
		FileSize:             originalFileSize,
		Checksum:             checksum,
		DistributedFileInfos: distributedFile,
	}

	// Read existing JSON data if the file exists
	var fileInfos []FileInfo
	if _, err := os.Stat(jsonFilePath); err == nil {
		file, err := os.Open(jsonFilePath)
		if err != nil {
			return fmt.Errorf("failed to open JSON file: %v", err)
		}
		defer file.Close()

		err = json.NewDecoder(file).Decode(&fileInfos)
		if err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("failed to decode existing JSON: %v", err)
		}
	}

	// Append the new file info to the array
	fileInfos = append(fileInfos, newFileInfo)

	// Marshal the updated array back to JSON
	dataMap, err := json.MarshalIndent(fileInfos, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}

	// Write JSON data to the file (overwrite)
	err = os.MkdirAll(filepath.Dir(jsonFilePath), os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}
	err = os.WriteFile(jsonFilePath, dataMap, 0644)
	if err != nil {
		return fmt.Errorf("failed to write JSON file: %v", err)
	}

	return nil

}

// calculateChecksum computes the SHA256 checksum of a file's contents.
func calculateChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file for checksum: %v", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to compute checksum: %v", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// return filename
func GetDistributedFile() ([]string, error) {
	FilePath := "/Users/iyeeun/Desktop/datamap.json"
	// 파일 열기
	file, err := os.Open(FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file : %v", err)
	}
	defer file.Close()

	// Json파일 열어서 디코딩
	var data []DistributedFile
	decoder := json.NewDecoder(file)
	ero := decoder.Decode(&data)
	if ero != nil {
		return nil, fmt.Errorf("json 디코딩 실패 %v", ero)
	}

	// 모든 original_file_name 수집
	var fileNames []string
	for _, item := range data {
		fileNames = append(fileNames, item.OriginalFileName)
	}

	return fileNames, nil

}

func Dis_Remove(arg []string) (err error) {
	//일단 list에 존재하는지 확인
	fmt.Printf(arg[0] + "\n")
	listOfFiles, err := GetDistributedFile()
	if err != nil {
		return fmt.Errorf("GetDistributedFile failed %v", err)
	}
	fmt.Printf("number of files: %d\n", len(listOfFiles))

	check := false

	// for _, name := range listOfFiles {
	// 	fmt.Printf("check: " + name + "\n")
	// }

	for _, name := range listOfFiles {
		fmt.Printf("name: "+name, "\n")
		if name == arg[0] {
			check = true
		}
	}

	if !check {
		return fmt.Errorf("%s not found", arg[0])
	} else {
		remoteCallDeleteFile(arg)
	}

	return nil

}

func remoteCallDeleteFile(args []string) (err error) {
	fmt.Printf("Calling remoteCallDeleteFile with args: %v\n", args)

	deleteFileCommand := *deleteFileDefinition
	deleteFileCommand.SetArgs(args)

	err = deleteFileCommand.Execute()
	if err != nil {
		return fmt.Errorf("error executing copyCommand: %w", err)
	}

	return nil
}

var deleteFileDefinition = &cobra.Command{
	Use: "deletefile remote:path",
	Annotations: map[string]string{
		"versionIntroduced": "v1.42",
		"groups":            "Important",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		f, fileName := cmd.NewFsFile(args[0])
		cmd.Run(true, false, command, func() error {
			if fileName == "" {
				return fmt.Errorf("%s is a directory or doesn't exist: %w", args[0], fs.ErrorObjectNotFound)
			}
			fileObj, err := f.NewObject(context.Background(), fileName)
			if err != nil {
				return err
			}
			return operations.DeleteFile(context.Background(), fileObj)
		})
	},
}
