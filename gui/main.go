package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/rclone/rclone/fs/dis_operations"
)

// 1. When making change to main.go file, build with go build -o MyApp.exe
// 2. and drag the build icon (.exe file) to outside the gui file to
// C:\Users\samue\Desktop\cloud_storage> go build -o rclone.exe
// 3. Finally create a 바로가기 icon if you want it to be ran outside the rclone directory

var loadingIndicator = widget.NewProgressBarInfinite()

func checkCoreFile() int {
	rcloneDir := dis_operations.GetRcloneDirPath()
	dataDir := filepath.Join(rcloneDir, "data")

	datamapBase := filepath.Join(dataDir, dis_operations.GetDatamapFileName())
	lbBase := filepath.Join(dataDir, dis_operations.GetLBFileName())

	datamapFcef := datamapBase + ".fcef"
	lbFcef := lbBase + ".fcef"

	// Check original files
	baseExists := fileExists(datamapBase) && fileExists(lbBase)
	// Check .fcef files
	fcefExists := fileExists(datamapFcef) && fileExists(lbFcef)

	if baseExists || fcefExists {
		return 1
	}
	return -1
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func refreshRemoteFileList(fileListContainer *fyne.Container, logOutput *widget.RichText, progress *widget.ProgressBar, w fyne.Window, modeSelect *widget.Select, targetEntry *widget.Entry) {
	rootPath := dis_operations.GetRcloneDirPath()
	dataPath := filepath.Join(rootPath, "data")

	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		return
	}

	//Check if both core files exist
	if checkCoreFile() == -1 {
		fmt.Println("NO found")
		return
	}
	fileListContainer.Objects = nil // 기존 항목 비우기

	cmd := exec.Command("./rclone", "dis_ls")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fileListContainer.Add(widget.NewLabel(fmt.Sprintf("❌ Failed to load list:\n%s", string(output))))
		fileListContainer.Refresh()
		return
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fileName := line

		// Always use a button for consistency
		fileButton := widget.NewButton(fileName, func(fn string) func() {
			return func() {
				if modeSelect.Selected == "Dis_Download" {
					targetEntry.SetText(fn)
				}
			}
		}(fileName)) // closure to capture fileName properly

		deleteButton := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
			dialog.ShowConfirm("Delete File", fmt.Sprintf("Delete '%s'?", fileName), func(confirm bool) {
				if confirm {
					progress.Show()
					go func() {
						defer progress.Hide()
						loadingIndicator.Show()

						cmd := exec.Command("rclone", "dis_rm", fileName)
						rmOut, rmErr := cmd.CombinedOutput()
						if rmErr != nil {
							logOutput.ParseMarkdown(fmt.Sprintf("❌ **Delete Error:**\n```\n%s\n```", string(rmOut)))
						} else {
							logOutput.ParseMarkdown("🟢 **Deleted!**")
							refreshRemoteFileList(fileListContainer, logOutput, progress, w, modeSelect, targetEntry)
						}
						loadingIndicator.Hide()
					}()
				}
			}, w)
		})

		row := container.NewBorder(nil, nil, nil, deleteButton, fileButton)
		fileListContainer.Add(row)
	}

	fileListContainer.Refresh()
}

// Function to prompt user for new password
func showPasswordSetupWindow(w fyne.Window) {
	fmt.Println("showPasswordSetupWindow")
	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("Enter new password")

	submitButton := widget.NewButton("Set Password", func() {
		password := passwordEntry.Text
		if password == "" {
			dialog.ShowError(fmt.Errorf("Password cannot be empty"), w)
			return
		}

		// Save the password
		dis_operations.SaveUserPassword(password)
		showMainGUIContent(w) // Just change window content
	})

	passwordForm := container.NewVBox(
		widget.NewLabel("Set a new password"),
		passwordEntry,
		submitButton,
	)

	w.SetContent(passwordForm)
}

// Function to prompt user for existing password
func showPasswordPromptWindow(w fyne.Window) {
	fmt.Println("showPasswordPromptWindow")
	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("Enter your password")

	submitButton := widget.NewButton("Unlock", func() {
		password := passwordEntry.Text
		if password == "" {
			dialog.ShowError(fmt.Errorf("Password cannot be empty"), w)
			return
		}

		// Try decrypting files with given password
		err := dis_operations.DecryptAllFilesInPath(password)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Invalid password or decryption failed"), w)
			return
		}

		showMainGUIContent(w) // Just change window content
	})

	passwordForm := container.NewVBox(
		widget.NewLabel("Enter your password"),
		passwordEntry,
		submitButton,
	)

	w.SetContent(passwordForm)
}

// Function to encrypt all files before closing the app
func encryptFilesOnExit() {
	userPassword := dis_operations.GetUserPassword()
	if userPassword == "" {
		fmt.Println("Error: No user password found.")
		return
	}

	err := dis_operations.EncryptAllFilesInPath(userPassword)
	if err != nil {
		fmt.Println("Error encrypting files:", err)
	} else {
		fmt.Println("All files encrypted successfully.")
	}
}

