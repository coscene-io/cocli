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

package prompts

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

	startSignal *sync.WaitGroup
}

func NewUploadStatusMonitor(startSignal *sync.WaitGroup) *UploadStatusMonitor {
	return &UploadStatusMonitor{
		uploadStatusMap: make(map[string]*uploadStatus),
		orderedFileList: []string{},
		windowWidth:     0,
		startSignal:     startSignal,
	}
}

// AddUploadedFileMsg is a message that is used to add a file that has been uploaded before
type AddUploadedFileMsg struct {
	Name string
}

// addUploadedFile is used to add a file that has been uploaded before
func (m *UploadStatusMonitor) addUploadedFile(name string) {
	m.orderedFileList = append(m.orderedFileList, name)
	m.uploadStatusMap[name] = &uploadStatus{
		completionStatus: previouslyUploaded,
	}
}

// AddFileMsg is a message that is used to add a file to the upload status monitor
type AddFileMsg struct {
	Name        string
	Total       int64
	IsMultiPart bool
}

// addFile is used to add a file to the upload status monitor
func (m *UploadStatusMonitor) addFile(name string, size int64, isMultiPart bool) {
	m.orderedFileList = append(m.orderedFileList, name)
	m.uploadStatusMap[name] = &uploadStatus{
		total:            size,
		uploaded:         0,
		completionStatus: normalCompletion,
	}
	if isMultiPart {
		m.uploadStatusMap[name].completionStatus = multipartInProgress
	}
}

// UpdateFileMsg is a message that is used to update the progress of a file
type UpdateFileMsg struct {
	Name     string
	Uploaded int64
}

// updateFile is used to update the progress of a file
func (m *UploadStatusMonitor) updateFile(name string, uploaded int64) {
	m.uploadStatusMap[name].uploaded = uploaded
}

// CompleteMultipartMsg is a message that is used to mark a multipart upload as completed
type CompleteMultipartMsg struct {
	Name string
}

// completeMultipart is used to mark a multipart upload as completed
func (m *UploadStatusMonitor) completeMultipart(name string) {
	m.uploadStatusMap[name].completionStatus = multipartCompleted
}

// calculateUploadProgress is used to calculate the progress of a file upload
func (m *UploadStatusMonitor) calculateUploadProgress(name string) float64 {
	status := m.uploadStatusMap[name]

	// Since multipart upload has an addition completion step that takes time
	// we will show 99% until the completion step is done
	if status.uploaded == status.total {
		if status.completionStatus == multipartInProgress {
			return 99
		} else if status.completionStatus == multipartCompleted {
			return 100
		}
	}
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
	case AddUploadedFileMsg:
		m.addUploadedFile(msg.Name)
	case AddFileMsg:
		m.addFile(msg.Name, msg.Total, msg.IsMultiPart)
	case UpdateFileMsg:
		m.updateFile(msg.Name, msg.Uploaded)
	case CompleteMultipartMsg:
		m.completeMultipart(msg.Name)
	case tea.QuitMsg:
		return m, tea.Quit
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEscape, tea.KeyCtrlD:
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
		if m.uploadStatusMap[k].completionStatus == previouslyUploaded {
			uploadedStr := "Previously uploaded, skipping"
			spaceWidth := m.windowWidth - len(k) - len(uploadedStr) - 1
			s += fmt.Sprintf("%s:%s%s\n", k, strings.Repeat(" ", spaceWidth), uploadedStr)
			skipCount++
			continue
		}

		// Display progress bar for each entry
		progress := m.calculateUploadProgress(k)
		barWidth := m.windowWidth - len(k) - 12                             // Adjust for label and percentage
		progressCount := min(int(progress*float64(barWidth)/100), barWidth) // min used to prevent float rounding errors
		emptyCount := barWidth - progressCount
		progressBar := strings.Repeat("█", progressCount)
		emptyBar := strings.Repeat(" ", emptyCount)
		s += fmt.Sprintf("%s: [%s%s] %.2f%%\n", k, progressBar, emptyBar, progress)
		if progress == 100 {
			successCount++
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

// completionStatus is used to track the completion status of a file upload
type completionStatus int

const (
	// normalCompletion is used to indicate that the file upload completes normally
	normalCompletion completionStatus = iota
	// multipartInProgress is used to indicate that the multipart upload is in progress
	multipartInProgress
	// multipartCompleted is used to indicate that the multipart upload has completed
	multipartCompleted
	// previouslyUploaded is used to indicate that the file has been uploaded before
	previouslyUploaded
)

// uploadStatus is used to keep track of the progress of a file upload
type uploadStatus struct {
	total            int64
	uploaded         int64
	completionStatus completionStatus
}
