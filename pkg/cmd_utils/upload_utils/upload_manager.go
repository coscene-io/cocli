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
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	openv1alpha1resource "buf.build/gen/go/coscene-io/coscene-openapi/protocolbuffers/go/coscene/openapi/dataplatform/v1alpha1/resources"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/coscene-io/cocli/internal/constants"
	"github.com/coscene-io/cocli/internal/fs"
	"github.com/coscene-io/cocli/internal/name"
	"github.com/coscene-io/cocli/pkg/cmd_utils"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
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

// FileInfo contains the path, size and sha256 of a file.
type FileInfo struct {
	Path     string
	Size     int64
	Sha256   string
	Uploaded int64
	Status   UploadStatusEnum
}

// UploadManager is a manager for uploading files through minio client.
// Note that it's user's responsibility to check the Errs field after Wait() to see if there's any error.
type UploadManager struct {
	opts    *MultipartOpts
	apiOpts *ApiOpts
	client  *minio.Client

	statusMonitorDoneSignal *sync.WaitGroup
	statusMonitor           *tea.Program
	fileInfos               map[string]*FileInfo
	fileList                *[]string // maintain the order of files

	isDebug bool
	Errs    map[string]error
	sync.WaitGroup
}

func NewUploadManagerFromConfig(proj *name.Project, timeout time.Duration, hideMonitor bool, apiOpts *ApiOpts, multiOpts *MultipartOpts) (*UploadManager, error) {
	if err := multiOpts.Valid(); err != nil {
		return nil, errors.Wrap(err, "invalid multipart options")
	}
	generateSecurityTokenRes, err := apiOpts.GenerateSecurityToken(context.Background(), proj.String())
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
	um := &UploadManager{
		opts:                    multiOpts,
		apiOpts:                 apiOpts,
		client:                  mc,
		statusMonitorDoneSignal: new(sync.WaitGroup),
		isDebug:                 log.GetLevel() == log.DebugLevel,
		fileInfos:               make(map[string]*FileInfo),
		fileList:                new([]string),
		Errs:                    make(map[string]error),
	}

	// statusMonitorStartSignal is to ensure status monitor is ready before sending messages.
	um.statusMonitorDoneSignal.Add(1)
	um.statusMonitor = tea.NewProgram(NewUploadStatusMonitor(um.fileInfos, um.fileList, hideMonitor))
	go um.runUploadStatusMonitor()

	return um, nil
}

func (um *UploadManager) Run(ctx context.Context, rcd *name.Record, fileOpts *FileOpts) error {
	if err := fileOpts.Valid(); err != nil {
		return err
	}

	files := fs.GenerateFiles(fileOpts.Path, fileOpts.Recursive, fileOpts.IncludeHidden)
	fileUploadUrlBatches := um.generateUploadUrlBatches(files, rcd, fileOpts.relDir)

	for fileUploadUrls := range fileUploadUrlBatches {
		for fileResourceName, uploadUrl := range fileUploadUrls {
			fileResource, err := name.NewFile(fileResourceName)
			if err != nil {
				um.AddErr(fileResourceName, errors.Wrapf(err, "unable to parse file resource name"))
				continue
			}

			fileAbsolutePath := filepath.Join(fileOpts.relDir, fileResource.Filename)

			if err = um.UploadFileThroughUrl(fileAbsolutePath, uploadUrl); err != nil {
				um.AddErr(fileAbsolutePath, errors.Wrapf(err, "unable to upload file"))
				continue
			}
		}
	}

	um.Wait()
	return nil
}

// AddFile adds a file to the upload manager.
func (um *UploadManager) AddFile(path string) {
	*um.fileList = append(*um.fileList, path)
	um.fileInfos[path] = &FileInfo{
		Path: path,
	}
}

func (um *UploadManager) Debugf(format string, args ...interface{}) {
	if um.isDebug {
		msg := fmt.Sprintf(format, args...)
		um.statusMonitor.Printf("DEBUG: %s\n", msg)
	}
}

func (um *UploadManager) runUploadStatusMonitor() {
	defer um.statusMonitorDoneSignal.Done()
	finalModel, err := um.statusMonitor.Run()
	if err != nil {
		log.Fatalf("Error running upload status monitor: %v", err)
	}
	um.PrintErrs()
	if q, ok := finalModel.(manualQuit); ok && q.Quit() {
		log.Fatalf("Upload quit manually")
	}
}

// Wait waits for all uploads to finish. And wait for status monitor to finish.
func (um *UploadManager) Wait() {
	um.WaitGroup.Wait()
	time.Sleep(1 * time.Second) // Buffer time for status monitor to finish receiving messages.
	um.statusMonitor.Quit()
	um.statusMonitorDoneSignal.Wait()
}

