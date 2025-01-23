package dis_operations

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rclone/rclone/fs/config"
)

func GetDistributedInfo(filePath string, remote Remote) (DistributedFile, error) {
	if filePath == "" {
		return DistributedFile{}, errors.New("filePath cannot be empty")
	}

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return DistributedFile{}, fmt.Errorf("failed to stat file %s: %v", filePath, err)
	}

	return DistributedFile{
		DistributedFile: filepath.Base(filePath),
		DisFileSize:     fileInfo.Size(),
		Remote:          remote,
	}, nil
}

func MakeDataMap(originalFilePath string, distributedFiles []DistributedFile, paddingAmount int64) error {
	if originalFilePath == "" {
		return errors.New("originalFilePath cannot be empty")
	}

	jsonFilePath := getJsonFilePath()

	originalFileName := filepath.Base(originalFilePath)
	originalFileInfo, err := os.Stat(originalFilePath)
	if err != nil {
		return fmt.Errorf("failed to stat original file: %v", err)
	}

	checksum, err := calculateChecksum(originalFilePath)
	if err != nil {
		return fmt.Errorf("failed to calculate checksum: %v", err)
	}

	newFileInfo := FileInfo{
		FileName:             originalFileName,
		FileSize:             originalFileInfo.Size(),
		Checksum:             checksum,
		Padding:              paddingAmount,
		DistributedFileInfos: distributedFiles,
	}

	existingFiles, err := readJsonFile(jsonFilePath)
	if err != nil {
		return err
	}

	updatedFiles := removeFileByName(existingFiles, originalFileName)
	updatedFiles = append(updatedFiles, newFileInfo)

	return writeJsonFile(jsonFilePath, updatedFiles)
}

func RemoveFileFromMetadata(fileName string) error {
	jsonFilePath := getJsonFilePath()

	existingFiles, err := readJsonFile(jsonFilePath)
	if err != nil {
		return err
	}

	updatedFiles := removeFileByName(existingFiles, fileName)

	return writeJsonFile(jsonFilePath, updatedFiles)
}

func GetFileInfoStruct(fileName string) (FileInfo, error) {
	jsonFilePath := getJsonFilePath()

	files, err := readJsonFile(jsonFilePath)
	if err != nil {
		return FileInfo{}, err
	}

	for _, file := range files {
		if file.FileName == fileName {
			return file, nil
		}
	}

	return FileInfo{}, fmt.Errorf("file name '%s' not found", fileName)
}

func DoesFileStructExist(fileName string) (bool, error) {
	jsonFilePath := getJsonFilePath()

	files, err := readJsonFile(jsonFilePath)
	if err != nil {
		return false, err
	}

	for _, file := range files {
		if file.FileName == fileName {
			return true, nil
		}
	}

	return false, nil
}

func GetDistributedFileStruct(fileName string) ([]DistributedFile, error) {
	jsonFilePath := getJsonFilePath()

	files, err := readJsonFile(jsonFilePath)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.FileName == fileName {
			return file.DistributedFileInfos, nil
		}
	}

	return nil, fmt.Errorf("file name '%s' not found", fileName)
}

func GetChecksum(fileName string) string {
	fileInfo, err := GetFileInfoStruct(fileName)
	if err != nil {
		return ""
	}
	return fileInfo.Checksum
}

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

func getJsonFilePath() string {
	path := GetRcloneDirPath()
	return filepath.Join(path, "data", "datamap.json")
}

func readJsonFile(filePath string) ([]FileInfo, error) {
	file, err := os.Open(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []FileInfo{}, nil // Return empty slice if file does not exist
		}
		return nil, fmt.Errorf("failed to open JSON file: %v", err)
	}
	defer file.Close()

	var files []FileInfo
	if err := json.NewDecoder(file).Decode(&files); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("failed to decode JSON: %v", err)
	}
	return files, nil
}

func writeJsonFile(filePath string, data []FileInfo) error {
	if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}

	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write JSON file: %v", err)
	}
	return nil
}

func removeFileByName(files []FileInfo, fileName string) []FileInfo {
	updatedFiles := []FileInfo{}
	for _, file := range files {
		if file.FileName != fileName {
			updatedFiles = append(updatedFiles, file)
		}
	}
	return updatedFiles
}

func GetRcloneDirPath() (path string) {
	fullConfigPath := config.GetConfigPath()
	path = filepath.Dir(fullConfigPath)

	return path
}
