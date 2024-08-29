// Copyright 2024 coScene
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package upload_utils

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/reflow/wordwrap"
)

type manualQuit interface {
	Quit() bool
}

// NewUploadStatusMonitor is used to create a new upload status monitor, note that
// uploadStatusMap and orderedFileList are used to maintain the state of the monitor
// and will change as the underlying map/list changes.
func NewUploadStatusMonitor(uploadStatusMap map[string]*FileInfo, orderedFileList *[]string, hidden bool) tea.Model {
	if !hidden {
		return &UploadStatusMonitor{
			uploadStatusMap: uploadStatusMap,
			orderedFileList: orderedFileList,
			windowWidth:     0,
		}
	}
	return &DummyMonitor{}
}

// UploadStatusMonitor is a bubbletea model that is used to monitor the progress of file uploads
type UploadStatusMonitor struct {
	// orderedFileList is used to maintain the order of the files
	orderedFileList *[]string

	// uploadStatusMap is the source of truth for the progress of each file
	uploadStatusMap map[string]*FileInfo

	// windowWidth is used to calculate the width of the terminal
	windowWidth int

	ManualQuit bool
}

// calculateUploadProgress is used to calculate the progress of a file upload
func (m *UploadStatusMonitor) calculateUploadProgress(name string) float64 {
	status := m.uploadStatusMap[name]
	if status.Size == 0 {
		return 100
	}
	return float64(status.Uploaded) * 100 / float64(status.Size)
}

func (m *UploadStatusMonitor) Init() tea.Cmd {
	return tick()
}

func (m *UploadStatusMonitor) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.windowWidth = msg.Width
	case tea.QuitMsg:
		return m, tea.Quit
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEscape, tea.KeyCtrlD:
			m.ManualQuit = true
			return m, tea.Quit
		}
	case TickMsg:
		return m, tick()
	}
	return m, nil
}

func (m *UploadStatusMonitor) View() string {
	s := "Upload Status:\n"
	skipCount := 0
	successCount := 0
	for _, k := range *m.orderedFileList {
		// Check if the file has been uploaded before
		statusStrLen := m.windowWidth - len(k) - 1
		switch m.uploadStatusMap[k].Status {
		case Unprocessed:
			s += fmt.Sprintf("%s:%*s\n", k, statusStrLen, "Preparing for upload")
		case PreviouslyUploaded:
			s += fmt.Sprintf("%s:%*s\n", k, statusStrLen, "Previously uploaded, skipping")
			skipCount++
		case UploadCompleted:
			s += fmt.Sprintf("%s:%*s\n", k, statusStrLen, "Upload completed")
			successCount++
		case MultipartCompletionInProgress:
			s += fmt.Sprintf("%s:%*s\n", k, statusStrLen, "Completing multipart upload")
		case UploadFailed:
			s += fmt.Sprintf("%s:%*s\n", k, statusStrLen, "Upload failed")
		case UploadInProgress:
			progress := m.calculateUploadProgress(k)
			barWidth := max(m.windowWidth-len(k)-12, 10)                        // Adjust for label and percentage, make sure it is at least 10
			progressCount := min(int(progress*float64(barWidth)/100), barWidth) // min used to prevent float rounding errors
			emptyBar := strings.Repeat("-", barWidth-progressCount)
			progressBar := strings.Repeat("█", progressCount)
			s += fmt.Sprintf("%s: [%s%s] %*.2f%%\n", k, progressBar, emptyBar, 6, progress)
		}
	}

	// Add summary of all file status
	s += "\n"
	s += fmt.Sprintf("Total: %d, Skipped: %d, Success: %d", len(*m.orderedFileList), skipCount, successCount)
	if successCount+skipCount < len(*m.orderedFileList) {
		s += fmt.Sprintf(", Remaining: %d", len(*m.orderedFileList)-successCount-skipCount)
	}
	s += "\n"
	s = wordwrap.String(s, m.windowWidth)
	return s
}

type TickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func (m *UploadStatusMonitor) Quit() bool {
	return m.ManualQuit
}

type DummyMonitor struct {
	ManualQuit bool
}

func (m *DummyMonitor) Init() tea.Cmd {
	return nil
}

func (m *DummyMonitor) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.QuitMsg:
		return m, tea.Quit
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEscape, tea.KeyCtrlD:
			m.ManualQuit = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *DummyMonitor) View() string {
	return ""
}

func (m *DummyMonitor) Quit() bool {
	return m.ManualQuit
}
