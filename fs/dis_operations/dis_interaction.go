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

func DoOverwrite() bool {
	switch i := config.CommandDefault([]string{"yYes overwrite this file", "nNo skip the file"}, 0); i {
	case 'y':
		return true
	case 'n':
		return false
	default:
		fmt.Printf("Invalid Input!\n")
		fmt.Printf("Do overwrite the file?\n")
		return DoOverwrite()
	}
}
