package dis_operations

import (
	"fmt"

	"github.com/rclone/rclone/reedsolomon"
	//"sync"
)

func Dis_Download(args []string) (err error) {
	distributedFileNames, err := GetDistributedFile()
	if err != nil {
		return err
	}

	if !contains(distributedFileNames, args[0]) {
		return fmt.Errorf("file not found in remote")
	}

	// Get Distribution list
	distributedFileInfos, err := GetDistributedFileStruct(args[0])
	if err != nil {
		return err
	}

	// Get shards  via API call

	//var mu sync.Mutex
	//var wg sync.WaitGroup
	shardDir, err := reedsolomon.GetShardDir()
	if err != nil {
		return err
	}
	for _, disFileStruct := range distributedFileInfos {
		source := fmt.Sprintf("%s:%s", disFileStruct.Remote.Name, remoteDirectory)
		fmt.Printf("Downloading shard %s to %s of size %d\n", source, args[1], disFileStruct.DisFileSize)

		//wg.Add(1)
		err := remoteCallCopy([]string{source, shardDir})
		if err != nil {
			fmt.Printf("error in Dis_Upload for file %s: %v\n", source, err)
			return err
		}
	}
	// Send to erasure coding to recover

	// Move downloaded file to destination

	return nil
}

func contains(slice []string, str string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
}
