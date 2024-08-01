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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/coscene-io/cocli/internal/fs"
	"github.com/coscene-io/cocli/internal/prompts"
	"github.com/minio/minio-go/v7"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
	"golang.org/x/exp/slices"
)

const (
	uploadIdKeyTemplate     = "STORE-KEY-UPLOAD-ID-%s"
	uploadedSizeKeyTemplate = "STORE-KEY-UPLOADED-SIZE-%s"
	partsKeyTemplate        = "STORE-KEY-PARTS-%s"
	minPartSize             = 1024 * 1024 * 16       // 16MiB
	maxSinglePutObjectSize  = 1024 * 1024 * 1024 * 5 // 5GiB
	uploadDBRelativePath    = ".cocli.uploader.db"
)

type UploadManagerErr struct {
	Err  error
	Path string
}

// UploadManager is a manager for uploading files through minio client.
// Note that it's user's responsibility to check the Errs field after Wait() to see if there's any error.
type UploadManager struct {
	db                       *leveldb.DB
	client                   *minio.Client
	uploadProgressChan       chan prompts.UpdateFileMsg
	statusMonitor            *tea.Program
	statusMonitorStartSignal *sync.WaitGroup
	statusMonitorDoneSignal  *sync.WaitGroup
	Errs                     []UploadManagerErr
	sync.WaitGroup
}

func NewUploadManager(client *minio.Client) (*UploadManager, error) {
	// init db
	curUser, err := user.Current()
	if err != nil {
		return nil, errors.Wrap(err, "Get current user failed")
	}

	uploadDB, err := leveldb.OpenFile(path.Join(curUser.HomeDir, uploadDBRelativePath), nil)
	if err != nil {
		return nil, errors.Wrap(err, "Open level db failed")
	}

	u := &UploadManager{
		uploadProgressChan:       make(chan prompts.UpdateFileMsg, 10),
		db:                       uploadDB,
		client:                   client,
		statusMonitorStartSignal: new(sync.WaitGroup),
		statusMonitorDoneSignal:  new(sync.WaitGroup),
	}

	// statusMonitorStartSignal is to ensure status monitor is ready before sending messages.
	u.statusMonitorStartSignal.Add(1)
	u.statusMonitorDoneSignal.Add(1)
	u.statusMonitor = tea.NewProgram(prompts.NewUploadStatusMonitor(u.statusMonitorStartSignal))
	go u.handleUploadMonitor()
	u.statusMonitorStartSignal.Wait()

	go u.handleUploadProgress()

	return u, nil
}

// Wait waits for all uploads to finish. And wait for status monitor to finish.
func (u *UploadManager) Wait() {
	u.WaitGroup.Wait()
	time.Sleep(1 * time.Second) // Buffer time for status monitor to finish receiving messages.
	u.statusMonitor.Quit()
	u.statusMonitorDoneSignal.Wait()
}

// AddUploadedFile sends a message to status monitor to add uploaded file.
func (u *UploadManager) AddUploadedFile(name string) {
	u.statusMonitor.Send(prompts.AddUploadedFileMsg{Name: name})
}

// FPutObject uploads a file to a bucket with a key and sha256.
// If the file size is larger than minPartSize, it will use multipart upload.
func (u *UploadManager) FPutObject(absPath string, bucket string, key string, sha256 string, userTags map[string]string) error {
	// Check if file sha256 matches.
	actualSha256, size, err := fs.CalSha256AndSize(absPath)
	if err != nil {
		return errors.Wrap(err, "Cal file sha256 failed")
	}
	if actualSha256 != sha256 {
		return errors.New("File sha256 not match, actual: " + actualSha256 + ", expect: " + sha256)
	}

	u.Add(1)
	go func() {
		defer u.Done()
		u.client.TraceOn(log.StandardLogger().WriterLevel(log.DebugLevel))

		if size > int64(minPartSize) {
			u.statusMonitor.Send(prompts.AddFileMsg{Name: absPath, Total: size, IsMultiPart: true})
			err = u.FMultipartPutObject(context.Background(), bucket, key,
				absPath, size, minio.PutObjectOptions{UserTags: userTags})
		} else {
			u.statusMonitor.Send(prompts.AddFileMsg{Name: absPath, Total: size, IsMultiPart: false})
			progress := newUploadProgressReader(absPath, size, u.uploadProgressChan)
			_, err = u.client.FPutObject(context.Background(), bucket, key, absPath,
				minio.PutObjectOptions{Progress: progress, UserTags: userTags})
		}
		if err != nil {
			u.Errs = append(u.Errs, UploadManagerErr{Err: err, Path: absPath})
		}
	}()
	return nil
}

func (u *UploadManager) handleUploadMonitor() {
	defer u.statusMonitorDoneSignal.Done()
	_, err := u.statusMonitor.Run()
	if err != nil {
		log.Fatalf("Error running upload status monitor: %v", err)
	}
}

func (u *UploadManager) handleUploadProgress() {
	for {
		progress := <-u.uploadProgressChan
		u.statusMonitor.Send(progress)
	}
}

