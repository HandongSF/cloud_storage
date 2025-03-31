package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func refreshRemoteFileList(fileListContainer *fyne.Container, logOutput *widget.RichText, progress *widget.ProgressBar, w fyne.Window) {
	fileListContainer.Objects = nil // 기존 항목 비우기

	cmd := exec.Command("../rclone", "dis_ls")
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

		fileLabel := widget.NewLabel(fileName)
		deleteButton := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
			dialog.ShowConfirm("Delete File", fmt.Sprintf("Delete '%s'?", fileName), func(confirm bool) {
				if confirm {
					progress.Show()
					go func() {
						defer progress.Hide()

						cmd := exec.Command("../rclone", "dis_rm", fileName)
						rmOut, rmErr := cmd.CombinedOutput()
						if rmErr != nil {
							logOutput.ParseMarkdown(fmt.Sprintf("❌ **Delete Error:**\n```\n%s\n```", string(rmOut)))
						} else {
							logOutput.ParseMarkdown("🟢 **Deleted!**")
							refreshRemoteFileList(fileListContainer, logOutput, progress, w)
						}
					}()
				}
			}, w)
		})

		row := container.NewBorder(nil, nil, nil, deleteButton, fileLabel)
		fileListContainer.Add(row)
	}

	fileListContainer.Refresh()
}

func main() {
	a := app.New()
	w := a.NewWindow("Dis_Upload / Dis_Download GUI")
	w.Resize(fyne.NewSize(600, 600))

	// Remote 파일 목록 영역
	fileListContainer := container.NewVBox()
	scrollableFileList := container.NewVScroll(fileListContainer)
	scrollableFileList.SetMinSize(fyne.NewSize(580, 150))

	// 로그 영역
	logOutput := widget.NewRichTextWithText("")
	logOutput.Wrapping = fyne.TextWrapWord
	scrollableLog := container.NewVScroll(logOutput)
	scrollableLog.SetMinSize(fyne.NewSize(580, 150))

	// 로딩 인디케이터
	progress := widget.NewProgressBar()
	progress.Hide()

	// 모드 선택
	modeSelect := widget.NewSelect([]string{"Dis_Upload", "Dis_Download"}, nil)
	modeSelect.SetSelected("Dis_Upload")

	// 업로드용
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

	loadBalancerOptions := []string{"RoundRobin", "LeastConnections", "Random"}
	loadBalancerSelect := widget.NewSelect(loadBalancerOptions, nil)

	// 다운로드용
	targetEntry := widget.NewEntry()
	targetEntry.SetPlaceHolder("Enter target file name (ex: test.jpg)")
	destinationEntry := widget.NewEntry()
	destinationEntry.SetPlaceHolder("Enter destination path")

	// 실행 버튼
	startButton := widget.NewButton("Run", func() {
		mode := modeSelect.Selected
		logOutput.ParseMarkdown("")
		progress.Show()

		go func() {
			defer progress.Hide()

			if mode == "Dis_Upload" {
				source := sourceEntry.Text
				loadBalancer := loadBalancerSelect.Selected

				if source == "" || loadBalancer == "" {
					logOutput.ParseMarkdown("*❌ Error:* Enter file path and load balancer")
					return
				}

				// 파일 존재 여부 확인
				_, err := os.Stat(source)
				if err != nil {
					logOutput.ParseMarkdown(fmt.Sprintf("❌ **Error reading file:**\n```\n%s\n```", err.Error()))
					return
				}

				progress.SetValue(0)
				progress.Show()

				cmd := exec.Command("../rclone", "dis_upload", source, "--loadbalancer", loadBalancer)
				output, err := cmd.CombinedOutput()

				// 출력에서 진행률 파싱
				outputStr := string(output)
				if strings.Contains(outputStr, "Progress:") {
					lines := strings.Split(outputStr, "\n")
					for _, line := range lines {
						if strings.Contains(line, "Progress:") {
							// Progress: X% 형식에서 숫자만 추출
							progressStr := strings.Split(line, "Progress:")[1]
							progressStr = strings.TrimSpace(progressStr)
							progressStr = strings.TrimSuffix(progressStr, "%")
							if progressValue, err := strconv.ParseFloat(progressStr, 64); err == nil {
								progress.SetValue(progressValue / 100)
							}
						}
					}
				}

				if err != nil {
					logOutput.ParseMarkdown(fmt.Sprintf("❌ **Upload Error:**\n```\n%s\n```", string(output)))
				} else {
					progress.SetValue(1)
					logOutput.ParseMarkdown("🟢 **Success!**")
					refreshRemoteFileList(fileListContainer, logOutput, progress, w)
				}
				progress.Hide()
			} else if mode == "Dis_Download" {
				target := targetEntry.Text
				dest := destinationEntry.Text

				if target == "" || dest == "" {
					logOutput.ParseMarkdown("*❌ Error:* Enter target file and destination")
					return
				}

				cmd := exec.Command("../rclone", "dis_download", target, dest)
				output, err := cmd.CombinedOutput()
				if err != nil {
					logOutput.ParseMarkdown(fmt.Sprintf("❌ **Download Error:**\n```\n%s\n```", string(output)))
				} else {
					logOutput.ParseMarkdown("🟢 **Success!**")
				}
			}
		}()
	})

	// 모드에 따라 UI 전환
	modeSelect.OnChanged = func(mode string) {
		if mode == "Dis_Upload" {
			sourceEntry.Show()
			fileSelectButton.Show()
			loadBalancerSelect.Show()
			targetEntry.Hide()
			destinationEntry.Hide()
		} else {
			sourceEntry.Hide()
			fileSelectButton.Hide()
			loadBalancerSelect.Hide()
			targetEntry.Show()
			destinationEntry.Show()
		}
	}
	modeSelect.OnChanged(modeSelect.Selected)

	// UI 구성
	content := container.NewVBox(
		scrollableFileList,
		modeSelect,
		sourceEntry,
		fileSelectButton,
		loadBalancerSelect,
		targetEntry,
		destinationEntry,
		progress,
		startButton,
		scrollableLog,
	)

	w.SetContent(content)
	refreshRemoteFileList(fileListContainer, logOutput, progress, w)
	w.ShowAndRun()
}
