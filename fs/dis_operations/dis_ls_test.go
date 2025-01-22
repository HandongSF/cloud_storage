package dis_operations

import (
	"fmt"
	"testing"
)

func TestGetDistributedFile(t *testing.T) {
	listOfFile, err := Dis_ls()
	if err == nil {
		for idx, name := range listOfFile {
			fmt.Printf("%d : %s\n", idx+1, name)
		}
	} else {
		t.Errorf("get distributed file name failed %v", err)
	}
}
