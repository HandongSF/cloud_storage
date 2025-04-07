package main

import (
	"fmt"
	"os"
	"os/exec"
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

var loadingIndicator = widget.NewProgressBarInfinite()

func refreshRemoteFileList(fileListContainer *fyne.Container, logOutput *widget.RichText, progress *widget.ProgressBar, w fyne.Window, modeSelect *widget.Select, targetEntry *widget.Entry) {
	fileListContainer.Objects = nil // 기존 항목 비우기

	cmd := exec.Command("rclone", "dis_ls")
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

func showMainGUIContent(w fyne.Window) {
	fmt.Println("showMainGUI")

	w.Resize(fyne.NewSize(600, 600))
	w.SetTitle("Dis_Upload / Dis_Download GUI")

	w.SetOnClosed(func() {
		encryptFilesOnExit()
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

	loadBalancerOptions := []string{"RoundRobin", "ResourceBased", "DownloadOptima", "UploadOptima"}
	loadBalancerSelect := widget.NewSelect(loadBalancerOptions, nil)

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

	startButton := widget.NewButton("Run", func() {
		mode := modeSelect.Selected
		logOutput.ParseMarkdown("")
		progressBar.Show()
		progressBar.SetValue(0)

		if mode == "Dis_Upload" {
			source := sourceEntry.Text
			loadBalancer := loadBalancerSelect.Selected

			if source == "" || loadBalancer == "" {
				logOutput.ParseMarkdown("*❌ Error:* Enter file path and load balancer")
				return
			}

			_, err := os.Stat(source)
			if err != nil {
				logOutput.ParseMarkdown(fmt.Sprintf("❌ **Error reading file:**\n```\n%s\n```", err.Error()))
				return
			}

			cmd := exec.Command("../rclone", "dis_upload", source, "--loadbalancer", loadBalancer)
			err = cmd.Run()
			if err != nil {
				logOutput.ParseMarkdown("❌ **Upload failed!**")
			} else {
				logOutput.ParseMarkdown("🟢 **Success! All shards uploaded.**")
				refreshRemoteFileList(fileListContainer, logOutput, progressBar, w, modeSelect, targetEntry)
			}
		} else if mode == "Dis_Download" {
			target := targetEntry.Text
			dest := destinationEntry.Text

			if target == "" || dest == "" {
				logOutput.ParseMarkdown("*❌ Error:* Enter target file and destination")
				return
			}

			cmd := exec.Command("rclone", "dis_download", target, dest)
			err := cmd.Run()
			if err != nil {
				logOutput.ParseMarkdown("❌ **Download failed!**")
			} else {
				logOutput.ParseMarkdown("🟢 **Success! File downloaded successfully.**")
			}
		}
	})

	modeSelect.OnChanged = func(mode string) {
		if mode == "Dis_Upload" {
			sourceEntry.Show()
			fileSelectButton.Show()
			loadBalancerSelect.Show()
			targetEntry.Hide()
			destinationSelectButton.Hide()
			destinationEntry.Hide()
		} else {
			sourceEntry.Hide()
			fileSelectButton.Hide()
			loadBalancerSelect.Hide()
			targetEntry.Show()
			destinationSelectButton.Show()
			destinationEntry.Show()
		}
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

func main() {
	a := app.New()
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
