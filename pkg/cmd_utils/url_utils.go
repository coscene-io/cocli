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
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	retryWaitMin = 1 * time.Second
	retryWaitMax = 5 * time.Second
)

// Progress is a simple struct to keep track of the progress of a file upload/download
type Progress struct {
	PrintPrefix string
	TotalSize   int64
	BytesRead   int64
	Retry       int
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
		postFix := ""
		if pr.Retry > 0 {
			postFix = fmt.Sprintf("on %d retries", pr.Retry)
		}
		fmt.Printf("\r\033[KFile successfully downloaded %s\n", postFix)
		return
	}

	retryHint := ""
	if pr.Retry > 0 {
		retryHint = fmt.Sprintf("(Retry #%d) ", pr.Retry)
	}
	fmt.Printf("\r\033[K%s%s: %d/%d %d%%", retryHint, pr.PrintPrefix, pr.BytesRead, pr.TotalSize, 100*pr.BytesRead/pr.TotalSize)
}

// DownloadFileThroughUrl downloads a single file from the given downloadUrl.
// file is the absolute path of the file to be downloaded.
// downloadUrl is the pre-signed url to download the file from.
func DownloadFileThroughUrl(file string, downloadUrl string, maxRetries int) error {
	err := os.MkdirAll(filepath.Dir(file), 0755)
	if err != nil {
		return errors.Wrapf(err, "unable to create directories for file %v", file)
	}

	fileWriter, err := os.Create(file)
	if err != nil {
		return errors.Wrapf(err, "unable to open file %v for writing", file)
	}
	defer func() { _ = fileWriter.Close() }()

	for attempt := 0; attempt <= maxRetries; attempt++ {
		err = downloadWithFileWriter(fileWriter, downloadUrl, attempt)
		if err == nil {
			return nil
		}

		retryPrefix := ""
		if attempt > 0 {
			retryPrefix = fmt.Sprintf("(Retry #%d) ", attempt)
		}
		log.Errorf("%sUnable to download file: %v", retryPrefix, err)

		if attempt < maxRetries {
			time.Sleep(retryablehttp.DefaultBackoff(retryWaitMin, retryWaitMax, attempt, nil))
		}
	}

	return errors.Errorf("unable to download file %v after %d retries", file, maxRetries)
}

// downloadWithFileWriter downloads the file from the given downloadUrl and writes it to the fileWriter.
// It also updates the progress of the download.
func downloadWithFileWriter(fileWriter *os.File, downloadUrl string, retry int) error {
	defer fmt.Print("\r\033[K")

	resp, err := http.Get(downloadUrl)
	if err != nil {
		return errors.Wrapf(err, "unable to get file from url %v", downloadUrl)
	}
	defer func() { _ = resp.Body.Close() }()

	progress := &Progress{
		PrintPrefix: "File download in progress",
		TotalSize:   resp.ContentLength,
		BytesRead:   0,
		Retry:       retry,
	}

	tee := io.TeeReader(resp.Body, progress)

	_, err = io.Copy(fileWriter, tee)
	if err != nil {
		return errors.Wrapf(err, "unable to write file %v", fileWriter.Name())
	}

	return nil
}
