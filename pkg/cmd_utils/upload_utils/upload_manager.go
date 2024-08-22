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
	"github.com/coscene-io/cocli/internal/config"
	"github.com/coscene-io/cocli/internal/constants"
	"github.com/coscene-io/cocli/internal/name"
	"github.com/coscene-io/cocli/pkg/cmd_utils"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"io"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/minio/minio-go/v7"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

const (
	userTagRecordIdKey     = "X-COS-RECORD-ID"
	uploadIdKey            = "STORE-KEY-UPLOAD-ID"
	uploadedSizeKey        = "STORE-KEY-UPLOADED-SIZE"
	partsKey               = "STORE-KEY-PARTS"
	maxSinglePutObjectSize = 1024 * 1024 * 1024 * 500 // 500GiB
	defaultWindowSize      = 1024 * 1024 * 1024       // 1GiB
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
	opts                    *MultipartOpts
	client                  *minio.Client
	uploadProgressChan      chan UpdateStatusMsg
	statusMonitorDoneSignal *sync.WaitGroup
	StatusMonitor           *tea.Program
	isDebug                 bool
	FileInfos               map[string]FileInfo
	Errs                    map[string]error
	sync.WaitGroup
}

func NewUploadManagerFromConfig(pm *config.ProfileManager, proj *name.Project, timeout time.Duration, multiOpts *MultipartOpts) (*UploadManager, error) {
	generateSecurityTokenRes, err := pm.SecurityTokenCli().GenerateSecurityToken(context.Background(), proj.String())
	if err != nil {
		return nil, errors.Wrap(err, "unable to generate security token")
	}
	mc, err := minio.New(generateSecurityTokenRes.Endpoint, &minio.Options{
		Creds:     credentials.NewStaticV4(generateSecurityTokenRes.GetAccessKeyId(), generateSecurityTokenRes.GetAccessKeySecret(), generateSecurityTokenRes.GetSessionToken()),
		Secure:    true,
		Region:    "",
		Transport: cmd_utils.NewTransport(timeout),
	})
	if err != nil {
		return nil, errors.Wrap(err, "unable to create minio client")
	}
	return NewUploadManager(mc, multiOpts)
}

func NewUploadManager(client *minio.Client, opts *MultipartOpts) (*UploadManager, error) {
	um := &UploadManager{
		opts:                    opts,
		uploadProgressChan:      make(chan UpdateStatusMsg, 10),
		client:                  client,
		statusMonitorDoneSignal: new(sync.WaitGroup),
		isDebug:                 log.GetLevel() == log.DebugLevel,
		FileInfos:               make(map[string]FileInfo),
		Errs:                    make(map[string]error),
	}

	// statusMonitorStartSignal is to ensure status monitor is ready before sending messages.
	statusMonitorStartSignal := new(sync.WaitGroup)
	um.statusMonitorDoneSignal.Add(1)
	um.StatusMonitor = tea.NewProgram(NewUploadStatusMonitor(statusMonitorStartSignal))
	go um.runUploadStatusMonitor()
	statusMonitorStartSignal.Wait()

	go um.handleUploadProgress()
	return um, nil
}

func (um *UploadManager) Debugf(format string, args ...interface{}) {
	if um.isDebug {
		msg := fmt.Sprintf(format, args...)
		um.StatusMonitor.Printf("DEBUG: %s\n", msg)
	}
}