func cleanShardFolderOnExit() error {
	shardPath := filepath.Join(dis_operations.GetRcloneDirPath(), "shard")

	entries, err := os.ReadDir(shardPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Folder doesn't exist, nothing to clean
			return nil
		}
		return fmt.Errorf("failed to read shard folder: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			fullPath := filepath.Join(shardPath, entry.Name())
			if err := os.Remove(fullPath); err != nil {
				return err
			}
		}
	}

	return nil
}

func showMainGUIContent(w fyne.Window) {
	fmt.Println("showMainGUI")
	w.Resize(fyne.NewSize(600, 600))
	w.SetTitle("Dis_Upload / Dis_Download GUI")
	w.SetCloseIntercept(func() {
		encryptFilesOnExit()
		cleanShardFolderOnExit()
		w.Close() // manually trigger close
	})

	fileListContainer := container.NewVBox()
	scrollableFileList := container.NewVScroll(fileListContainer)
	scrollableFileList.SetMinSize(fyne.NewSize(580, 150))

	logOutput := widget.NewRichTextWithText("")
	logOutput.Wrapping = fyne.TextWrapWord
	scrollableLog := container.NewVScroll(logOutput)
	scrollableLog.SetMinSize(fyne.NewSize(580, 150))

	progressBar := widget.NewProgressBar()
	progressBar.Hide()

	modeSelect, sourceEntry, fileSelectButton, loadBalancerSelect, targetEntry, destinationEntry, destinationSelectButton := createInputFields(w)

	startButton := widget.NewButton("Run", func() {
		handleRunButton(
			modeSelect, sourceEntry, loadBalancerSelect, targetEntry, destinationEntry,
			logOutput, progressBar, fileListContainer, w,
		)
	})

	modeSelect.OnChanged = func(mode string) {
		toggleModeUI(mode, sourceEntry, fileSelectButton, loadBalancerSelect, targetEntry, destinationEntry, destinationSelectButton)
	}
	modeSelect.OnChanged(modeSelect.Selected)

	content := container.NewVBox(
		scrollableFileList,
		modeSelect,
		sourceEntry,
		fileSelectButton,
		loadBalancerSelect,
		targetEntry,
		destinationEntry,
		destinationSelectButton,
		progressBar,
		startButton,
		scrollableLog,
	)

	w.SetContent(content)
	refreshRemoteFileList(fileListContainer, logOutput, progressBar, w, modeSelect, targetEntry)
}

func createInputFields(w fyne.Window) (
	*widget.Select, *widget.Entry, *widget.Button, *widget.Select,
	*widget.Entry, *widget.Entry, *widget.Button,
) {
	modeSelect := widget.NewSelect([]string{"Dis_Upload", "Dis_Download"}, nil)
	modeSelect.SetSelected("Dis_Upload")

	sourceEntry := widget.NewEntry()
	sourceEntry.SetPlaceHolder("Enter source file path")

	fileSelectButton := widget.NewButton("Choose File", func() {
		fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if reader != nil {
				sourceEntry.SetText(reader.URI().Path())
				defer reader.Close()
			}
		}, w)
		fileDialog.SetFilter(storage.NewExtensionFileFilter([]string{".txt", ".jpg", ".png", ".pdf"}))
		fileDialog.Show()
	})

	loadBalancerSelect := widget.NewSelect(
		[]string{"RoundRobin", "ResourceBased", "DownloadOptima", "UploadOptima"}, nil,
	)

	targetEntry := widget.NewEntry()
	targetEntry.SetPlaceHolder("Enter target file name (ex: test.jpg)")

	destinationEntry := widget.NewEntry()
	destinationEntry.SetPlaceHolder("Enter destination path")
	destinationSelectButton := widget.NewButton("Choose Destination", func() {
		dialog.NewFolderOpen(func(list fyne.ListableURI, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if list != nil {
				destinationEntry.SetText(list.Path())
			}
		}, w).Show()
	})
	return modeSelect, sourceEntry, fileSelectButton, loadBalancerSelect, targetEntry, destinationEntry, destinationSelectButton
}

func toggleModeUI(mode string, sourceEntry *widget.Entry, fileSelect *widget.Button,
	loadBalancer *widget.Select, targetEntry, destEntry *widget.Entry, destBtn *widget.Button) {

	if mode == "Dis_Upload" {
		sourceEntry.Show()
		fileSelect.Show()
		loadBalancer.Show()
		targetEntry.Hide()
		destBtn.Hide()
		destEntry.Hide()
	} else {
		sourceEntry.Hide()
		fileSelect.Hide()
		loadBalancer.Hide()
		targetEntry.Show()
		destBtn.Show()
		destEntry.Show()
	}
}

