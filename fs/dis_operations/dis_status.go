package dis_operations

import (
	"fmt"

	"github.com/rclone/rclone/reedsolomon"
)

func CheckState(loadbalancer LoadBalancerType) error {
	flag, state, origin_name := CheckFlagAndState()
	if !flag {
		return nil
	}

	fmt.Printf("There is unfinished work: %s - %s\n", state, origin_name)

	var answer bool

	if state == "upload" {
		answer = DoReUpload(origin_name)
		if answer {
			// reupload
			fmt.Printf("state: %s, answer: %t\n", state, answer)
			return Dis_Upload([]string{origin_name}, true, loadbalancer)
		} else {
			// dump old file
			return DumpUploadState([]string{origin_name})
		}
	} else if state == "download" {
		answer = DoReDownload(origin_name)
		if answer {
			//redownload
			return Dis_Download([]string{origin_name}, true)
		} else {
			// dump old file
			return DumpDownloadState([]string{origin_name})
		}
	} else if state == "rm" {
		// dump as default
		return DumpRmState([]string{origin_name})
	}

	return nil

}

func DumpRmState(args []string) (err error) {
	// Remove shards in remote and info in datamap
	err = Dis_rm(args, true)
	if err != nil {
		return err
	}

	return nil
}

func DumpDownloadState(args []string) (err error) {
	// Dump Shards in Shards Directory
	err = DumpDownloadShards(args)
	if err != nil {
		return err
	}

	// Reset flags to false so it's not triggered next time
	err = ResetCheckFlag(args[0])
	if err != nil {
		return err
	}

	return nil
}

func DumpUploadState(args []string) (err error) {
	// Dump Shards in Shards Directory
	err = DumpUploadShards(args)
	if err != nil {
		return err
	}

	// Remove shards in remote and info in datamap
	err = Dis_rm(args, false)
	if err != nil {
		return err
	}

	return nil
}

func DumpUploadShards(args []string) (err error) {
	var shardsToDump = []string{}
	distributedFiles, err := GetDistributedFileStruct(args[0])
	if err != nil {
		return err
	}
	var errs []error
	for _, distributedFile := range distributedFiles {
		if !distributedFile.Check {
			hashVal, temp_err := CalculateHash(distributedFile.DistributedFile)
			fmt.Println("Shard to dump is:" + hashVal)
			if temp_err != nil {
				errs = append(errs, temp_err)
			}
			shardsToDump = append(shardsToDump, hashVal)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors occurred during hashing: %v", errs)
	}
	reedsolomon.DeleteShardWithFileNames(shardsToDump)

	return nil
}

func DumpDownloadShards(args []string) (err error) {
	var shardsToDump = []string{}
	distributedFiles, err := GetDistributedFileStruct(args[0])
	if err != nil {
		return err
	}

	for _, distributedFile := range distributedFiles {
		if !distributedFile.Check {
			shardsToDump = append(shardsToDump, distributedFile.DistributedFile)
		}
	}

	reedsolomon.DeleteShardWithFileNames(shardsToDump)

	return nil
}
