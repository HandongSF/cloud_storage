package main

import (
	"bufio"
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

var loadingIndicator = widget.NewProgressBarInfinite()

func refreshRemoteFileList(fileListContainer *fyne.Container, logOutput *widget.RichText, progress *widget.Label, w fyne.Window) {
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

		fileLabel := widget.NewLabel(fileName)
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
							refreshRemoteFileList(fileListContainer, logOutput, progress, w)
						}
						loadingIndicator.Hide()
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
	progressLabel := widget.NewLabel("")
	progressLabel.Hide()
	progressBar := widget.NewProgressBar()
	progressBar.Hide()

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

	loadBalancerOptions := []string{"RoundRobin", "ResourceBased", "DownloadOptima", "UploadOptima"}
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
		progressLabel.Show()
		progressBar.Show()
		progressBar.SetValue(0)

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

			cmd := exec.Command("../rclone", "dis_upload", source, "--loadbalancer", loadBalancer)

			// 파이프라인 설정
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				logOutput.ParseMarkdown(fmt.Sprintf("❌ **Error setting up pipe:**\n```\n%s\n```", err.Error()))
				return
			}

			// 명령어 시작
			if err := cmd.Start(); err != nil {
				logOutput.ParseMarkdown(fmt.Sprintf("❌ **Error starting command:**\n```\n%s\n```", err.Error()))
				return
			}

			// 출력 처리
			scanner := bufio.NewScanner(stdout)
			var totalShards int
			var currentShard int
			var shardCountFound bool

			for scanner.Scan() {
				line := scanner.Text()
				logOutput.ParseMarkdown(line + "\n")

				// 총 샤드 개수 파싱
				if !shardCountFound && strings.Contains(line, "File split into") {
					parts := strings.Split(line, "data +")
					if len(parts) > 1 {
						parityStr := strings.Split(parts[1], "parity")[0]
						parityStr = strings.TrimSpace(parityStr)
						if parity, err := strconv.Atoi(parityStr); err == nil {
							// 데이터 샤드(5) + 패리티 샤드(3) = 총 8개
							totalShards = 5 + parity
							shardCountFound = true
							progressLabel.SetText(fmt.Sprintf("Total shards to upload: %d", totalShards))
							logOutput.ParseMarkdown(fmt.Sprintf("**Total shards to upload: %d**\n", totalShards))
							progressBar.SetValue(0)
						}
					}
				}

				// 샤드 업로드 완료 확인
				if strings.Contains(line, "Time taken for copy cmd:") {
					currentShard++
					if totalShards > 0 {
						progressValue := float64(currentShard) / float64(totalShards)
						progressBar.SetValue(progressValue)
						progressLabel.SetText(fmt.Sprintf("Progress: %d/%d shards (%.1f%%)",
							currentShard, totalShards, progressValue*100))
						logOutput.ParseMarkdown(fmt.Sprintf("**Progress: %d/%d shards (%.1f%%)**\n",
							currentShard, totalShards, progressValue*100))
					}
				}
			}

			// 명령어 완료 대기
			if err := cmd.Wait(); err != nil {
				logOutput.ParseMarkdown(fmt.Sprintf("❌ **Upload Error:**\n```\n%s\n```", err.Error()))
			} else {
				progressBar.SetValue(1)
				progressLabel.SetText("Success! All shards uploaded.")
				logOutput.ParseMarkdown("🟢 **Success! All shards uploaded.**")
				refreshRemoteFileList(fileListContainer, logOutput, progressLabel, w)
			}
			progressLabel.Hide()
			progressBar.Hide()
		} else if mode == "Dis_Download" {
			target := targetEntry.Text
			dest := destinationEntry.Text

			if target == "" || dest == "" {
				logOutput.ParseMarkdown("*❌ Error:* Enter target file and destination")
				return
			}

			progressLabel.Show()
			progressBar.Show()
			progressBar.SetValue(0)

			cmd := exec.Command("rclone", "dis_download", target, dest)

			// 파이프라인 설정
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				logOutput.ParseMarkdown(fmt.Sprintf("❌ **Error setting up pipe:**\n```\n%s\n```", err.Error()))
				return
			}

			// 명령어 시작
			if err := cmd.Start(); err != nil {
				logOutput.ParseMarkdown(fmt.Sprintf("❌ **Error starting command:**\n```\n%s\n```", err.Error()))
				return
			}

			// 출력 처리
			scanner := bufio.NewScanner(stdout)
			var totalShards int
			var currentShard int
			var shardCountFound bool

			for scanner.Scan() {
				line := scanner.Text()
				logOutput.ParseMarkdown(line + "\n")

				// 총 샤드 개수 파싱 (8개로 고정)
				if !shardCountFound && strings.Contains(line, "Downloading shard") {
					totalShards = 8
					shardCountFound = true
					progressLabel.SetText(fmt.Sprintf("Total shards to download: %d", totalShards))
				}

				// 샤드 다운로드 완료 확인
				if strings.Contains(line, "Time taken for copy cmd:") {
					currentShard++
					if totalShards > 0 {
						progressValue := float64(currentShard) / float64(totalShards)
						progressBar.SetValue(progressValue)
						progressLabel.SetText(fmt.Sprintf("Progress: %d/%d shards (%.1f%%)",
							currentShard, totalShards, progressValue*100))
					}
				}
			}

			// 명령어 완료 대기
			if err := cmd.Wait(); err != nil {
				logOutput.ParseMarkdown(fmt.Sprintf("❌ **Download Error:**\n```\n%s\n```", err.Error()))
			} else {
				progressBar.SetValue(1)
				progressLabel.SetText("Success! All shards downloaded.")
				logOutput.ParseMarkdown("🟢 **Success! File downloaded successfully.**")
			}
			progressLabel.Hide()
			progressBar.Hide()
		}
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
		progressLabel,
		progressBar,
		loadingIndicator,
		startButton,
		scrollableLog,
	)

	w.SetContent(content)
	refreshRemoteFileList(fileListContainer, logOutput, progressLabel, w)
	w.ShowAndRun()
}