func (um *UploadManager) runUploadStatusMonitor() {
	defer um.statusMonitorDoneSignal.Done()
	finalModel, err := um.StatusMonitor.Run()
	if err != nil {
		log.Fatalf("Error running upload status monitor: %v", err)
	}
	um.PrintErrs()
	if finalModel.(*UploadStatusMonitor).ManualQuit {
		log.Fatalf("Upload status monitor quit manually")
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

// PrintErrs prints all errors.
func (um *UploadManager) PrintErrs() {
	if len(um.Errs) > 0 {
		fmt.Printf("\n%d files failed to upload\n", len(um.Errs))
		for kPath, vErr := range um.Errs {
			fmt.Printf("Upload %v failed with: \n%v\n\n", kPath, vErr)
		}
		return
	}
}

// UploadFileThroughUrl uploads a single file to the given uploadUrl.
// um is the upload manager to use.
// file is the absolute path of the file to be uploaded.
// uploadUrl is the pre-signed url to upload the file to.
func (um *UploadManager) UploadFileThroughUrl(file string, uploadUrl string) error {
	parsedUrl, err := url.Parse(uploadUrl)
	if err != nil {
		return errors.Wrap(err, "parse upload url failed")
	}

	// Parse tags
	tagsMap, err := url.ParseQuery(parsedUrl.Query().Get("X-Amz-Tagging"))
	if err != nil {
		return errors.Wrap(err, "parse tags failed")
	}
	tags := lo.MapValues(tagsMap, func(value []string, _ string) string {
		if len(value) == 0 {
			return ""
		}
		return value[0]
	})

	// Parse bucket and key
	if !strings.HasPrefix(parsedUrl.Path, "/default/") {
		return errors.New("invalid upload url")
	}
	key := strings.TrimPrefix(parsedUrl.Path, "/default/")

	um.FPutObject(file, "default", key, tags)
	return nil
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
		um.client.TraceOn(log.StandardLogger().WriterLevel(log.TraceLevel))

		size, err := um.opts.partSize()
		if err != nil {
			um.AddErr(absPath, err)
			return
		}

		if fileInfo.Size > int64(size) {
			err = um.FMultipartPutObject(context.Background(), bucket, key,
				absPath, fileInfo.Size, fileInfo.Sha256, minio.PutObjectOptions{UserTags: userTags, PartSize: size, NumThreads: um.opts.Threads})
		} else {
			progress := &uploadProgressReader{
				absPath:            absPath,
				total:              fileInfo.Size,
				uploadProgressChan: um.uploadProgressChan,
			}
			um.StatusMonitor.Send(UpdateStatusMsg{Name: absPath, Status: UploadInProgress})
			_, err = um.client.FPutObject(context.Background(), bucket, key, absPath,
				minio.PutObjectOptions{Progress: progress, UserTags: userTags})
		}
		if err != nil {
			um.AddErr(absPath, err)
		}
	}()
}

func (um *UploadManager) FMultipartPutObject(ctx context.Context, bucket string, key string, filePath string, fileSize int64, fileSha256 string, opts minio.PutObjectOptions) (err error) {
	// Check for largest object size allowed.
	if fileSize > int64(maxSinglePutObjectSize) {
		return errors.Errorf("Your proposed upload size ‘%d’ exceeds the maximum allowed object size ‘%d’ for single PUT operation.", fileSize, maxSinglePutObjectSize)
	}

	c := minio.Core{Client: um.client}

	// Create uploader directory if not exists
	if err = os.MkdirAll(constants.DefaultUploaderDirPath, 0755); err != nil {
		return errors.Wrap(err, "Create uploader directory failed")
	}

	// Create uploader db
	db, err := NewUploadDB(filePath, opts.UserTags[userTagRecordIdKey], fileSha256)
	if err != nil {
		return errors.Wrap(err, "Create uploader db failed")
	}
	defer db.Close()

	// ----------------- Start fetching previous upload info from db -----------------
	// Fetch upload id. If not found, initiate a new multipart upload.
	var uploadId string
	uploadIdBytes, err := db.Get(uploadIdKey)
	if err != nil {
		um.Debugf("Get upload id by: %s warn: %v", uploadIdKey, err)
	}
	if uploadIdBytes != nil {
		uploadId = string(uploadIdBytes)
		// todo(shuhao): Check if the upload id is still valid.
		//uploads, err := c.ListMultipartUploads(ctx, bucket, key, "", "", "/", 2000)
		//if err != nil {
		//	return errors.Wrap(err, "List multipart uploads failed")
		//}
		//um.StatusMonitor.Println("uploads: ", uploads)
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
	um.Debugf("Get upload id: %s by: %s", uploadId, uploadIdKey)

	// Fetch uploaded size
	var uploadedSize int64
	uploadedSizeBytes, err := db.Get(uploadedSizeKey)
	if err != nil {
		um.Debugf("Get uploaded size by: %s warn: %v", uploadedSizeKey, err)
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
	um.Debugf("Get uploaded size: %d by: %s", uploadedSize, uploadedSizeKey)

	// Fetch uploaded parts
	var parts []minio.CompletePart
	partsBytes, err := db.Get(partsKey)
	if err != nil {
		um.Debugf("Get uploaded parts by: %s warn: %v", partsKey, err)
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
	sort.Ints(partNumbers)
	um.Debugf("Get uploaded parts: %v by: %s", partNumbers, partsKey)
	// ----------------- End fetching previous upload info from db -----------------

	// todo(shuhao): should handle abort multipart upload on user interrupt.

	// Set contentType based on filepath extension if not given or default
	// value of "application/octet-stream" if the extension has no associated type.
	if opts.ContentType == "" {
		if opts.ContentType = mime.TypeByExtension(filepath.Ext(filePath)); opts.ContentType == "" {
			opts.ContentType = "application/octet-stream"
		}
	}

	// Calculate the optimal parts info for a given size.
	totalPartsCount, partSize, lastPartSize, err := minio.OptimalPartInfo(fileSize, opts.PartSize)
	if err != nil {
		return errors.Wrap(err, "Optimal part info failed")
	}
	um.Debugf("Total part: %v, part size: %v, last part size: %v", totalPartsCount, partSize, lastPartSize)

	// Declare a channel that sends the next part number to be uploaded.
	uploadPartsCh := make(chan int, opts.NumThreads)
	// Declare a channel that sends back the response of a part upload.
	uploadedPartsCh := make(chan uploadedPartRes, opts.NumThreads)
	// Declare a channel that sends back the completed part numbers.
	completedPartsCh := make(chan int, opts.NumThreads)

	// Send each part number to the channel to be processed.
	go func() {
		defer close(uploadPartsCh)

		windowSize := defaultWindowSize
		// Make sure at least one part is uploading.
		if windowSize < int(opts.PartSize) {
			windowSize = int(opts.PartSize)
		}
		uploadingParts := NewHeap(make([]int, 0, opts.NumThreads))

		curPart := FindMinMissingInteger(partNumbers)
		// minPart is the minimum part number present in the window.
		minPart := curPart

		for {
			if uploadingParts.Len() > 0 {
				// Wait for a part to complete.
				select {
				case <-ctx.Done():
					return
				case partNumber := <-completedPartsCh:
					uploadingParts.Remove(partNumber)
					minPart = uploadingParts.Peek()
					um.Debugf("completed part received: %d", partNumber)
				default:
				}
			}

			// Upload parts in window.
			for curPart <= totalPartsCount && curPart < minPart+windowSize/int(partSize) {
				if !slices.Contains(partNumbers, curPart) {
					um.Debugf("sending part to be uploaded: %d", curPart)
					uploadingParts.Push(curPart)
					uploadPartsCh <- curPart
				}
				curPart++
			}
		}
	}()

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
				if partToUpload == totalPartsCount {
					curPartSize = lastPartSize
				}

				sectionReader := &uploadProgressSectionReader{
					SectionReader:      io.NewSectionReader(fileReader, readOffset, curPartSize),
					uploadProgressChan: um.uploadProgressChan,
					absPath:            filePath,
				}
				um.Debugf("Uploading part: %d", partToUpload)
				objPart, err := c.PutObjectPart(ctx, bucket, key, uploadId, partToUpload, sectionReader, curPartSize, minio.PutObjectPartOptions{SSE: opts.ServerSideEncryption})
				if err != nil {
					um.Debugf("Upload part: %d failed: %v", partToUpload, err)
					uploadedPartsCh <- uploadedPartRes{
						Error: err,
					}
				} else {
					um.Debugf("Upload part: %d success", partToUpload)
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
				return errors.Wrapf(err, "Marshal parts failed")
			}
			batch := map[string][]byte{
				uploadIdKey:     []byte(uploadId),
				partsKey:        partsJsonBytes,
				uploadedSizeKey: []byte(strconv.FormatInt(uploadedSize, 10)),
			}
			if err = db.BatchPut(batch); err != nil {
				return errors.Wrapf(err, "Batch write parts failed")
			}
			completedPartsCh <- uploadRes.Part.PartNumber
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

	if err = db.Delete(); err != nil {
		return errors.Wrap(err, "Delete db failed")
	}
	um.StatusMonitor.Send(UpdateStatusMsg{Name: filePath, Status: UploadCompleted})

	return nil
}

// uploadProgressReader is a reader that sends progress updates to a channel.
type uploadProgressReader struct {
	absPath            string
	total              int64
	uploaded           int64
	uploadProgressChan chan UpdateStatusMsg
}

func (r *uploadProgressReader) Read(b []byte) (int, error) {
	n := int64(len(b))
	r.uploaded += n

	updateMsg := UpdateStatusMsg{Name: r.absPath, Uploaded: n}
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

// uploadProgressSectionReader is a SectionReader that also sends progress updates to a channel.
type uploadProgressSectionReader struct {
	*io.SectionReader
	uploadProgressChan chan UpdateStatusMsg
	absPath            string
}

func (r *uploadProgressSectionReader) Read(b []byte) (int, error) {
	n, err := r.SectionReader.Read(b)
	r.uploadProgressChan <- UpdateStatusMsg{Name: r.absPath, Uploaded: int64(n), Status: UploadInProgress}
	return n, err
}
