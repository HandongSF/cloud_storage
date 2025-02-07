package dis_operations

import (
	"fmt"
)

func CheckState() error {
	flag, state, origin_name := CheckFlagAndState()
	if flag == false {
		return nil
	}

	fmt.Printf("이전에 중단된 작업이 있습니다: %s - %s\n", state, origin_name)

	var answer bool

	if state == "upload" {
		answer = DoReUpload(origin_name)
		if answer {
			//reupload할거임
			fmt.Printf("state: %s, answer: %t\n", state, answer)
			return Dis_Upload([]string{origin_name}, true)
		} else {
			//reupload 안 함
			// 기존에 있는 reupload 지움 = remote에 올라가있는 파일 지우고, datamap도 지우고

		}
	} else if state == "download" {
		answer = DoReDownload(origin_name)
		if answer {
			//redownload함
			return Dis_Download([]string{origin_name}, true)
		} else {
			//redownload 안 함
			// 기존에 있는 redownload 지움 = shard에 있는 파일 지우고, datamap check flag 정상화
		}
	} else if state == "rm" {
		//무조건 지우던거 마져 지움
		return nil
	}

	return nil

}
