package dis_operations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/spf13/cobra"
)

func LoadBalancer_RoundRobin() (config.Remote, error) {
	jsonFilePath := getLoadBalancerJsonFilePath()
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

func LoadBalancer_LeastConnected() (config.Remote, error) {
	jsonFilePath := getLoadBalancerJsonFilePath()
	existingLBInfo, err := readLoadBalancerJsonFile(jsonFilePath)
	if err != nil {
		return config.Remote{}, err
	}

	remote := getKeyOfSmallestValue(existingLBInfo.RemoteConnectionCounter)
	IncrementRemoteConnectionCounter(remote)

	return remote, nil
}

func LoadBalancer_ResourceBased() (config.Remote, error) {
	remotes := config.GetRemotes()
	var errs []error
	var wg sync.WaitGroup
	var mu sync.Mutex // To protect shared variables

	var bestRemote config.Remote
	var maxFreeStorage int64

	for _, remote := range remotes {
		wg.Add(1)
		go func(remote config.Remote) {
			defer wg.Done()

			val, err := remoteCallAbout([]string{remote.Name})
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("error in remoteCallAbout for remote %s: %w", remote.Name, err))
				mu.Unlock()
				return
			}

			mu.Lock()
			if val > maxFreeStorage {
				maxFreeStorage = val
				bestRemote = remote
			}
			mu.Unlock()

		}(remote)
	}

	wg.Wait()

	// If there were errors and no valid remote found, return an error
	if len(errs) == len(remotes) {
		return config.Remote{}, fmt.Errorf("all remotes failed: %v", errs)
	}

	return bestRemote, nil
}

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

func getKeyOfSmallestValue(counter map[config.Remote]int) config.Remote {
	var minKey config.Remote
	var minValue int
	firstIteration := true

	for key, value := range counter {
		if firstIteration || value < minValue {
			minValue = value
			minKey = key
			firstIteration = false
		}
	}

	return minKey
}

var aboutCommandDefinitionForRemoteCall = &cobra.Command{
	Use: "about remote:",
	Annotations: map[string]string{
		"versionIntroduced": "v1.41",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		f := cmd.NewFsSrc(args)

		cmd.Run(false, false, command, func() error {
			freeStorage, err := getFreeStorage(f)
			if err != nil {
				return err
			}

			// Print free storage
			fmt.Printf("Free Storage: %v\n", freeStorage)

			// Store free storage in the command context for later retrieval
			command.SetContext(context.WithValue(command.Context(), "freeStorage", freeStorage))

			return nil
		})
	},
}

// Function to call the new command definition and return free storage
func remoteCallAbout(args []string) (int64, error) {
	fmt.Printf("Calling remoteCallAbout with args: %v\n", args)

	// Create a new command instance
	aboutCommand := *aboutCommandDefinitionForRemoteCall
	aboutCommand.SetArgs(args)

	// Execute the command
	err := aboutCommand.Execute()
	if err != nil {
		return 0, fmt.Errorf("error executing aboutCommand: %w", err)
	}

	// Retrieve free storage from the command context
	freeStorage, ok := aboutCommand.Context().Value("freeStorage").(int64)
	if !ok {
		return 0, errors.New("failed to retrieve free storage")
	}

	return freeStorage, nil
}

// Helper function to get free storage
func getFreeStorage(f fs.Fs) (int64, error) {
	doAbout := f.Features().About
	if doAbout == nil {
		return 0, fmt.Errorf("%v doesn't support about", f)
	}

	u, err := doAbout(context.Background())
	if err != nil {
		return 0, fmt.Errorf("about call failed: %w", err)
	}
	if u == nil {
		return 0, errors.New("nil usage returned")
	}

	// Return free storage
	return *u.Free, nil
}
