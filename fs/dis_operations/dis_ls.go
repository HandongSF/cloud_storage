package dis_operations

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// return filename
func GetDistributedFile() ([]string, error) {
	FilePath := ""
	FilePath, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to find Path: %v", err)
	}
	FilePath = filepath.Join(FilePath, "data", "datamap.json")
	// 파일 열기
	file, err := os.Open(FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file : %v", err)
	}
	defer file.Close()

	var data []FileInfo
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %v", err)
	}

	var fileNames []string
	for _, item := range data {
		fileNames = append(fileNames, item.FileName)
	}

	return fileNames, nil
}

func GetFileInfoStruct(file_Name string) (FileInfo, error) {
	FilePath := ""
	FilePath, err := os.Getwd()
	if err != nil {
		return FileInfo{}, fmt.Errorf("failed to find Path: %v", err)
	}
	FilePath = filepath.Join(FilePath, "data", "datamap.json")
	// 파일 열기
	file, err := os.Open(FilePath)
	if err != nil {
		return FileInfo{}, fmt.Errorf("failed to open file : %v", err)
	}
	defer file.Close()

	// Json file decoding
	var files []FileInfo
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&files)
	if err != nil {
		return FileInfo{}, fmt.Errorf("failed to decode JSON: %v", err)
	}

	// 주어진 파일 이름으로 검색
	for _, file := range files {
		if file.FileName == file_Name {
			return file, nil
		}
	}

	// 파일 이름을 찾지 못한 경우
	return FileInfo{}, fmt.Errorf("file name '%s' not found", file_Name)
}

func GetDistributedFileStruct(fileName string) ([]DistributedFile, error) {
	FilePath := ""
	FilePath, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to find Path: %v", err)
	}
	FilePath = filepath.Join(FilePath, "data", "datamap.json")

	// open json file
	file, err := os.Open(FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file : %v", err)
	}
	defer file.Close()

	// Json file decoding
	var files []FileInfo
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&files)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %v", err)
	}

	// 주어진 파일 이름으로 검색
	for _, file := range files {
		if file.FileName == fileName {
			return file.DistributedFileInfos, nil
		}
	}

	// 파일 이름을 찾지 못한 경우
	return nil, fmt.Errorf("file name '%s' not found", fileName)
}
