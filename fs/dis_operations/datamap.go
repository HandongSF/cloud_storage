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
)

func GetDistributedInfo(filePath string, remote Remote) (DistributedFile, error) {
	if filePath == "" {
		return DistributedFile{}, errors.New("FileName cannot be empty")
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

// MakeDataMap makes file info json
func MakeDataMap(originalFilePath string, distributedFile []DistributedFile) error {
	if originalFilePath == "" {
		return errors.New("originalFilePath cannot be empty")
	}

	jsonFilePath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to find Path: %v", err)
	}
	jsonFilePath = filepath.Join(jsonFilePath, "data", "datamap.json")
	fmt.Println("Updated Path:", jsonFilePath)

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

func RemoveFileFromMetadata(fileName string) error {
	jsonFilePath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to find Path: %v", err)
	}
	jsonFilePath = filepath.Join(jsonFilePath, "data", "datamap.json")
	fmt.Println("Updated Path:", jsonFilePath)

	file, err := os.Open(jsonFilePath)
	if err != nil {
		return fmt.Errorf("failed to open metadata file: %v", err)
	}
	defer file.Close()

	byteValue, err := os.ReadFile(jsonFilePath)
	if err != nil {
		return fmt.Errorf("failed to read metadata file: %v", err)
	}

	var metadata []FileInfo
	if err := json.Unmarshal(byteValue, &metadata); err != nil {
		return fmt.Errorf("failed to unmarshal metadata: %v", err)
	}

	newMetadata := []FileInfo{}
	for _, fileInfo := range metadata {
		if fileInfo.FileName != fileName {
			newMetadata = append(newMetadata, fileInfo)
		}
	}

	newByteValue, err := json.MarshalIndent(newMetadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal new metadata: %v", err)
	}

	// JSON 파일에 쓰기
	if err := os.WriteFile(jsonFilePath, newByteValue, 0644); err != nil {
		return fmt.Errorf("failed to write updated metadata file: %v", err)
	}

	return nil

}