// AddErr adds an error to the manager.
func (um *UploadManager) AddErr(path string, err error) {
	um.fileInfos[path].Status = UploadFailed
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
	fileInfo, ok := um.fileInfos[absPath]
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
				absPath:  absPath,
				fileInfo: fileInfo,
			}
			um.fileInfos[absPath].Status = UploadInProgress
			_, err = um.client.FPutObject(context.Background(), bucket, key, absPath,
				minio.PutObjectOptions{Progress: progress, UserTags: userTags, DisableMultipart: true})
		}
		if err != nil {
			um.AddErr(absPath, err)
		} else {
			um.fileInfos[absPath].Status = UploadCompleted
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
		result, err := c.ListObjectParts(ctx, bucket, key, uploadId, 0, 2000)
		if err != nil || len(result.ObjectParts) == 0 {
			um.Debugf("List object parts by: %s failed: %v", uploadIdKey, err)
			uploadId = ""
			if err = db.Reset(); err != nil {
				return errors.Wrap(err, "Reset db failed")
			}
		} else {
			um.Debugf("Upload id: %s is still valid", uploadId)
		}
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

	um.fileInfos[filePath].Uploaded = uploadedSize
	um.fileInfos[filePath].Status = UploadInProgress

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
			// Upload parts in window.
			for curPart <= totalPartsCount && curPart < minPart+windowSize/int(partSize) {
				if !slices.Contains(partNumbers, curPart) {
					um.Debugf("sending part to be uploaded: %d", curPart)
					uploadingParts.Push(curPart)
					uploadPartsCh <- curPart
				}
				curPart++
			}

			// Wait for a part to complete.
			select {
			case <-ctx.Done():
				return
			case partNumber := <-completedPartsCh:
				uploadingParts.Remove(partNumber)
				if uploadingParts.Len() == 0 {
					// Handle the case when partNumber is the last part.
					// In this case, it means that all other parts in the window are uploaded.
					// We thus need to update the minPart to the immediate next part outside the window.
					minPart = partNumber + windowSize/int(partSize)
				} else {
					minPart = uploadingParts.Peek()
				}
				um.Debugf("completed part received: %d", partNumber)
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
					SectionReader: io.NewSectionReader(fileReader, readOffset, curPartSize),
					fileInfo:      um.fileInfos[filePath],
					absPath:       filePath,
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

	um.fileInfos[filePath].Status = MultipartCompletionInProgress

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

	return nil
}

const (
	processBatchSize = 20
)

func (um *UploadManager) generateUploadUrlBatches(filesGenerator <-chan string, recordName *name.Record, relativeDir string) <-chan map[string]string {
	ret := make(chan map[string]string)
	go func() {
		defer close(ret)
		var files []*openv1alpha1resource.File
		for f := range filesGenerator {
			um.AddFile(f)
			checksum, size, err := fs.CalSha256AndSize(f)
			if err != nil {
				um.AddErr(f, errors.Wrapf(err, "unable to calculate sha256 for file"))
				continue
			}
			um.fileInfos[f].Size = size
			um.fileInfos[f].Sha256 = checksum

			relativePath, err := filepath.Rel(relativeDir, f)
			if err != nil {
				um.AddErr(f, errors.Wrapf(err, "unable to get relative path"))
				continue
			}

			// Check if the file already exists in the record.
			getFileRes, err := um.apiOpts.GetFile(context.TODO(), name.File{
				ProjectID: recordName.ProjectID,
				RecordID:  recordName.RecordID,
				Filename:  relativePath,
			}.String())
			if err == nil && getFileRes.Sha256 == checksum && getFileRes.Size == size {
				um.fileInfos[f].Status = PreviouslyUploaded
				continue
			}

			files = append(files, &openv1alpha1resource.File{
				Name: name.File{
					ProjectID: recordName.ProjectID,
					RecordID:  recordName.RecordID,
					Filename:  relativePath,
				}.String(),
				Filename: relativePath,
				Sha256:   checksum,
				Size:     size,
			})

			if len(files) == processBatchSize {
				res, err := um.apiOpts.GenerateFileUploadUrls(context.TODO(), recordName, files)
				if err != nil {
					for _, file := range files {
						um.AddErr(filepath.Join(relativeDir, file.Filename), errors.Wrapf(err, "unable to generate upload urls"))
					}
					continue
				}
				ret <- res
				files = nil
			}
		}

		if len(files) > 0 {
			res, err := um.apiOpts.GenerateFileUploadUrls(context.TODO(), recordName, files)
			if err != nil {
				for _, file := range files {
					um.AddErr(filepath.Join(relativeDir, file.Filename), errors.Wrapf(err, "unable to generate upload urls"))
				}
				return
			}
			ret <- res
		}
	}()

	return ret
}

// uploadProgressReader is a reader that sends progress updates to a channel.
type uploadProgressReader struct {
	absPath  string
	fileInfo *FileInfo
}

func (r *uploadProgressReader) Read(b []byte) (int, error) {
	n := int64(len(b))
	r.fileInfo.Uploaded += n
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
	absPath  string
	fileInfo *FileInfo
}

func (r *uploadProgressSectionReader) Read(b []byte) (int, error) {
	n, err := r.SectionReader.Read(b)
	atomic.AddInt64(&r.fileInfo.Uploaded, int64(n))
	return n, err
}
