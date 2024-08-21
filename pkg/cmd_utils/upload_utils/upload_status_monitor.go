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
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/reflow/wordwrap"
)

// UploadStatusMonitor is a bubbletea model that is used to monitor the progress of file uploads
type UploadStatusMonitor struct {
	// orderedFileList is used to maintain the order of the files
	orderedFileList []string

	// uploadStatusMap is the source of truth for the progress of each file
	uploadStatusMap map[string]*uploadStatus

	// windowWidth is used to calculate the width of the terminal
	windowWidth int

	// startSignal is used to signal the upload status monitor has finished initialization
	startSignal *sync.WaitGroup

	ManualQuit bool
}

func NewUploadStatusMonitor(startSignal *sync.WaitGroup) *UploadStatusMonitor {
	startSignal.Add(1)
	return &UploadStatusMonitor{
		uploadStatusMap: make(map[string]*uploadStatus),
		orderedFileList: []string{},
		windowWidth:     0,
		startSignal:     startSignal,
	}
}

// AddFileMsg is a message that is used to add a file to the upload status monitor
type AddFileMsg struct {
	Name string
}

type CanUpdateStatus interface {
	UpdateStatus(m *UploadStatusMonitor)
}

func (msg AddFileMsg) UpdateStatus(m *UploadStatusMonitor) {
	m.orderedFileList = append(m.orderedFileList, msg.Name)
	m.uploadStatusMap[msg.Name] = &uploadStatus{
		total:    0,
		uploaded: 0,
		status:   Unprocessed,
	}
}

type UpdateStatusMsg struct {
	Name     string
	Total    int64
	Uploaded int64
	Status   UploadStatusEnum
}

func (msg UpdateStatusMsg) UpdateStatus(m *UploadStatusMonitor) {
	if msg.Total > 0 {
		m.uploadStatusMap[msg.Name].total = msg.Total
	}
	if msg.Uploaded > 0 {
		m.uploadStatusMap[msg.Name].uploaded += msg.Uploaded
	}
	if msg.Status != Unprocessed {
		m.uploadStatusMap[msg.Name].status = msg.Status
	}
}

// calculateUploadProgress is used to calculate the progress of a file upload
func (m *UploadStatusMonitor) calculateUploadProgress(name string) float64 {
	status := m.uploadStatusMap[name]
	return float64(status.uploaded) * 100 / float64(status.total)
}

func (m *UploadStatusMonitor) Init() tea.Cmd {
	m.startSignal.Done()
	return nil
}

func (m *UploadStatusMonitor) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.windowWidth = msg.Width
	case CanUpdateStatus:
		msg.UpdateStatus(m)
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

func (m *UploadStatusMonitor) View() string {
	s := "Upload Status:\n"
	skipCount := 0
	successCount := 0
	for _, k := range m.orderedFileList {
		// Check if the file has been uploaded before
		statusStrLen := m.windowWidth - len(k) - 1
		switch m.uploadStatusMap[k].status {
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
			barWidth := max(m.windowWidth-len(k)-12, 10)                // Adjust for label and percentage, make sure it is at least 10
			progressCount := min(int(progress*float64(barWidth)/100), barWidth) // min used to prevent float rounding errors
			emptyBar := strings.Repeat("-", barWidth-progressCount)
			progressBar := strings.Repeat("█", progressCount)
			s += fmt.Sprintf("%s: [%s%s] %*.2f%%\n", k, progressBar, emptyBar, 6, progress)
		}
	}

	// Add summary of all file status
	s += "\n"
	s += fmt.Sprintf("Total: %d, Skipped: %d, Success: %d", len(m.orderedFileList), skipCount, successCount)
	if successCount+skipCount < len(m.orderedFileList) {
		s += fmt.Sprintf(", Remaining: %d", len(m.orderedFileList)-successCount-skipCount)
	}
	s += "\n"
	s = wordwrap.String(s, m.windowWidth)
	return s
}

// UploadStatusEnum is used to keep track of the state of a file upload
type UploadStatusEnum int

const (
	// Unprocessed is used to indicate that the file has not been processed yet
	Unprocessed UploadStatusEnum = iota

	// PreviouslyUploaded is used to indicate that the file has been uploaded before
	PreviouslyUploaded

	// UploadInProgress is used to indicate that the file upload is in progress
	UploadInProgress

	// UploadCompleted is used to indicate that the file upload has completed
	UploadCompleted

	// MultipartCompletionInProgress is used to indicate that the multipart upload completion is in progress
	MultipartCompletionInProgress

	// UploadFailed is used to indicate that the file upload has failed
	UploadFailed
)

// uploadStatus is used to keep track of the progress of a file upload
type uploadStatus struct {
	total    int64
	uploaded int64
	status   UploadStatusEnum
}
