package dis_operations

import (
	"fmt"

	"github.com/rclone/rclone/fs/config"
)

// ShowRemotes shows an overview of the config file
func ShowDescription(filename string) bool {
	fmt.Printf("A duplicate of file %s already exists in remote.\n", filename)
	fmt.Println()
	fmt.Printf("Do overwrite the file?\n")
	return DoOverwrite()
}

func ShowDescription_RemoveFile(filename string, err error) bool {
	fmt.Printf("Error occured during decoding file %s\n", filename)
	fmt.Printf("%s\n", err)
	fmt.Println()
	fmt.Printf("Remove file from remote completely? (This will effectively call dis_rm <file_name>)\n")
	return DoRemove()
}

func AskDestination() string {
	var dest string
	fmt.Print("Enter path to download file: ")
	_, err := fmt.Scanln(&dest)
	if err != nil {
		fmt.Printf("Error reading input: %v\n", err)
	}
	return dest
}

func GetUserConfirmation(prompt string, options []string, defaultIndex int) bool {
	switch i := config.CommandDefault(options, defaultIndex); i {
	case 'y':
		return true
	case 'n':
		return false
	default:
		fmt.Printf("Invalid Input!\n")
		fmt.Printf("%s\n", prompt)
		return GetUserConfirmation(prompt, options, defaultIndex)
	}
}

func DoOverwrite() bool {
	return GetUserConfirmation("Do you want to overwrite the file?", []string{"yYes overwrite this file", "nNo skip the file"}, 0)
}

func DoRemove() bool {
	return GetUserConfirmation("Do you want to remove the file?", []string{"yYes remove this file", "nNo keep the file"}, 0)
}

func DoReUpload(fileName string) bool {
	return GetUserConfirmation("Do you want to reupload the "+fileName+" ?", []string{"yYes reupload the file", "nNo remove the file"}, 0)
}

func DoReDownload(fileName string) bool {
	return GetUserConfirmation("Do you want to redownload the "+fileName+" ?", []string{"yYes redownload the file", "nNo remove the file"}, 0)
}
