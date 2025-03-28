package main

import (
	"fmt"
	"os/exec"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

func refreshRemoteFileList(fileListOutput *widget.RichText) {
	cmd := exec.Command("../rclone", "dis_ls")
	output, err := cmd.CombinedOutput()

	if err != nil {
		fileListOutput.ParseMarkdown(fmt.Sprintf("❌ **Failed to load remote file list:**\n```\n%s\n```", string(output)))
	} else {
		fileListOutput.ParseMarkdown(fmt.Sprintf("📂 **Remote Files:**\n```\n%s\n```", string(output)))
	}
}

func main() {
	a := app.New()
	w := a.NewWindow("Dis_Upload / Dis_Download GUI")
	w.Resize(fyne.NewSize(600, 500))

	// Remote 파일 목록 영역
	fileListOutput := widget.NewRichTextWithText("📂 Loading remote file list...")
	fileListOutput.Wrapping = fyne.TextWrapWord
	refreshRemoteFileList(fileListOutput)

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

	// 로그 영역
	logOutput := widget.NewRichTextWithText("")
	logOutput.Wrapping = fyne.TextWrapWord

	// 실행 버튼
	startButton := widget.NewButton("Run", func() {
		mode := modeSelect.Selected
		if mode == "Dis_Upload" {
			source := sourceEntry.Text
			loadBalancer := loadBalancerSelect.Selected

			if source == "" || loadBalancer == "" {
				logOutput.ParseMarkdown("*❌ Error:* Enter file path and load balancer")
				return
			}
			cmd := exec.Command("../rclone", "dis_upload", source, "--loadbalancer", loadBalancer)
			output, err := cmd.CombinedOutput()
			if err != nil {
				logOutput.ParseMarkdown(fmt.Sprintf("❌ **Upload Error:** %v\n```\n%s\n```", err, string(output)))
			} else {
				logOutput.ParseMarkdown(fmt.Sprintf("🟢 **Upload Success:**\n```\n%s\n```", string(output)))
				refreshRemoteFileList(fileListOutput)
			}
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
				logOutput.ParseMarkdown(fmt.Sprintf("❌ **Download Error:** %v\n```\n%s\n```", err, string(output)))
			} else {
				logOutput.ParseMarkdown(fmt.Sprintf("🟢 **Download Success:**\n```\n%s\n```", string(output)))
			}
		}
	})

	// 모드에 따라 UI 바꾸기
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
	// 초기 상태
	modeSelect.OnChanged(modeSelect.Selected)

	// UI 배치
	content := container.NewVBox(
		fileListOutput,
		modeSelect,
		sourceEntry,
		fileSelectButton,
		loadBalancerSelect,
		targetEntry,
		destinationEntry,
		startButton,
		logOutput,
	)

	w.SetContent(content)
	w.ShowAndRun()
}
