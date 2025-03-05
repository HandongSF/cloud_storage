package dis_operations

import (
	"fmt"
)

var remoteDirectory = "Distribution"

// The Top Data Structure
type FileInfo struct {
	FileName             string                     `json:"original_file_name"`
	FileSize             int64                      `json:"original_file_size"`
	DisFileSize          int64                      `json:"distributed_file_size"`
	Flag                 bool                       `json:"flag"`
	State                string                     `json:"state"`
	Checksum             string                     `json:"checksum"`
	Padding              int64                      `json:"padding_amount"`
	DistributedFileInfos map[string]DistributedFile `json:"distributed_file_infos"`
}

type DistributedFile struct {
	DistributedFile string `json:"distributed_file_name"`
	Remote          Remote `json:"remote"`
	Checksum        string `json:"dis_checksum"`
	Check           bool   `json:"state_check"`
}

type Remote struct {
	Name string `json:"remote_name"`
	Type string `json:"remote_type"`
}

type BoltzmannInfo struct {
	//RecentSpeeds   []float64      `json:"recent_speeds"` // Stores recent upload speeds (e.g., last 5 uploads)
	//MaxSpeed       float64        `json:"max_speed"`     // Maximum observed throughput (moving average)
	ShardCount int `json:"shard_count"`
	//FileShardCount map[string]int `json:"file_shard_count"` // Tracks shard concentration per file
	//Penalty        float64        `json:"penalty"`          // Penalty factor for unsafe conditions
}

type LoadBalancerInfo struct {
	RoundRobinCounter    int                      `json:"RoundRobin_Counter"`
	RemoteBoltzmannInfos map[string]BoltzmannInfo `json:"Remote_Bolzmann_Info"`
}

func (r Remote) String() string {
	return fmt.Sprintf("%s|%s", r.Name, r.Type) // Use a separator to avoid conflicts
}

func (b *BoltzmannInfo) IncrementShardCount() {
	b.ShardCount++
}

func (b *BoltzmannInfo) DecrementShardCount() {
	if b.ShardCount > 0 {
		b.ShardCount--
	}
}

// func (b *BoltzmannInfo) UpdateRecentSpeed(newSpeed float64, maxEntries int) {
// 	b.RecentSpeeds = append(b.RecentSpeeds, newSpeed)

// 	// Keep only the last `maxEntries` speeds
// 	if len(b.RecentSpeeds) > maxEntries {
// 		b.RecentSpeeds = b.RecentSpeeds[len(b.RecentSpeeds)-maxEntries:]
// 	}

// 	b.UpdateMaxSpeed()
// }

// func (b *BoltzmannInfo) UpdateMaxSpeed() {
// 	if len(b.RecentSpeeds) == 0 {
// 		b.MaxSpeed = 0
// 		return
// 	}

// 	// Find the maximum speed
// 	max := b.RecentSpeeds[0]
// 	for _, speed := range b.RecentSpeeds {
// 		if speed > max {
// 			max = speed
// 		}
// 	}
// 	b.MaxSpeed = max
// }

// func (b *BoltzmannInfo) UpdateFileShardCount(file string, count int) {
// 	if b.FileShardCount == nil {
// 		b.FileShardCount = make(map[string]int)
// 	}
// 	b.FileShardCount[file] += count
// }

// func (b *BoltzmannInfo) RemoveFileShard(file string) {
// 	delete(b.FileShardCount, file)
// }

// func (b *BoltzmannInfo) ApplyPenalty(penaltyAmount float64) {
// 	b.Penalty += penaltyAmount
// }

func (distributionFile *DistributedFile) AllocateRemote(loadbalancer LoadBalancerType) error {
	var remote Remote
	var err error

	switch loadbalancer {
	case RoundRobin:
		remote, err = LoadBalancer_RoundRobin()
	case LeastDistributed:
		remote, err = LoadBalancer_LeastDistributed()
	case ResourceBased:
		remote, err = LoadBalancer_ResourceBased()
	default:
		remote, err = LoadBalancer_RoundRobin()
	}

	if err != nil {
		return err
	}
	distributionFile.Remote = remote
	return nil
}

func (boltzmannInfo *BoltzmannInfo) PrintInfo() {
	fmt.Println()
	//fmt.Printf("Max Speed: %f\n", boltzmannInfo.MaxSpeed)
	fmt.Printf("Shard Count: %d\n", boltzmannInfo.ShardCount)
	//fmt.Printf("Penalty: %f\n", boltzmannInfo.Penalty)
	//fmt.Println("Recent Speeds:", boltzmannInfo.RecentSpeeds)
	//fmt.Println("File Shard Count:", boltzmannInfo.FileShardCount)
	fmt.Println()
}
