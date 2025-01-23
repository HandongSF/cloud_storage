package dis_operations

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/operations"
	rsync "github.com/rclone/rclone/fs/sync"
	"github.com/rclone/rclone/reedsolomon"
	"github.com/spf13/cobra"
)

func Dis_Upload(args []string) (err error) {
	// Check if file exists
	absolutePath, err := dis_init(args[0])
	if err != nil {
		return err
	}

	// Try to get File
	isDuplicate, err := DoesFileStructExist(args[0])
	if err != nil {
		return err
	}

	if isDuplicate {
		fmt.Printf("Duplicate exists for file: %s", args[0])
		return nil
	}

	dis_names, shardSize, padding := reedsolomon.DoEncode(absolutePath)
	fmt.Printf("%d\n", padding)
	remotes := config.GetRemotes()
	distributedFileArray := make([]DistributedFile, len(dis_names))
	rr_counter := 0 // Round Robin

	err = MakeDistributionDir(remotes)
	if err != nil {
		return err
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	var errs []error

	start := time.Now()

	for i, source := range dis_names {
		// Prepare destination for the file upload
		dest := fmt.Sprintf("%s:%s", remotes[rr_counter].Name, remoteDirectory)
		fmt.Printf("Uploading file %s to %s of size %d\n", source, dest, shardSize)

		wg.Add(1)

		go func(i, rr int, source string, dest string) {
			defer wg.Done()

			// Perform the upload (Via API Call)
			err := remoteCallCopy([]string{source, dest})
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

			// Get the distributed info for the shard
			distributionFile, err := GetDistributedInfo(source, Remote{remotes[rr].Name, remotes[rr].Type})
			if err != nil {
				errs = append(errs, fmt.Errorf("error in GetDistributedInfo for %s: %w", source, err))
				return
			}

			mu.Lock()
			distributedFileArray[i] = distributionFile
			mu.Unlock()
		}(i, rr_counter, source, dest)

		mu.Lock()
		rr_counter = (rr_counter + 1) % len(remotes)
		mu.Unlock()
	}

	wg.Wait()

	elapsed := time.Since(start)
	fmt.Printf("Time taken for dis_uplaod: %s\n", elapsed)

	if len(errs) > 0 {
		return fmt.Errorf("errors occurred: %v", errs)
	}

	// Get the full path for the original file
	originalFileFullPath, err := getAbsolutePath(args[0])
	if err != nil {
		return err
	}

	// Make the data map using the distributed files
	err = MakeDataMap(originalFileFullPath, distributedFileArray, padding)
	if err != nil {
		return err
	}

	// Erase Temp Shards
	reedsolomon.DeleteShardDir()

	fmt.Printf("Completed Dis_Upload!\n")
	return nil
}

func MakeDistributionDir(remotes []config.Remote) (err error) {
	var wg sync.WaitGroup
	var errs []error
	for _, remote := range remotes {
		argument := fmt.Sprintf("%s:%s", remote.Name, remoteDirectory)
		wg.Add(1)

		go func(arg string) {
			defer wg.Done()

			err := remoteCallMkdir([]string{arg})
			if err != nil {
				errs = append(errs, fmt.Errorf("error creating directory at %s: %w", arg, err))
				return
			}
		}(argument)
	}

	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("errors occurred: %v", errs)
	}

	return nil
}

func remoteCallCopy(args []string) (err error) {
	fmt.Printf("Calling remoteCallCopy with args: %v\n", args)

	// Fetch the copy command
	copyCommand := *copyCommandDefinition
	copyCommand.SetArgs(args)

	err = copyCommand.Execute()
	if err != nil {
		return fmt.Errorf("error executing copyCommand: %w", err)
	}

	return nil
}

var (
	createEmptySrcDirs = false
)

var copyCommandDefinition = &cobra.Command{
	Use: "copy source:path dest:path",
	Annotations: map[string]string{
		"groups": "Copy,Filter,Listing,Important",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(2, 2, command, args)
		fsrc, srcFileName, fdst := cmd.NewFsSrcFileDst(args)
		cmd.RunWithSustainOS(true, true, command, func() error {
			if srcFileName == "" {
				return rsync.CopyDir(context.Background(), fdst, fsrc, createEmptySrcDirs)
			}
			return operations.CopyFile(context.Background(), fdst, fsrc, srcFileName, srcFileName)
		}, true)
	},
}

func remoteCallMkdir(args []string) (err error) {
	fmt.Printf("Calling remoteCallMkdir with args: %v\n", args)

	// Fetch the copy command
	copyCommand := *mkdirCommandDefinition
	copyCommand.SetArgs(args)

	err = copyCommand.Execute()
	if err != nil {
		return fmt.Errorf("error executing mkdirCommand: %w", err)
	}

	return nil
}

var mkdirCommandDefinition = &cobra.Command{
	Use:   "mkdir remote:path",
	Short: `Make the path if it doesn't already exist.`,
	Annotations: map[string]string{
		"groups": "Important",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fdst := cmd.NewFsDir(args)
		if !fdst.Features().CanHaveEmptyDirectories && strings.Contains(fdst.Root(), "/") {
			fs.Logf(fdst, "Warning: running mkdir on a remote which can't have empty directories does nothing")
		}
		cmd.RunWithSustainOS(true, false, command, func() error {
			return operations.Mkdir(context.Background(), fdst, "")
		}, true)
	},
}

func dis_init(arg string) (string, error) {
	// Use the existing getAbsolutePath function to resolve the absolute path
	absolutePath, err := getAbsolutePath(arg)
	if err != nil {
		fmt.Println("Error resolving the absolute path:", err)
		return "", err
	}

	// Check if the file exists
	if _, err := os.Stat(absolutePath); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("File does not exist:", absolutePath)
			return "", fmt.Errorf("file does not exist: %s", absolutePath)
		}
		// Handle other errors (e.g., permission issues)
		fmt.Println("Error checking file:", err)
		return "", err
	}

	// If the file exists, print success message
	fmt.Println("Success: File found at", absolutePath)
	return absolutePath, nil
}
