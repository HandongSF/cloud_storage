package dis_operations

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

func ConvertFileNameForUP(name string) (string, error) {
	hashFileName, err := CalculateHash(name)
	if err != nil {
		fmt.Errorf("failed to calculate hash: %v", err)
	}

	dir := GetShardPath()
	hashedFilePath := filepath.Join(dir, hashFileName)
	originalFilePath := filepath.Join(dir, name)

	err = os.Rename(originalFilePath, hashedFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to rename file from %q to %q: %v", originalFilePath, hashedFilePath, err)
	}

	fmt.Printf("File renamed to %s\n", hashFileName)
	return hashFileName, nil
}

func ConvertFileNameForDo(hashedName string, originalName string) error {
	dir := GetShardPath()
	hashedFilePath := filepath.Join(dir, hashedName)
	originalFilePath := filepath.Join(dir, originalName)

	_, err := os.Stat(hashedFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("no file")
		}
	}

	err = os.Rename(hashedFilePath, originalFilePath)
	if err != nil {
		return fmt.Errorf("failed to rename file from %q to %q: %v", hashedFilePath, originalFilePath, err)
	}

	fmt.Printf("File restored to original name: %s\n", originalName)
	return nil

}

func CalculateHash(name string) (string, error) {
	hash := sha256.New()
	_, err := hash.Write([]byte(name))
	if err != nil {
		return "", fmt.Errorf("failed to write hash: %v", err)
	}
	hashFileName := hex.EncodeToString(hash.Sum(nil))
	return hashFileName, nil

}

func GetShardPath() string {
	path := GetRcloneDirPath()
	return filepath.Join(path, "shard")
}
