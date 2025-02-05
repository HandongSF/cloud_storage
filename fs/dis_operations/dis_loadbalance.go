package dis_operations

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rclone/rclone/fs/config"
)

func LoadBalancer_RoundRobin() (config.Remote, error) {
	jsonFilePath := getJsonFilePath()
	existingLBInfo, err := readLoadBalancerJsonFile(jsonFilePath)
	if err != nil {
		return config.Remote{}, err
	}

	remotes := config.GetRemotes()
	var remote = remotes[existingLBInfo.RoundRobinCounter%len(remotes)]

	IncrementRoundRobinCounter()
	IncrementRemoteConnectionCounter(remote)

	return remote, nil
}

// func LoadBalancer_LeastConnected() config.Remote {

// }

// func LoadBalancer_ResourcedBased() config.Remote {

// }

func IncrementRoundRobinCounter() error {
	jsonFilePath := getJsonFilePath()

	existingLBInfo, err := readLoadBalancerJsonFile(jsonFilePath)
	if err != nil {
		return err
	}

	existingLBInfo.RoundRobinCounter++

	return writeLoadBalancerJsonFile(jsonFilePath, existingLBInfo)
}

func IncrementRemoteConnectionCounter(remote config.Remote) error {
	jsonFilePath := getJsonFilePath()

	existingLBInfo, err := readLoadBalancerJsonFile(jsonFilePath)
	if err != nil {
		return err
	}

	if existingLBInfo.RemoteConnectionCounter == nil {
		existingLBInfo.RemoteConnectionCounter = make(map[config.Remote]int)
	}

	// Update the counter for the given remote ID
	existingLBInfo.RemoteConnectionCounter[remote]++

	// Write the updated data back to the file
	return writeLoadBalancerJsonFile(jsonFilePath, existingLBInfo)
}

func getLoadBalancerJsonFilePath() string {
	path := GetRcloneDirPath()
	return filepath.Join(path, "data", "loadbalancer.json")
}

func readLoadBalancerJsonFile(filePath string) (LoadBalancerInfo, error) {
	file, err := os.Open(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return LoadBalancerInfo{}, nil // Return empty slice if file does not exist
		}
		return LoadBalancerInfo{}, fmt.Errorf("failed to open JSON file: %v", err)
	}
	defer file.Close()

	var info LoadBalancerInfo
	if err := json.NewDecoder(file).Decode(info); err != nil && !errors.Is(err, io.EOF) {
		return LoadBalancerInfo{}, fmt.Errorf("failed to decode JSON: %v", err)
	}
	return info, nil
}

func writeLoadBalancerJsonFile(filePath string, data LoadBalancerInfo) error {
	if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}

	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write JSON file: %v", err)
	}
	return nil
}
