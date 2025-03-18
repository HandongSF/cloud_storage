package dis_config

import (
	"context"
	"fmt"
	"io"
	"os"
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

func Config_upload(args []string) error {
	// path := getRcloneDirPath()
	// remotes := config.GetRemotes()
	// dir := filepath.Base(path)
	// fmt.Printf("dir: %s\n", dir)

	// var wg sync.WaitGroup
	// var errs []error

	// for _, remote := range remotes {

	// 	wg.Add(1)

	// 	go func(remote config.Remote) {
	// 		defer wg.Done()
	// 		dest := fmt.Sprintf("%s:%s", remote.Name, dir)

	// 		err := remoteCallCopy([]string{path, dest})
	// 		if err != nil {
	// 			errs = append(errs, fmt.Errorf("error in remoteCallCopy for file %s: %w", path, err))
	// 			return
	// 		}
	// 	}(remote)

	// }

	// wg.Wait()
	// if len(errs) > 0 {
	// 	return fmt.Errorf("errors occurred: %v", errs)
	// }
	// fmt.Println("config file uploaded!!")
	// return nil

	/*
		처음엔 path(rclone자체가 저장되어있는)를 받음
		그리고 원래
		path := getRcloneDirPath() 에 들어가서 config파일에 내용이 있는지 확인
		if config 파일이 존재림
			-> 어딘가 복사해놓을지 물림
				if 복사하고 싶다면, path받음
					저장
		-> 그냥 savedpath에 저장되어있는 rclone파일을 path에 올림
	*/

	savedPath := args[0]
	stat, err := os.Stat(savedPath)
	if err != nil {
		return fmt.Errorf("savedPath의 정보를 가져올 수 없습니다: %w", err)
	}
	if !stat.IsDir() {
		return fmt.Errorf("savedPath는 디렉토리가 아닙니다")
	}
	if filepath.Base(savedPath) != "rclone" {
		return fmt.Errorf("잘못된 디렉토리입니다. 'rclone' 디렉토리를 업로드해야 합니다")
	}

	path := getRcloneDirPath()
	fmt.Printf("rclone path: %s\n", path)

	configFilePath := filepath.Join(path, "rclone.conf")
	_, err = os.Stat(configFilePath)
	if err == nil {
		if DoBackup() {
			var backupDest string
			fmt.Printf("Enter a path: ")
			fmt.Scanf("%s", &backupDest)
			stat, err := os.Stat(backupDest)
			if err != nil || !stat.IsDir() {
				return fmt.Errorf("입력한 백업 대상 경로가 존재하지 않거나 디렉토리가 아닙니다: %s", backupDest)

			}

			if err := copyDir(path, backupDest); err != nil {
				return fmt.Errorf("백업 실패: %w", err)
			}

			fmt.Println("completed successfully backup")
		}
	}

	if err := copyDir(savedPath, path); err != nil {
		return fmt.Errorf("saved rclone 디렉토리 업로드 실패: %w", err)
	}
	fmt.Println("saved rclone 디렉토리 내용이 성공적으로 업로드되었습니다!")

	return nil
}

// copyDirContents는 srcDir 안의 모든 파일과 서브디렉토리를 destDir로 복사합니다.
func copyDirContents(srcDir, destDir string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("디렉토리 읽기 실패: %w", err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(srcDir, entry.Name())
		destPath := filepath.Join(destDir, entry.Name())

		info, err := entry.Info()
		if err != nil {
			return err
		}

		if info.IsDir() {
			// 하위 디렉토리가 있으면 destPath에 디렉토리를 생성한 후, 재귀적으로 복사
			if err := os.MkdirAll(destPath, info.Mode()); err != nil {
				return err
			}
			if err := copyDir(srcPath, destPath); err != nil {
				return err
			}
		} else {
			// 파일인 경우 복사
			if err := copyFile(srcPath, destPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyDir(src string, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !srcInfo.IsDir() {
		return fmt.Errorf("source is not dir")
	}

	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(src, path)

		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return copyFile(path, dstPath)
	})

}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, srcInfo.Mode())
}

func GetUserConfirmation(prompt string, options []string, defaultIndex int) bool {
	switch i := config.CommandDefault(options, defaultIndex); i {
	case 'y':
		return true
	case 'n':
		return false
	default:
		fmt.Printf("Invalid Input!\n")
		fmt.Printf("%s\n", prompt)
		return GetUserConfirmation(prompt, options, defaultIndex)
	}
}

func DoBackup() bool {
	return GetUserConfirmation("Do you want to backup? ", []string{"yYes backup the rclone dir", "nNo ignore"}, 0)
}
