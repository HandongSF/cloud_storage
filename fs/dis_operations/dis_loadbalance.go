package dis_operations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/spf13/cobra"
)

type LoadBalancerType string

const (
	RoundRobin       LoadBalancerType = "RoundRobin"
	LeastDistributed LoadBalancerType = "LeastDistributed"
	ResourceBased    LoadBalancerType = "ResourceBased"
	None             LoadBalancerType = "None" // Invalid value
)

// Validate the input for load balancer
func (lb LoadBalancerType) IsValid() bool {
	switch lb {
	case RoundRobin, LeastDistributed, ResourceBased:
		return true
	default:
		return false
	}
}

func LoadBalancer_RoundRobin() (Remote, error) {
	jsonFilePath := getLoadBalancerJsonFilePath()
	existingLBInfo, err := readJSON(jsonFilePath)
	if err != nil {
		return Remote{}, err
	}

	remotes := config.GetRemotes()
	if len(remotes) == 0 {
		return Remote{}, fmt.Errorf("no available remotes")
	}

	// Select a remote using Round Robin
	selectedRemote := remotes[existingLBInfo.RoundRobinCounter%len(remotes)]
	selectedRemoteObj := Remote{selectedRemote.Name, selectedRemote.Type}

	// Increment counters
	IncrementRoundRobinCounter()
	IncrementRemoteConnectionCounter(selectedRemoteObj)

	return selectedRemoteObj, nil
}

func LoadBalancer_LeastDistributed() (Remote, error) {
	jsonFilePath := getLoadBalancerJsonFilePath()
	existingLBInfo, err := readJSON(jsonFilePath)
	if err != nil {
		return Remote{}, err
	}

	remote := getKeyOfSmallestValue(existingLBInfo.RemoteConnectionCounter)
	IncrementRemoteConnectionCounter(remote)

	return remote, nil
}

var bestRemote_save = Remote{"", ""}

func LoadBalancer_ResourceBased() (Remote, error) {
	if bestRemote_save.Name != "" && bestRemote_save.Type != "" {
		return bestRemote_save, nil
	}

	remotes := config.GetRemotes()
	var errs []error
	var wg sync.WaitGroup
	var mu sync.Mutex // To protect shared variables

	var bestRemote Remote
	var maxFreeStorage int64

	for _, remote := range remotes {
		wg.Add(1)
		go func(remote config.Remote) {
			defer wg.Done()

			val, err := remoteCallAbout([]string{remote.Name + ":"})
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("error in remoteCallAbout for remote %s: %w", remote.Name, err))
				mu.Unlock()
				return
			}

			mu.Lock()
			if val > maxFreeStorage {
				maxFreeStorage = val
				bestRemote = Remote{remote.Name, remote.Type}
			}
			mu.Unlock()

		}(remote)
	}

	wg.Wait()

	// If there were errors and no valid remote found, return an error
	if len(errs) == len(remotes) {
		return Remote{}, fmt.Errorf("all remotes failed: %v", errs)
	}

	bestRemote_save = bestRemote

	return bestRemote, nil
}

func IncrementRoundRobinCounter() error {
	jsonFilePath := getLoadBalancerJsonFilePath()
	existingLBInfo, err := readJSON(jsonFilePath)
	if err != nil {
		return err
	}

	existingLBInfo.RoundRobinCounter = (existingLBInfo.RoundRobinCounter + 1)

	return writeJSON(jsonFilePath, existingLBInfo)
}

func DecrementRemoteConnectionCounter(distributedFileArray []DistributedFile) error {
	jsonFilePath := getLoadBalancerJsonFilePath()

	existingLBInfo, err := readJSON(jsonFilePath)
	if err != nil {
		return err
	}

	remoteCounter := make(map[string]int)

	for _, disdistributedFile := range distributedFileArray {
		remoteCounter[disdistributedFile.Remote.String()]++
	}

	for remoteKey, occurrence := range remoteCounter {
		if _, exists := existingLBInfo.RemoteConnectionCounter[remoteKey]; !exists {
			existingLBInfo.RemoteConnectionCounter[remoteKey] = 0
		}

		// Decrement but prevent negative values
		if existingLBInfo.RemoteConnectionCounter[remoteKey] < occurrence {
			existingLBInfo.RemoteConnectionCounter[remoteKey] = 0
		} else {
			existingLBInfo.RemoteConnectionCounter[remoteKey] -= occurrence
		}
	}

	err = writeJSON(jsonFilePath, existingLBInfo)
	if err != nil {
		return fmt.Errorf("failed to write JSON: %w", err)
	}

	return nil
}

