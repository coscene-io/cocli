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

package cmd_utils

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// Progress is a simple struct to keep track of the progress of a file upload/download
type Progress struct {
	PrintPrefix string
	TotalSize   int64
	BytesRead   int64
}

// Write is used to satisfy the io.Writer interface.
// Instead of writing somewhere, it simply aggregates
// the total bytes on each read
func (pr *Progress) Write(p []byte) (n int, err error) {
	n, err = len(p), nil
	pr.BytesRead += int64(n)
	pr.Print()
	return
}

// Print displays the current progress of the file upload
// each time Write is called
func (pr *Progress) Print() {
	if pr.BytesRead == pr.TotalSize {
		fmt.Print("\r\033[K")
		return
	}
	fmt.Printf("\r%s: %d/%d %d%%", pr.PrintPrefix, pr.BytesRead, pr.TotalSize, 100*pr.BytesRead/pr.TotalSize)
}

// DownloadFileThroughUrl downloads a single file from the given downloadUrl.
// file is the absolute path of the file to be downloaded.
// downloadUrl is the pre-signed url to download the file from.
func DownloadFileThroughUrl(file string, downloadUrl string) {
	err := os.MkdirAll(filepath.Dir(file), 0755)
	if err != nil {
		log.Errorf("Unable to create directories for file %v", file)
		return
	}

	fileWriter, err := os.Create(file)
	if err != nil {
		log.Errorf("Unable to open file %v for writing", file)
		return
	}
	defer func() { _ = fileWriter.Close() }()

	resp, err := http.Get(downloadUrl)
	if err != nil {
		log.Errorf("Unable to get file from %v", downloadUrl)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	progress := &Progress{
		PrintPrefix: "File download in progress",
		TotalSize:   resp.ContentLength,
		BytesRead:   0,
	}

	tee := io.TeeReader(resp.Body, progress)

	_, err = io.Copy(fileWriter, tee)
	if err != nil {
		log.Errorf("Unable to write file %v", file)
		return
	}
}
