package dis_config

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/rclone/rclone/cmd"
	rsync "github.com/rclone/rclone/cmd/sync"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/operations"
	operationsflags "github.com/rclone/rclone/fs/operations/operationsflags"
	rclsync "github.com/rclone/rclone/fs/sync" // alias: rclsync
	"github.com/spf13/cobra"
)

var (
	createEmptySrcDirs = false
	opt                = operations.LoggerOpt{}
	loggerFlagsOpt     = operationsflags.AddLoggerFlagsOptions{}
)

var syncCommandDefinition = &cobra.Command{
	Use: "sync source:path dest:path",
	Annotations: map[string]string{
		"groups": "Sync,Copy,Filter,Listing,Important",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(2, 2, command, args)
		fsrc, srcFileName, fdst := cmd.NewFsSrcFileDst(args)
		cmd.RunWithSustainOS(true, true, command, func() error {
			ctx := context.Background()
			opt, close, err := rsync.GetSyncLoggerOpt(ctx, fdst, command)
			if err != nil {
				return err
			}
			defer close()

			if anyNotBlank(loggerFlagsOpt.Combined, loggerFlagsOpt.MissingOnSrc, loggerFlagsOpt.MissingOnDst,
				loggerFlagsOpt.Match, loggerFlagsOpt.Differ, loggerFlagsOpt.ErrFile, loggerFlagsOpt.DestAfter) {
				ctx = operations.WithSyncLogger(ctx, opt)
			}

			if srcFileName == "" {
				// 동기화: source(내용) → destination (destination만 변경)
				return rclsync.Sync(ctx, fdst, fsrc, createEmptySrcDirs)
			}
			// 파일인 경우 fallback: 파일 복사
			return operations.CopyFile(ctx, fdst, fsrc, srcFileName, srcFileName)
		}, true)
	},
}

func anyNotBlank(s ...string) bool {
	for _, x := range s {
		if x != "" {
			return true
		}
	}
	return false
}

func remoteCallSync(args []string) error {
	fmt.Printf("Calling remoteCallSync with args: %v\n", args)

	syncCmd := *syncCommandDefinition
	syncCmd.SetArgs(args)

	if err := syncCmd.Execute(); err != nil {
		return fmt.Errorf("error executing sync command: %w", err)
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
				return rclsync.CopyDir(context.Background(), fdst, fsrc, createEmptySrcDirs)
			}
			return operations.CopyFile(context.Background(), fdst, fsrc, srcFileName, srcFileName)
		}, true)
	},
}

func getRcloneDirPath() string {
	fullConfigPath := config.GetConfigPath()
	return filepath.Dir(fullConfigPath)
}

func SyncRemoteToLocal(remote config.Remote, localPath string) error {
	dirName := filepath.Base(localPath)
	src := fmt.Sprintf("%s:%s", remote.Name, dirName)
	args := []string{src, localPath}
	fmt.Printf("SyncRemoteToLocal: syncing from %s to %s\n", src, localPath)
	return remoteCallSync(args)
}

func SyncLocalToRemote(remote config.Remote, localPath string) error {
	dirName := filepath.Base(localPath)
	dest := fmt.Sprintf("%s:%s", remote.Name, dirName)
	args := []string{localPath, dest} // local → remote
	fmt.Printf("SyncLocalToRemote: syncing from %s to %s\n", localPath, dest)
	return remoteCallSync(args)
}

func SyncAllLocalToRemote(localPath string) error {
	remotes := config.GetRemotes()
	if len(remotes) == 0 {
		return fmt.Errorf("동기화할 remote가 없음")
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	for _, remote := range remotes {
		wg.Add(1)
		go func(r config.Remote) {
			defer wg.Done()
			err := SyncLocalToRemote(r, localPath)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("remote %s: %w", r.Name, err))
				mu.Unlock()
			}
		}(remote)
	}
	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("error during local->remote sync", errs)
	}
	fmt.Println("successflly local->remote sync")
	return nil
}

func SyncAnyRemoteToLocal(localPath string) error {
	remotes := config.GetRemotes()
	if len(remotes) == 0 {
		return fmt.Errorf("동기화할 remote가 없음")
	}

	var lastErr error
	for _, remote := range remotes {
		fmt.Printf("Trying remote '%s' for sync...\n", remote.Name)

		err := SyncRemoteToLocal(remote, localPath)
		if err != nil {
			fmt.Printf("remote '%s' sync 실패: %v\n", remote.Name, err)
			lastErr = err
			continue
		} else {
			fmt.Printf("remote '%s' sync 성공!\n", remote.Name)
			return nil
		}
	}

	return fmt.Errorf("all remote failed! last err : %v", lastErr)
}

func Config_upload() error {
	path := getRcloneDirPath()
	remotes := config.GetRemotes()
	dir := filepath.Base(path)
	fmt.Printf("dir: %s\n", dir)

	var wg sync.WaitGroup
	var errs []error

	for _, remote := range remotes {

		wg.Add(1)

		go func(remote config.Remote) {
			defer wg.Done()
			dest := fmt.Sprintf("%s:%s", remote.Name, dir)

			err := remoteCallCopy([]string{path, dest})
			if err != nil {
				errs = append(errs, fmt.Errorf("error in remoteCallCopy for file %s: %w", path, err))
				return
			}
		}(remote)

	}

	wg.Wait()
	if len(errs) > 0 {
		return fmt.Errorf("errors occurred: %v", errs)
	}
	fmt.Println("config file uploaded!!")
	return nil
}