func IncrementRemoteConnectionCounter(remote Remote) error {
	jsonFilePath := getLoadBalancerJsonFilePath()

	existingLBInfo, err := readJSON(jsonFilePath)
	if err != nil {
		return err
	}

	if existingLBInfo.RemoteConnectionCounter == nil {
		existingLBInfo.RemoteConnectionCounter = make(map[string]int)
	}

	remoteKey := remote.String()

	existingLBInfo.RemoteConnectionCounter[remoteKey]++

	err = writeJSON(jsonFilePath, existingLBInfo)
	if err != nil {
		return fmt.Errorf("failed to write JSON: %w", err)
	}

	return nil
}

func getLoadBalancerJsonFilePath() string {
	// Get the directory path (you will need to implement GetRcloneDirPath based on your application logic)
	path := GetRcloneDirPath()

	// Construct the file path
	filePath := filepath.Join(path, "data", "loadbalancer.json")

	// Check if the file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// Create the directories if they don't exist
		err := os.MkdirAll(filepath.Dir(filePath), 0755)
		if err != nil {
			fmt.Println("Error creating directories:", err)
			return ""
		}

		// Create the JSON file
		file, err := os.Create(filePath)
		if err != nil {
			fmt.Println("Error creating file:", err)
			return ""
		}
		defer file.Close()

		// Initialize LoadBalancerInfo with default values
		lbInfo := LoadBalancerInfo{
			RoundRobinCounter:       0,
			RemoteConnectionCounter: make(map[string]int),
		}

		// Marshal the LoadBalancerInfo struct to JSON format
		data, err := json.MarshalIndent(lbInfo, "", "  ")
		if err != nil {
			fmt.Println("Error marshaling data:", err)
			return ""
		}

		// Write the initialized data to the file
		_, err = file.Write(data)
		if err != nil {
			fmt.Println("Error writing to file:", err)
			return ""
		}
	}

	return filePath
}

func readJSON(filename string) (*LoadBalancerInfo, error) {
	// Open the file
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Read the file contents
	var lbInfo LoadBalancerInfo
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&lbInfo)
	if err != nil {
		return nil, err
	}

	return &lbInfo, nil
}

func writeJSON(filename string, lbInfo *LoadBalancerInfo) error {
	data, err := json.MarshalIndent(lbInfo, "", "  ")
	if err != nil {
		return err
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return err
	}

	return nil
}

func getKeyOfSmallestValue(counter map[string]int) Remote {
	var minKey string
	var minValue int
	firstIteration := true

	// Find the key with the smallest value
	for key, value := range counter {
		if firstIteration || value < minValue {
			minValue = value
			minKey = key
			firstIteration = false
		}
	}

	// Handle empty counter case
	if minKey == "" {
		return Remote{} // Return an empty Remote struct if the map is empty
	}

	// Split the key back into Name and Type
	parts := strings.Split(minKey, "|")
	if len(parts) != 2 {
		return Remote{}
	}

	return Remote{
		Name: parts[0],
		Type: parts[1],
	}
}

var aboutCommandDefinitionForRemoteCall = &cobra.Command{
	Use: "about remote:",
	Annotations: map[string]string{
		"versionIntroduced": "v1.41",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		f := cmd.NewFsSrc(args)

		cmd.RunWithSustainOS(false, false, command, func() error {
			freeStorage, err := getFreeStorage(f)
			if err != nil {
				return err
			}

			// Print free storage
			fmt.Printf("Remote %s Free Storage: %v\n", args[0], freeStorage)

			// Store free storage in the command context for later retrieval
			command.SetContext(context.WithValue(command.Context(), "freeStorage", freeStorage))

			return nil
		}, true)
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
