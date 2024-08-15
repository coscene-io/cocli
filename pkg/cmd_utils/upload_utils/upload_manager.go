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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
	minPartSize             = 1024 * 1024 * 16         // 16MiB
	maxSinglePutObjectSize  = 1024 * 1024 * 1024 * 500 // 5GiB
	uploadDBRelativePath    = ".cocli.uploader.db"
)

// FileInfo contains the path, size and sha256 of a file.
type FileInfo struct {
	Path   string
	Size   int64
	Sha256 string
}

// UploadManager is a manager for uploading files through minio client.
// Note that it's user's responsibility to check the Errs field after Wait() to see if there's any error.
type UploadManager struct {
	db                      *leveldb.DB
	client                  *minio.Client
	uploadProgressChan      chan UpdateStatusMsg
	statusMonitorDoneSignal *sync.WaitGroup
	StatusMonitor           *tea.Program
	FileInfos               map[string]FileInfo
	Errs                    map[string]error
	sync.WaitGroup
}

func NewUploadManager(client *minio.Client) (*UploadManager, error) {
	// init db
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, errors.Wrap(err, "Get current user home dir failed")
	}

	uploadDB, err := leveldb.OpenFile(path.Join(homeDir, uploadDBRelativePath), nil)
	if err != nil {
		return nil, errors.Wrap(err, "Open level db failed")
	}

	um := &UploadManager{
		uploadProgressChan:      make(chan UpdateStatusMsg, 10),
		db:                      uploadDB,
		client:                  client,
		statusMonitorDoneSignal: new(sync.WaitGroup),
		FileInfos:               make(map[string]FileInfo),
		Errs:                    make(map[string]error),
	}

	// statusMonitorStartSignal is to ensure status monitor is ready before sending messages.
	statusMonitorStartSignal := new(sync.WaitGroup)
	statusMonitorStartSignal.Add(1)
	um.statusMonitorDoneSignal.Add(1)
	um.StatusMonitor = tea.NewProgram(NewUploadStatusMonitor(statusMonitorStartSignal))
	go um.runUploadStatusMonitor()
	statusMonitorStartSignal.Wait()

	go um.handleUploadProgress()
	return um, nil
}

func (um *UploadManager) runUploadStatusMonitor() {
	defer um.statusMonitorDoneSignal.Done()
	_, err := um.StatusMonitor.Run()
	if err != nil {
		log.Fatalf("Error running upload status monitor: %v", err)
	}
}

func (um *UploadManager) handleUploadProgress() {
	for {
		progress := <-um.uploadProgressChan
		um.StatusMonitor.Send(progress)
	}
}

// Wait waits for all uploads to finish. And wait for status monitor to finish.
func (um *UploadManager) Wait() {
	um.WaitGroup.Wait()
	time.Sleep(1 * time.Second) // Buffer time for status monitor to finish receiving messages.
	um.StatusMonitor.Quit()
	um.statusMonitorDoneSignal.Wait()
}

// AddErr adds an error to the manager.
func (um *UploadManager) AddErr(path string, err error) {
	um.StatusMonitor.Send(UpdateStatusMsg{
		Name:   path,
		Status: UploadFailed,
	})
	um.Errs[path] = err
}

// FPutObject uploads a file to a bucket with a key and sha256.
// If the file size is larger than minPartSize, it will use multipart upload.
func (um *UploadManager) FPutObject(absPath string, bucket string, key string, userTags map[string]string) {
	// Check if file sha256 matches.
	fileInfo, ok := um.FileInfos[absPath]
	if !ok {
		um.AddErr(absPath, errors.New("File info not found"))
		return
	}

	um.Add(1)
	go func() {
		defer um.Done()
		um.client.TraceOn(log.StandardLogger().WriterLevel(log.DebugLevel))

		var err error
		if fileInfo.Size > int64(minPartSize) {
			err = um.FMultipartPutObject(context.Background(), bucket, key,
				absPath, fileInfo.Size, minio.PutObjectOptions{UserTags: userTags})
		} else {
			progress := newUploadProgressReader(absPath, fileInfo.Size, um.uploadProgressChan)
			um.StatusMonitor.Send(UpdateStatusMsg{Name: absPath, Status: UploadInProgress})
			_, err = um.client.FPutObject(context.Background(), bucket, key, absPath,
				minio.PutObjectOptions{Progress: progress, UserTags: userTags})
		}
		if err != nil {
			um.AddErr(absPath, err)
		}
	}()
}

func (um *UploadManager) FMultipartPutObject(ctx context.Context, bucket string, key string, filePath string, fileSize int64, opts minio.PutObjectOptions) (err error) {
	// Check for largest object size allowed.
	if fileSize > int64(maxSinglePutObjectSize) {
		return errors.Errorf("Your proposed upload size ‘%d’ exceeds the maximum allowed object size ‘%d’ for single PUT operation.", fileSize, maxSinglePutObjectSize)
	}

	c := minio.Core{Client: um.client}

	// ----------------- Start fetching previous upload info from db -----------------
	// Fetch upload id. If not found, initiate a new multipart upload.
	var uploadId string
	uploadIdKey := fmt.Sprintf(uploadIdKeyTemplate, filePath)
	uploadIdBytes, err := um.db.Get([]byte(uploadIdKey), nil)
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
	uploadedSizeBytes, err := um.db.Get([]byte(uploadedSizeKey), nil)
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
	um.StatusMonitor.Send(UpdateStatusMsg{Name: filePath, Uploaded: uploadedSize, Status: UploadInProgress})
	log.Debugf("Get uploaded size: %d by: %s", uploadedSize, uploadedSizeKey)

	// Fetch uploaded parts
	var parts []minio.CompletePart
	partsKey := fmt.Sprintf(partsKeyTemplate, filePath)
	partsBytes, err := um.db.Get([]byte(partsKey), nil)
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
			err = um.db.Write(batch, nil)
			if err != nil {
				log.Errorf("Store uploaded parts err: %v", err)
			}
			um.uploadProgressChan <- UpdateStatusMsg{Name: filePath, Uploaded: uploadedSize}
		}
	}

	um.StatusMonitor.Send(UpdateStatusMsg{Name: filePath, Status: MultipartCompletionInProgress})

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
	err = um.db.Write(batchDelete, nil)
	if err != nil {
		return errors.Wrapf(err, "Batch delete parts failed")
	}
	um.StatusMonitor.Send(UpdateStatusMsg{Name: filePath, Status: UploadCompleted})

	return nil
}

type uploadProgressReader struct {
	absPath            string
	total              int64
	uploaded           int64
	uploadProgressChan chan UpdateStatusMsg
}

func newUploadProgressReader(absPath string, total int64, uploadProgressChan chan UpdateStatusMsg) *uploadProgressReader {
	uploadProgressChan <- UpdateStatusMsg{Name: absPath, Uploaded: 0}
	return &uploadProgressReader{absPath: absPath, total: total, uploaded: 0, uploadProgressChan: uploadProgressChan}
}

func (r *uploadProgressReader) Read(b []byte) (int, error) {
	n := int64(len(b))
	r.uploaded += n

	updateMsg := UpdateStatusMsg{Name: r.absPath, Uploaded: r.uploaded}
	if r.uploaded == r.total {
		updateMsg.Status = UploadCompleted
	}
	r.uploadProgressChan <- updateMsg
	return int(n), nil
}

// uploadedPartRes - the response received from a part upload.
type uploadedPartRes struct {
	Error error // Any error encountered while uploading the part.
	Part  minio.ObjectPart
}
