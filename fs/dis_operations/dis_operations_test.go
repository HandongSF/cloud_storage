package dis_operations

import (
	"fmt"
	"os"
	"testing"
)

func TestGetDistributedInfo(t *testing.T) {
	tempFile, err := os.CreateTemp("", "testfile_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	fileName := tempFile.Name()
	remote := Remote{
		Name: "gdrive",
		Type: "drive",
	}

	distributedFile, err := GetDistributedInfo(fileName, remote)
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	// if distributedFile.DisFileSize != 0 {
	// 	t.Errorf("Expected file size to be 0, got: %d", distributedFile.DisFileSize)
	// }

	if distributedFile.remote != remote {
		t.Errorf("Expected remote: %+v, but got: %+v", remote, distributedFile.remote)
	}

}

func TestMakeDataMap(t *testing.T) {
	tempFile, err := os.CreateTemp("", "testfile_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	_, err = tempFile.WriteString("This is a test file for checksum")
	if err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()

	distributedFiles := []DistributedFile{
		{
			DistributedFile: "test_distributed_file",
			DisFileSize:     123,
			remote: Remote{
				Name: "remote_server",
				Type: "S3",
			},
		},
	}

	err = MakeDataMap(tempFile.Name(), distributedFiles)
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}
	// you have to change on your side!!
	jsonFilePath := "/Users/iyeeun/Desktop"
	if _, err := os.Stat(jsonFilePath); os.IsNotExist(err) {
		t.Errorf("Expected JSON file to be created at %s, but it does not exist", jsonFilePath)
	}

}

func TestCalculateChecksum(t *testing.T) {
	// you have to change on your side!!
	tempFile, err := os.CreateTemp("/Users/iyeeun/Desktop", "testfile_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	content := "This is a test file for checksum."
	_, err = tempFile.WriteString(content)
	if err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()

	expectedChecksum := "e200bd66430fb559c1a0d6322fe3a154e2ee200a6f113d66a60ce2605ddb88bc"
	checksum, err := calculateChecksum(tempFile.Name())
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	if checksum != expectedChecksum {
		t.Errorf("Expected checksum: %s, but got: %s", expectedChecksum, checksum)
	}
}

func TestGetDistributedFile(t *testing.T) {
	listOfFile, err := GetDistributedFile()
	if err == nil {
		fmt.Printf(listOfFile)
	} else {
		t.Errorf("get distributed file name failed %v", err)
	}
}

// func TestFullSequence(t *testing.T) {
// 	original_img_path :=
// }