func handleRunButton(
	modeSelect *widget.Select,
	sourceEntry *widget.Entry,
	loadBalancerSelect *widget.Select,
	targetEntry *widget.Entry,
	destinationEntry *widget.Entry,
	logOutput *widget.RichText,
	progressBar *widget.ProgressBar,
	fileListContainer *fyne.Container,
	w fyne.Window,
) {
	mode := modeSelect.Selected
	logOutput.ParseMarkdown("")
	progressBar.Show()
	progressBar.SetValue(0)

	if mode == "Dis_Upload" {
		startUpload(sourceEntry.Text, loadBalancerSelect.Selected, progressBar, logOutput, fileListContainer, w, modeSelect, targetEntry)
	} else {
		startDownload(targetEntry.Text, destinationEntry.Text, progressBar, logOutput, fileListContainer, w, modeSelect, targetEntry)
	}
}

func startUpload(source, loadBalancer string,
	progressBar *widget.ProgressBar, logOutput *widget.RichText,
	fileListContainer *fyne.Container, w fyne.Window,
	modeSelect *widget.Select, targetEntry *widget.Entry,
) {
	if source == "" || loadBalancer == "" {
		logOutput.ParseMarkdown("*❌ Error:* Enter file path and load balancer")
		return
	}
	if _, err := os.Stat(source); err != nil {
		logOutput.ParseMarkdown(fmt.Sprintf("❌ **Error reading file:**\n```\n%s\n```", err.Error()))
		return
	}

	cmd := exec.Command("rclone", "dis_upload", source, "--loadbalancer", loadBalancer)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		logOutput.ParseMarkdown(fmt.Sprintf("❌ **Pipe error:**\n```\n%s\n```", err.Error()))
		return
	}
	if err := cmd.Start(); err != nil {
		logOutput.ParseMarkdown(fmt.Sprintf("❌ **Start error:**\n```\n%s\n```", err.Error()))
		return
	}

	go monitorProgress(cmd, stdoutPipe, progressBar, logOutput, "upload", fileListContainer, w, modeSelect, targetEntry)
}

func startDownload(target, destination string,
	progressBar *widget.ProgressBar, logOutput *widget.RichText,
	fileListContainer *fyne.Container, w fyne.Window,
	modeSelect *widget.Select, targetEntry *widget.Entry,
) {
	if target == "" || destination == "" {
		logOutput.ParseMarkdown("*❌ Error:* Choose target file and destination")
		return
	}

	cmd := exec.Command("rclone", "dis_download", target, destination)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		logOutput.ParseMarkdown(fmt.Sprintf("❌ **Pipe error:**\n```\n%s\n```", err.Error()))
		return
	}
	if err := cmd.Start(); err != nil {
		logOutput.ParseMarkdown(fmt.Sprintf("❌ **Start error:**\n```\n%s\n```", err.Error()))
		return
	}

	go monitorProgress(cmd, stdoutPipe, progressBar, logOutput, "download", fileListContainer, w, modeSelect, targetEntry)
}

func monitorProgress(
	cmd *exec.Cmd, stdoutPipe io.ReadCloser,
	progressBar *widget.ProgressBar, logOutput *widget.RichText,
	mode string, fileListContainer *fyne.Container,
	w fyne.Window, modeSelect *widget.Select, targetEntry *widget.Entry,
) {
	scanner := bufio.NewScanner(stdoutPipe)
	var totalShards, currentShard int

	for scanner.Scan() {
		line := scanner.Text()

		if mode == "upload" && strings.Contains(line, "File split into") {
			parts := strings.Split(line, "data +")
			if len(parts) > 1 {
				dataCount, _ := strconv.Atoi(strings.TrimSpace(strings.Split(parts[0], "into ")[1]))
				parityCount, _ := strconv.Atoi(strings.TrimSpace(strings.Split(parts[1], "parity")[0]))
				totalShards = dataCount + parityCount
			}
		}

		if mode == "download" && strings.Contains(line, "Expecting to download") {
			parts := strings.Fields(line)
			for i, word := range parts {
				if word == "download" && i > 0 {
					totalShards, _ = strconv.Atoi(parts[i-1])
					break
				}
			}
		}

		if (mode == "upload" && strings.HasPrefix(line, "Time taken for copy cmd:")) ||
			(mode == "download" && strings.HasPrefix(line, "Downloaded shard")) {
			currentShard++
			if totalShards > 0 {
				progressBar.SetValue(float64(currentShard) / float64(totalShards))
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		logOutput.ParseMarkdown(fmt.Sprintf("❌ **%s failed!**", strings.Title(mode)))
	} else {
		progressBar.SetValue(1)
		logOutput.ParseMarkdown(fmt.Sprintf("🟢 **Success! All shards %sed.**", mode))
		refreshRemoteFileList(fileListContainer, logOutput, progressBar, w, modeSelect, targetEntry)
	}
}

func main() {
	a := app.NewWithID("com.example.myapp")
	w := a.NewWindow("Password Setup")
	w.Resize(fyne.NewSize(300, 100))

	if dis_operations.DoesUserPasswordExist() {
		// Password exists -> Ask user for it
		showPasswordPromptWindow(w)
	} else {
		// No password exists -> Ask user to create one
		showPasswordSetupWindow(w)
	}

	w.ShowAndRun() // Only call this once
}
