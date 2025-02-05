package dis_operations

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// 처음 시작하는 건지, 중단된 거 있는 지 확인하는 함수 - upload
func checkUploadState() bool {
	flag, state := CheckFlagAndState()
	if flag == false {
		return false
	}

	// 중단된 거 있어서 재시작 여부 물어보기
	fmt.Printf("이전에 중단된 작업이 있습니다: %s\n 이어서 진행하시겠습니까? (yes/no):", state)

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	if input == "yes" {
		return true
	}

	// 기존 파일 rm 하는 로직 추가 필요

	return false

}

func sendHashName(state string) []string {
	path := GetShardPath() // 샤드 폴더 위치

	files, err := os.ReadDir(path)
	if err != nil {
		fmt.Errorf("Error reading shard directory : %v\n", err)
	}

	var fileNames []string
	for _, files := range files {
		if !files.IsDir() { // 폴더가 아닌 파일 이름들만
			fileNames = append(fileNames, files.Name())
		}
	}

	return fileNames

}
