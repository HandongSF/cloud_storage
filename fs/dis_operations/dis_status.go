package dis_operations

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/reedsolomon"
)

func CheckState() bool {
	flag, state, origin_name := CheckFlagAndState()
	if flag == false {
		return false
	}
	fmt.Printf("이전에 중단된 작업이 있습니다: %s - %s\n", state, origin_name)
	return true

}
func ReStartFunction() {
	flag, state, origin_name := CheckFlagAndState()
	if flag == false {
		return
	}

	// 중단된 거 있어서 재시작 여부 물어보기
	fmt.Printf("이어서 진행하시겠습니까? (yes/no):")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	if input == "yes" {
		if state == "upload" {
			reDisUpload(input, origin_name)
		}
		if state == "download" {
			reDisDownload()
		}
		if state == "rm" {
			reDisRm(origin_name)
		}
	}

	if state == "upload" {
		reDisUpload(input, origin_name)
	}
	if state == "download" {
		reDisDownload()
	}
	if state == "rm" {
		reDisRm(origin_name)
	}

	return
}

// 처음 시작하는 건지, 중단된 거 있는 지 확인하는 함수 - upload
func reDisUpload(answer string, origin_name string) {

	// 유저가 다시 업로드 원하지 않으면 그냥 rm
	if answer == "no" {
		// 기존 파일 rm 하는 로직
		err := Dis_rm([]string{origin_name})
		if err != nil {
			fmt.Printf("파일 삭제 중 오류 발생: %v\n", err)
		}
	}

	// 유저가 다시 업로드 원하면 hashed 된 파일 이름을 풀어서 그 파일들 업로드
	hashedFileName := sendHashName()
	origin_names, err := GetOriginalFileNameList(origin_name, hashedFileName)
	if err != nil {
		fmt.Printf("해쉬파일이름 변환 오류 발생 : %v\n", err)
		return
	}

	// 파일들 다시 업로드
	rr_counter := 0 // Round Robin
	remotes := config.GetRemotes()

	var mu sync.Mutex
	var errs []error
	var wg sync.WaitGroup

	for i, source := range origin_names {
		// Prepare destination for the file upload
		dest := fmt.Sprintf("%s:%s", remotes[rr_counter].Name, remoteDirectory)

		wg.Add(1)

		go func(i, rr int, source string, dest string) {
			defer wg.Done()

			fileName := filepath.Base(source)
			dir := filepath.Dir(source)

			hashedFileName, err := ConvertFileNameForUP(fileName)
			if err != nil {
				errs = append(errs, fmt.Errorf("failed to converting file name %v", err))
			}
			source = filepath.Join(dir, hashedFileName)

			// Perform the upload (Via API Call)
			err = remoteCallCopy([]string{source, dest})
			if err != nil {
				errs = append(errs, fmt.Errorf("error in remoteCallCopy for file %s: %w", source, err))
				return
			}

			// Get the full path of the shard
			// shardFullPath, err := GetFullPath(source)
			if err != nil {
				errs = append(errs, fmt.Errorf("error getting full path for %s: %w", source, err))
				return
			}

			err = UpdateDistributedFileCheckFlag(origin_name, fileName, true)
			if err != nil {
				fmt.Printf("UpdateDistributedFileCheckFlag 에러 : %v\n", err)
			}

			mu.Lock()
			// Erase Temp Shard
			reedsolomon.DeleteShardWithFileNames([]string{hashedFileName})
			mu.Unlock()
		}(i, rr_counter, source, dest)

		mu.Lock()
		rr_counter = (rr_counter + 1) % len(remotes)
		mu.Unlock()
	}

	err = ResetCheckFlag(origin_name)

}

func sendHashName() []string {
	path := GetShardPath() // 샤드 폴더 위치

	files, err := os.ReadDir(path)
	if err != nil {
		fmt.Errorf("Error reading shard directory : %v\n", err)
	}

	var hashedfileNames []string
	for _, files := range files {
		if !files.IsDir() { // 폴더가 아닌 파일 이름들만
			hashedfileNames = append(hashedfileNames, files.Name())
		}
	}

	return hashedfileNames

}

func reDisDownload() {

	fmt.Println("구현 미완성 - reDownload")
}

func reDisRm(origin_name string) {

	// 삭제해야 할 파일이름들 불러오기
	rmFiles, err := RemoveUncompletedFile(origin_name)
	if err != nil {
		fmt.Println("RemoveUncompletedFile error: %v", err)
		return
	}

	// 파일들 삭제
	start := time.Now()
	var wg sync.WaitGroup
	errCh := make(chan error, len(rmFiles))

	remoteDirectory := "Distribution"
	for _, fileName := range rmFiles {
		wg.Add(1)
		go func(fileName string) {
			defer wg.Done()

			hashedFileName, err := CalculateHash(fileName)
			if err != nil {
				errCh <- fmt.Errorf("failed to calculate hash %v", err)
			}
			remotePath := fmt.Sprintf("%s:%s/%s", fileName, remoteDirectory, hashedFileName)
			fmt.Printf("Deleting file on remote: %s\n", remotePath)

			if err := remoteCallDeleteFile([]string{remotePath}); err != nil {
				errCh <- fmt.Errorf("failed to delete %s : %w", fileName, err)
			}

			// 삭제했다면 플래그 업데이트
			UpdateDistributedFileCheckFlag(origin_name, fileName, true)
		}(fileName)
	}

	wg.Wait()
	close(errCh)

	// 모든 삭제 작업에 대한 오류를 확인.
	var deleteErrs []error
	for err := range errCh {
		deleteErrs = append(deleteErrs, err)
	}

	// 삭제 중 오류가 하나라도 발생했다면, 오류 메시지를 출력.
	if len(deleteErrs) > 0 {
		fmt.Printf("Errors occurred while deleting files: %v\n", deleteErrs)
		return
	}

	// 모든 것이 성공했다면 flag 초기화
	ResetCheckFlag(origin_name)

	elapsed := time.Since(start)
	fmt.Printf("Time taken for dis_rm: %s\n", elapsed)

	fmt.Printf("Successfully deleted all parts of %s and update metadata\n", origin_name)

	return
}
