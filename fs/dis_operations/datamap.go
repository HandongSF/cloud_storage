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
	"sync"

	"github.com/rclone/rclone/fs/config"
)

var jsonFileMutex sync.Mutex

// making distributed file info
func GetDistributedInfo(fileName string, filePath string, remote Remote, checksum string) (DistributedFile, error) {
	if filePath == "" {
		return DistributedFile{}, errors.New("filePath cannot be empty")
	}

	return DistributedFile{
		DistributedFile: fileName,
		Remote:          remote,
		Checksum:        checksum,
		Check:           false,
	}, nil
}

// making file info about original file
func MakeDataMap(originalFilePath string, distributedFiles []DistributedFile, disFileSize int64, paddingAmount int64) error {
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
		DisFileSize:          disFileSize,
		Flag:                 true,
		State:                "upload",
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

// returning checksum of file we want to know
func GetChecksum(fileName string) string {
	fileInfo, err := GetFileInfoStruct(fileName)
	if err != nil {
		return ""
	}
	return fileInfo.Checksum
}

// calculating checksum of file
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

// getting path existing json file
func getJsonFilePath() string {
	path := GetRcloneDirPath()
	return filepath.Join(path, "data", "datamap.json")
}

// reading json file and then returning original file infos
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

// writting original file infos on json file
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

// returning original file infos without file we want to remove
func removeFileByName(files []FileInfo, fileName string) []FileInfo {
	updatedFiles := []FileInfo{}
	for _, file := range files {
		if file.FileName != fileName {
			updatedFiles = append(updatedFiles, file)
		}
	}
	return updatedFiles
}

// getting rclone dir path
func GetRcloneDirPath() (path string) {
	fullConfigPath := config.GetConfigPath()
	path = filepath.Dir(fullConfigPath)

	return path
}

// getting list of checksums about distributed files
func GetChecksumList(name string) (checksums []string) {
	disFileInfo, err := GetDistributedFileStruct(name)
	if err != nil {
		fmt.Printf("no file data")
	}
	for _, info := range disFileInfo {
		checksums = append(checksums, info.Checksum)
	}
	return checksums
}

// checking to see if it terminated abnormally and if so, returning what command is was previously
func CheckFlagAndState() (bool, string) {
	infos, err := readJsonFile(getJsonFilePath())
	if err != nil {
		fmt.Printf("failed to read json file at checkflag func")
	}
	flag := false
	for _, info := range infos {
		if info.Flag {
			return flag, info.State
		}
	}
	return flag, ""
}

// Updating file flag to true.
// this function is used when downloading or deleting a file.
func UpdateFileFlag(originalFileName string, state string) error {
	jsonFileMutex.Lock()

	jsonFilePath := getJsonFilePath()

	files, err := readJsonFile(jsonFilePath)
	if err != nil {
		return fmt.Errorf("failed to read JSON file: %v", err)
	}

	updated := false
	for i, file := range files {
		if file.FileName == originalFileName {
			files[i].Flag = true
			files[i].State = state
			updated = true
		}
	}

	if !updated {
		return fmt.Errorf("file '%s' not found", originalFileName)
	}

	if err := writeJsonFile(jsonFilePath, files); err != nil {
		return fmt.Errorf("failed to write updated JSON: %v", err)
	}

	jsonFileMutex.Unlock()
	return nil
}

// updating distributedfile check flag after uploading, downloading or removing
func UpdateDistributedFileCheckFlag(originalFileName string, distributedFileName string, newCheck bool) error {
	jsonFileMutex.Lock()

	jsonFilePath := getJsonFilePath()

	files, err := readJsonFile(jsonFilePath)
	if err != nil {
		return fmt.Errorf("failed to read JSON file : %v\n", err)
	}

	updated := false

	for i, file := range files {
		if file.FileName == originalFileName {
			for k, disInfo := range file.DistributedFileInfos {
				if disInfo.DistributedFile == distributedFileName {
					files[i].DistributedFileInfos[k].Check = newCheck
					updated = true
					break
				}
			}
		}
	}

	if !updated {
		return fmt.Errorf("failed to reset flag: original file '%s' not found", originalFileName)
	}

	err = writeJsonFile(jsonFilePath, files)
	if err != nil {
		return fmt.Errorf("failed to write updated JSON: %v", err)
	}

	jsonFileMutex.Unlock()
	return nil
}

// resetting file check flag after finishing operation
func ResetCheckFlag(originalFileName string) error {
	jsonFileMutex.Lock()

	jsonFilePath := getJsonFilePath()

	files, err := readJsonFile(jsonFilePath)
	if err != nil {
		return fmt.Errorf("failed to read JSON file: %v", err)
	}

	updated := false

	for i, file := range files {
		if file.FileName == originalFileName {
			files[i].Flag = false
			for k := range file.DistributedFileInfos {
				files[i].DistributedFileInfos[k].Check = false
			}
			updated = true
			break
		}
	}

	if !updated {
		return fmt.Errorf("failed to reset flag: original file '%s' not found", originalFileName)
	}

	if err := writeJsonFile(jsonFilePath, files); err != nil {
		return fmt.Errorf("failed to write updated JSON: %v", err)
	}

	jsonFileMutex.Unlock()
	return nil
}
