package dis_operations

import (
	"fmt"
	"os"
	"path/filepath"
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

	var checksums = []string{"40cadf18f34e25aded48efe68f1479ef5b24a885f4840f59d47175b33cf3a0d3"}

	distributedFile, err := GetDistributedInfo(fileName, remote, checksums[0])
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	// if distributedFile.DisFileSize != 0 {
	// 	t.Errorf("Expected file size to be 0, got: %d", distributedFile.DisFileSize)
	// }

	if distributedFile.Remote != remote {
		t.Errorf("Expected remote: %+v, but got: %+v", remote, distributedFile.Remote)
	}

}

func TestMakeDataMap(t *testing.T) {
	tempFile, err := os.CreateTemp("", "testfile_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	_, err = tempFile.WriteString("This is a test file!!")
	if err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()

	distributedFiles := []DistributedFile{
		{
			DistributedFile: "test_distributed_file",
			Remote: Remote{
				Name: "remote_server",
				Type: "S3",
			},
		},
	}

	err = MakeDataMap(tempFile.Name(), distributedFiles, 0, 0, 10, 10)
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	jsonFilePath, err := os.Getwd()
	if err != nil {
		fmt.Errorf("failed to find Path: %v", err)
	}
	jsonFilePath = filepath.Join(jsonFilePath, "data", "datamap.json")
	if _, err := os.Stat(jsonFilePath); os.IsNotExist(err) {
		t.Errorf("Expected JSON file to be created at %s, but it does not exist", jsonFilePath)
	}

}

func TestCalculateChecksum(t *testing.T) {
	// you have to change on your side!!
	tempFile, err := os.CreateTemp("", "testfile_*.txt")
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

func TestGetRcloneDirPath(t *testing.T) {
	fmt.Printf(GetRcloneDirPath())
}

func TestGetChecksumList(t *testing.T) {
	checksums := GetChecksumList("IMG_6146.JPG")
	for idx, checksum := range checksums {
		fmt.Println(idx, " ", checksum)
	}
}
