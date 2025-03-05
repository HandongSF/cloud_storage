package dis_operations

import (
	"encoding/json"
	"fmt"
	"os"
)

// return filename
func Dis_ls() ([]string, error) {

	//rclonePath := GetRcloneDirPath()

	//remote->local sync
	//err := dis_config.SyncAnyRemoteToLocal(rclonePath)
	//fmt.Printf("dis_ls err : %s\n", err)

	FilePath := getJsonFilePath()

	// 파일 열기
	file, err := os.Open(FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file : %v", err)
	}
	defer file.Close()

	var data map[string]FileInfo
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %v", err)
	}

	var fileNames []string
	for fileName := range data {
		fileNames = append(fileNames, fileName)
	}

	return fileNames, nil
}