func (u *UploadManager) FMultipartPutObject(ctx context.Context, bucket string, key string, filePath string, fileSize int64, opts minio.PutObjectOptions) (err error) {
	// Check for largest object size allowed.
	if fileSize > int64(maxSinglePutObjectSize) {
		return errors.Errorf("Your proposed upload size ‘%d’ exceeds the maximum allowed object size ‘%d’ for single PUT operation.", fileSize, maxSinglePutObjectSize)
	}

	c := minio.Core{Client: u.client}

	// ----------------- Start fetching previous upload info from db -----------------
	// Fetch upload id. If not found, initiate a new multipart upload.
	var uploadId string
	uploadIdKey := fmt.Sprintf(uploadIdKeyTemplate, filePath)
	uploadIdBytes, err := u.db.Get([]byte(uploadIdKey), nil)
	if err != nil {
		log.Debugf("Get upload id by: %s warn: %v", uploadIdKey, err)
	}
	if uploadIdBytes != nil {
		uploadId = string(uploadIdBytes)
		// todo(shuhao): Check if the upload id is still valid.
		//uploads, err := c.ListMultipartUploads(ctx, bucket, key, "", "", "/", 2000)
		//if err != nil {
		//	return errors.Wrap(err, "List multipart uploads failed")
		//}
		//if !lo.ContainsBy(uploads.Uploads, func(u minio.ObjectMultipartInfo) bool {
		//	return u.UploadID == uploadId
		//}) {
		//	uploadId = ""
		//}
	}
	if uploadId == "" {
		uploadId, err = c.NewMultipartUpload(ctx, bucket, key, opts)
		if err != nil {
			return errors.Wrap(err, "New multipart upload failed")
		}
	}
	log.Debugf("Get upload id: %s by: %s", uploadId, uploadIdKey)

	// Fetch uploaded size
	var uploadedSize int64
	uploadedSizeKey := fmt.Sprintf(uploadedSizeKeyTemplate, filePath)
	uploadedSizeBytes, err := u.db.Get([]byte(uploadedSizeKey), nil)
	if err != nil {
		log.Debugf("Get uploaded size by: %s warn: %v", uploadedSizeKey, err)
	}
	if uploadedSizeBytes != nil {
		uploadedSize, err = strconv.ParseInt(string(uploadedSizeBytes), 10, 64)
		if err != nil {
			uploadedSize = 0
		}
	} else {
		uploadedSize = 0
	}
	u.statusMonitor.Send(prompts.UpdateFileMsg{Name: filePath, Uploaded: uploadedSize})
	log.Debugf("Get uploaded size: %d by: %s", uploadedSize, uploadedSizeKey)

	// Fetch uploaded parts
	var parts []minio.CompletePart
	partsKey := fmt.Sprintf(partsKeyTemplate, filePath)
	partsBytes, err := u.db.Get([]byte(partsKey), nil)
	if err != nil {
		log.Debugf("Get uploaded parts by: %s warn: %v", partsKey, err)
	}
	if partsBytes != nil {
		err = json.Unmarshal(partsBytes, &parts)
		if err != nil {
			parts = []minio.CompletePart{}
		}
	} else {
		parts = []minio.CompletePart{}
	}
	partNumbers := lo.Map(parts, func(p minio.CompletePart, _ int) int {
		return p.PartNumber
	})
	log.Debugf("Get uploaded parts: %v by: %s", partNumbers, partsKey)
	// ----------------- End fetching previous upload info from db -----------------

	// todo(shuhao): should handle abort multipart upload on user interrupt.

	// Set contentType based on filepath extension if not given or default
	// value of "application/octet-stream" if the extension has no associated type.
	if opts.ContentType == "" {
		if opts.ContentType = mime.TypeByExtension(filepath.Ext(filePath)); opts.ContentType == "" {
			opts.ContentType = "application/octet-stream"
		}
	}

	if opts.PartSize == 0 {
		opts.PartSize = minPartSize
	}

	// Calculate the optimal parts info for a given size.
	totalPartsCount, partSize, lastPartSize, err := minio.OptimalPartInfo(fileSize, opts.PartSize)
	if err != nil {
		return errors.Wrap(err, "Optimal part info failed")
	}

	// Declare a channel that sends the next part number to be uploaded.
	uploadPartsCh := make(chan int)
	// Declare a channel that sends back the response of a part upload.
	uploadedPartsCh := make(chan uploadedPartRes)
	// Used for readability, lastPartNumber is always totalPartsCount.
	lastPartNumber := totalPartsCount

	// Send each part number to the channel to be processed.
	go func() {
		defer close(uploadPartsCh)
		for p := 1; p <= totalPartsCount; p++ {
			if slices.Contains(partNumbers, p) {
				log.Debugf("Part: %d already uploaded", p)
				continue
			}
			log.Debugf("Part: %d need to upload", p)
			uploadPartsCh <- p
		}
	}()

	if opts.NumThreads == 0 {
		opts.NumThreads = 4
	}

	// Get reader of the file to be uploaded.
	fileReader, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer fileReader.Close()

	// Starts parallel uploads.
	// Receive the part number to upload from the uploadPartsCh channel.
	for w := 1; w <= int(opts.NumThreads); w++ {
		go func() {
			for {
				var partToUpload int
				var ok bool
				select {
				case <-ctx.Done():
					return
				case partToUpload, ok = <-uploadPartsCh:
					if !ok {
						return
					}
				}

				// Calculate the offset and size for the part to be uploaded.
				readOffset := int64(partToUpload-1) * partSize
				curPartSize := partSize
				if partToUpload == lastPartNumber {
					curPartSize = lastPartSize
				}

				sectionReader := io.NewSectionReader(fileReader, readOffset, curPartSize)
				log.Debugf("Uploading part: %d", partToUpload)
				objPart, err := c.PutObjectPart(ctx, bucket, key, uploadId, partToUpload, sectionReader, curPartSize, minio.PutObjectPartOptions{SSE: opts.ServerSideEncryption})
				if err != nil {
					log.Debugf("Upload part: %d failed: %v", partToUpload, err)
					uploadedPartsCh <- uploadedPartRes{
						Error: err,
					}
				} else {
					log.Debugf("Upload part: %d success", partToUpload)
					uploadedPartsCh <- uploadedPartRes{
						Part: objPart,
					}
				}
			}
		}()
	}

	// Gather the responses as they occur and update any progress bar
	numToUpload := totalPartsCount - len(partNumbers)
	for m := 1; m <= numToUpload; m++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case uploadRes := <-uploadedPartsCh:
			if uploadRes.Error != nil {
				return uploadRes.Error
			}
			// Update the uploadedSize.
			uploadedSize += uploadRes.Part.Size
			parts = append(parts, minio.CompletePart{
				ETag:           uploadRes.Part.ETag,
				PartNumber:     uploadRes.Part.PartNumber,
				ChecksumCRC32:  uploadRes.Part.ChecksumCRC32,
				ChecksumCRC32C: uploadRes.Part.ChecksumCRC32C,
				ChecksumSHA1:   uploadRes.Part.ChecksumSHA1,
				ChecksumSHA256: uploadRes.Part.ChecksumSHA256,
			})

			partsJsonBytes, err := json.Marshal(parts)
			if err != nil {
				log.Fatalf("Marshal parts failed: %v", err)
			}
			batch := new(leveldb.Batch)
			batch.Put([]byte(uploadIdKey), []byte(uploadId))
			batch.Put([]byte(partsKey), partsJsonBytes)
			batch.Put([]byte(uploadedSizeKey), []byte(strconv.FormatInt(uploadedSize, 10)))
			err = u.db.Write(batch, nil)
			if err != nil {
				log.Errorf("Store uploaded parts err: %v", err)
			}
			u.uploadProgressChan <- prompts.UpdateFileMsg{Name: filePath, Uploaded: uploadedSize}
		}
	}

	// Verify if we uploaded all the data.
	if uploadedSize != fileSize {
		return errors.Wrapf(err, "Uploaded size: %d, file size: %d, does not match", uploadedSize, fileSize)
	}

	// Sort all completed parts.
	slices.SortFunc(parts, func(i, j minio.CompletePart) int {
		return i.PartNumber - j.PartNumber
	})

	_, err = c.CompleteMultipartUpload(ctx, bucket, key, uploadId, parts, opts)
	if err != nil {
		return errors.Wrapf(err, "Complete multipart upload failed")
	}

	batchDelete := new(leveldb.Batch)
	batchDelete.Delete([]byte(uploadIdKey))
	batchDelete.Delete([]byte(partsKey))
	batchDelete.Delete([]byte(uploadedSizeKey))
	err = u.db.Write(batchDelete, nil)
	if err != nil {
		return errors.Wrapf(err, "Batch delete parts failed")
	}
	u.statusMonitor.Send(prompts.CompleteMultipartMsg{Name: filePath})

	return nil
}

type uploadProgressReader struct {
	absPath            string
	total              int64
	uploaded           int64
	uploadProgressChan chan prompts.UpdateFileMsg
}

func newUploadProgressReader(absPath string, total int64, uploadProgressChan chan prompts.UpdateFileMsg) *uploadProgressReader {
	uploadProgressChan <- prompts.UpdateFileMsg{Name: absPath, Uploaded: 0}
	return &uploadProgressReader{absPath: absPath, total: total, uploaded: 0, uploadProgressChan: uploadProgressChan}
}

func (r *uploadProgressReader) Read(b []byte) (int, error) {
	n := int64(len(b))
	r.uploaded += n
	r.uploadProgressChan <- prompts.UpdateFileMsg{Name: r.absPath, Uploaded: r.uploaded}
	return int(n), nil
}

// uploadedPartRes - the response received from a part upload.
type uploadedPartRes struct {
	Error error // Any error encountered while uploading the part.
	Part  minio.ObjectPart
}
