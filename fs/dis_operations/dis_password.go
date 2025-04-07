package dis_operations

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	v2 "github.com/flew-software/filecrypt"
)

const fileCryptExtension string = ".fcef"

var app = v2.App{
	FileCryptExtension: fileCryptExtension,
	Overwrite:          true,
}

func generateRandomPassword(length int) (string, error) {
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}

func tryGetPassword() string {
	path := GetRcloneDirPath()
	filePath := filepath.Join(path, "password.txt")

	// If file exists, read and return the password
	if _, err := os.Stat(filePath); err == nil {
		data, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Println("Error reading password file:", err)
			return ""
		}
		return string(data)
	}

	// If it doesn't exist, create the directory
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		fmt.Println("Error creating directories:", err)
		return ""
	}

	// Generate a new random password
	randomPassword, err := generateRandomPassword(16) // 16-character password
	if err != nil {
		fmt.Println("Error generating password:", err)
		return ""
	}

	// Write the password to the file
	err = os.WriteFile(filePath, []byte(randomPassword), 0600)
	if err != nil {
		fmt.Println("Error writing to password file:", err)
		return ""
	}

	return randomPassword
}

func GetUserPassword() string {
	path := GetRcloneDirPath()
	filePath := filepath.Join(path, "user_password.txt")

	// If file exists, read and return the password
	if _, err := os.Stat(filePath); err == nil {
		data, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Println("Error reading password file:", err)
			return ""
		}
		return string(data)
	}
	return ""
}

// Checks whether the encrypted user password exists (non-empty file at given path)
func DoesUserPasswordExist() bool {
	path := GetRcloneDirPath()

	// Possible password file names
	filesToCheck := []string{
		filepath.Join(path, "user_password.txt"),
		filepath.Join(path, "user_password.txt.fcef"),
	}

	for _, filePath := range filesToCheck {
		info, err := os.Stat(filePath)
		if err == nil && info.Size() > 0 {
			return true
		}
		if err != nil && !os.IsNotExist(err) {
			fmt.Printf("Error checking file %s: %v\n", filePath, err)
		}
	}

	return false
}

func SaveUserPassword(newPassword string) error {
	path := GetRcloneDirPath()
	filePath := filepath.Join(path, "user_password.txt")

	// Check if the file already exists and is non-empty
	if info, err := os.Stat(filePath); err == nil {
		if info.Size() > 0 {
			// File exists and has content – do not overwrite
			return errors.New("password already exists and cannot be changed")
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("error checking file: %w", err)
	}

	// Ensure the directory exists
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("error creating directory: %w", err)
	}

	// Write the encrypted password (one-time only)
	err := os.WriteFile(filePath, []byte(newPassword), 0600)
	if err != nil {
		return fmt.Errorf("error writing password: %w", err)
	}

	return nil
}

func encryptFile(filePath, user_password string) error {
	_, err := app.Encrypt(filePath, v2.Passphrase(user_password))
	return err
}

func decryptFile(filePath, user_password string) error {
	_, err := app.Decrypt(filePath, v2.Passphrase(user_password))
	return err
}

func EncryptAllFilesInPath(user_password string) error {
	rootPath := GetRcloneDirPath()
	var encryptedFiles []string
	var originalFiles []string

	// First pass: Encrypt all files and collect paths
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Println("Error accessing file:", err)
			return err
		}

		if !info.IsDir() {
			encryptedPath, err := app.Encrypt(path, v2.Passphrase(user_password))
			if err != nil {
				fmt.Printf("Error encrypting file %s: %v\n", path, err)
				return err
			}
			encryptedFiles = append(encryptedFiles, encryptedPath)
			originalFiles = append(originalFiles, path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Second pass: Delete originals after all successful encryption
	for _, file := range originalFiles {
		if err := os.Remove(file); err != nil {
			fmt.Printf("Failed to delete original file %s: %v\n", file, err)
			return err
		}
		fmt.Printf("Deleted original file: %s\n", file)
	}

	return nil
}

func DecryptAllFilesInPath(user_password string) error {
	rootPath := GetRcloneDirPath()
	var decryptedFiles []string
	var encryptedFiles []string
	var passwordVerified bool

	// First pass: Decrypt all files
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Println("Error accessing file:", err)
			return err
		}

		if !info.IsDir() {
			decryptedPath, err := app.Decrypt(path, v2.Passphrase(user_password))
			if err != nil {
				fmt.Printf("Error decrypting file %s: %v\n", path, err)
				return err
			}
			decryptedFiles = append(decryptedFiles, decryptedPath)
			encryptedFiles = append(encryptedFiles, path)

			// Check password once using special file (e.g. file with "password" in name)
			if !passwordVerified && strings.Contains(filepath.Base(path), "user_password") {
				fmt.Println("password file found")
				if user_password == GetUserPassword() {
					fmt.Println("password match")
					passwordVerified = true
				} else {
					fmt.Println("password no match")
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// If password was correct: delete encrypted files
	if passwordVerified {
		for _, file := range encryptedFiles {
			if err := os.Remove(file); err != nil {
				fmt.Printf("Failed to delete encrypted file %s: %v\n", file, err)
				return err
			}
			fmt.Printf("Deleted encrypted file: %s\n", file)
		}
		return nil
	}

	// If password was incorrect: delete wrongly decrypted files
	for _, file := range decryptedFiles {
		if err := os.Remove(file); err != nil {
			fmt.Printf("Failed to delete wrongly decrypted file %s: %v\n", file, err)
			return err
		}
		fmt.Printf("Deleted invalid decrypted file: %s\n", file)
	}
	return fmt.Errorf("password verification failed. All decrypted files were deleted")
}
