package dis_operations

import (
	"encoding/json"
	"fmt"
	"os"
)

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
