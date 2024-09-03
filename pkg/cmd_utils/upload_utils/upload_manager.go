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
	"github.com/muesli/reflow/wordwrap"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
	"golang.org/x/exp/slices"
)

const (
	userTagRecordIdKey     = "X-COS-RECORD-ID"
	mutipartUploadInfoKey  = "STORE-KEY-MUTIPART-UPLOAD-INFO"
	maxSinglePutObjectSize = 1024 * 1024 * 1024 * 500 // 500GiB
	defaultWindowSize      = 1024 * 1024 * 1024       // 1GiB
	processBatchSize       = 20
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

// UploadInfo contains the information needed to upload a file or a file part (multipart upload).
type UploadInfo struct {
	Path   string
	Bucket string
	Key    string
	Tags   map[string]string

	// Upload result infos
	Result minio.ObjectPart
	Err    error

	// Multipart info
	UploadId        string
	PartId          int
	TotalPartsCount int
	ReadOffset      int64
	ReadSize        int64
	FileReader      *os.File
	DB              *UploadDB
}

// MultipartCheckpointInfo contains the information needed to resume a multipart upload.
type MultipartCheckpointInfo struct {
	UploadId     string               `json:"upload_id"`
	UploadedSize int64                `json:"uploaded_size"`
	Parts        []minio.CompletePart `json:"parts"`
}

// UploadManager is a manager for uploading files through minio client.
type UploadManager struct {
	// client and opts
	opts    *MultipartOpts
	apiOpts *ApiOpts
	client  *minio.Client

	// file status related
	fileInfos map[string]*FileInfo
	fileList  []string
	uploadWg  sync.WaitGroup

	// Monitor related
	windowWidth int
	manualQuit  bool
	monitor     *tea.Program

	// other
	errs    map[string]error
	isDebug bool
}

func NewUploadManagerFromConfig(proj *name.Project, timeout time.Duration, apiOpts *ApiOpts, multiOpts *MultipartOpts) (*UploadManager, error) {
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
		opts:      multiOpts,
		apiOpts:   apiOpts,
		client:    mc,
		isDebug:   log.GetLevel() == log.DebugLevel,
		fileInfos: make(map[string]*FileInfo),
		fileList:  []string{},
		errs:      make(map[string]error),
	}

	return um, nil
}

// Run is used to start the upload process.
func (um *UploadManager) Run(ctx context.Context, rcd *name.Record, fileOpts *FileOpts) error {
	if err := fileOpts.Valid(); err != nil {
		return err
	}

	um.monitor = tea.NewProgram(um)
	var monitorCompleteWg sync.WaitGroup
	monitorCompleteWg.Add(1)
	go func() {
		defer monitorCompleteWg.Done()

		_, err := um.monitor.Run()
		if err != nil {
			log.Fatalf("Error running upload status monitor: %v", err)
		}

		um.printErrs()
		if um.manualQuit {
			log.Fatalf("Upload quit manually")
		}
	}()

	// Send an empty message to wait for the monitor to start
	um.monitor.Send(struct{}{})
	um.client.TraceOn(log.StandardLogger().WriterLevel(log.TraceLevel))

	filesToUpload := fs.FindFiles(fileOpts.Path, fileOpts.Recursive, fileOpts.IncludeHidden)
	um.uploadWg.Add(len(filesToUpload) + len(fileOpts.AdditionalUploads))

	fileToUploadUrls := um.findAllUploadUrls(filesToUpload, rcd, fileOpts.relDir)
	for f, v := range fileOpts.AdditionalUploads {
		fileToUploadUrls[f] = v
		um.addFile(f)
		checksum, size, err := fs.CalSha256AndSize(f)
		if err != nil {
			um.addErr(f, errors.Wrapf(err, "unable to calculate sha256 for file"))
			continue
		}
		um.fileInfos[f].Size = size
		um.fileInfos[f].Sha256 = checksum
	}

	// Declare a channel that sends the next upload info to be processed
	uploadCh := make(chan UploadInfo)
	// Declare a channel that receives the result of the upload
	uploadResultCh := make(chan UploadInfo)

	// Start the upload workers
	for i := 0; i < um.opts.Threads; i++ {
		go func() {
			for {
				uploadInfo, ok := <-uploadCh
				um.debugF("Worker %d received upload task with path: %s, part id: %d", i, uploadInfo.Path, uploadInfo.PartId)
				if !ok {
					return
				}
				if uploadInfo.UploadId == "" {
					uploadInfo.Err = um.consumeSingleUploadInfo(ctx, uploadInfo)
				} else {
					uploadInfo.Result, uploadInfo.Err = um.consumeMultipartUploadInfo(ctx, uploadInfo)
				}

				uploadResultCh <- uploadInfo
			}
		}()
	}

	uploadInfos := um.produceUploadInfos(ctx, fileToUploadUrls, fileOpts.relDir)
	go um.scheduleUploads(ctx, uploadInfos, uploadCh, uploadResultCh, um.opts.Threads)
	um.uploadWg.Wait()

	um.monitor.Quit()
	monitorCompleteWg.Wait()

	return nil
}

// produceUploadInfos is a producer of upload infos for each file to be uploaded.
func (um *UploadManager) produceUploadInfos(ctx context.Context, fileToUploadUrls map[string]string, relDir string) <-chan UploadInfo {
	ret := make(chan UploadInfo)
	go func() {
		defer close(ret)
		for _, fileAbsolutePath := range um.fileList {
			uploadUrl, ok := fileToUploadUrls[fileAbsolutePath]
			if !ok {
				continue
			}

			bucket, key, tags, err := um.parseUrl(uploadUrl)
			if err != nil {
				um.addErr(fileAbsolutePath, errors.Wrapf(err, "unable to parse upload url"))
				continue
			}

			fileInfo := um.fileInfos[fileAbsolutePath]

			if fileInfo.Size <= int64(um.opts.sizeUint64) {
				ret <- UploadInfo{
					Path:   fileAbsolutePath,
					Bucket: bucket,
					Key:    key,
					Tags:   tags,
				}
			} else {
				multipartUploadInfo, err := um.produceMultipartUploadInfos(ctx, fileAbsolutePath, bucket, key, tags)
				if err != nil {
					um.addErr(fileAbsolutePath, errors.Wrapf(err, "unable to produce multipart upload infos"))
					continue
				}
				for _, info := range multipartUploadInfo {
					ret <- info
				}
			}
		}
	}()

	return ret
}

func (um *UploadManager) produceMultipartUploadInfos(ctx context.Context, fileAbsolutePath string, bucket string, key string, tags map[string]string) (uploadInfos []UploadInfo, err error) {
	fileInfo := um.fileInfos[fileAbsolutePath]

	// Check for largest object size allowed.
	if fileInfo.Size > int64(maxSinglePutObjectSize) {
		return nil, errors.Errorf("Your proposed upload size ‘%d’ exceeds the maximum allowed object size ‘%d’ for single PUT operation.", fileInfo.Size, maxSinglePutObjectSize)
	}

	// Create uploader directory if not exists
	if err = os.MkdirAll(constants.DefaultUploaderDirPath, 0755); err != nil {
		return nil, errors.Wrap(err, "Create uploader directory failed")
	}

	// Create uploader db
	db, err := NewUploadDB(fileAbsolutePath, tags[userTagRecordIdKey], fileInfo.Sha256)
	if err != nil {
		return nil, errors.Wrap(err, "Create uploader db failed")
	}

	c := minio.Core{Client: um.client}
	// ----------------- Start fetching previous upload info from db -----------------
	// Fetch upload id. If not found, initiate a new multipart upload.
	var checkpoint MultipartCheckpointInfo
	if err = db.Get(mutipartUploadInfoKey, &checkpoint); err != nil {
		um.debugF("Get checkpoint failed: %v", err)
		checkpoint = MultipartCheckpointInfo{}
	}

	// Fetch upload id. If not found, initiate a new multipart upload.
	if checkpoint.UploadId != "" {
		um.debugF("Upload id: %s is found in db", checkpoint.UploadId)

		// Check if the upload id is still valid
		result, err := c.ListObjectParts(ctx, bucket, key, checkpoint.UploadId, 0, 2000)
		if err != nil || len(result.ObjectParts) == 0 {
			um.debugF("List object parts by: %s failed: %v", checkpoint.UploadId, err)
			checkpoint.UploadId = ""
			if err = db.Reset(); err != nil {
				return nil, errors.Wrap(err, "Reset db failed")
			}
		} else {
			um.debugF("Upload id: %s is still valid", checkpoint.UploadId)
		}
	}

	if checkpoint.UploadId == "" {
		// first reset checkpoint
		checkpoint = MultipartCheckpointInfo{}
		checkpoint.UploadId, err = c.NewMultipartUpload(ctx, bucket, key, minio.PutObjectOptions{
			UserTags: tags,
			PartSize: um.opts.sizeUint64,
		})
		if err != nil {
			return nil, errors.Wrap(err, "New multipart upload failed")
		}
	}

	partNumbers := lo.Map(checkpoint.Parts, func(p minio.CompletePart, _ int) int {
		return p.PartNumber
	})
	sort.Ints(partNumbers)
	um.debugF("Get upload id: %s", checkpoint.UploadId)
	um.debugF("Get uploaded size: %d", checkpoint.UploadedSize)
	um.debugF("Get uploaded parts: %v", partNumbers)

	// ----------------- End fetching previous upload info from db -----------------
	// Calculate the optimal parts info for a given size.
	totalPartsCount, partSize, lastPartSize, err := minio.OptimalPartInfo(fileInfo.Size, um.opts.sizeUint64)
	if err != nil {
		return nil, errors.Wrap(err, "Optimal part info failed")
	}
	um.debugF("Total part: %v, part size: %v, last part size: %v", totalPartsCount, partSize, lastPartSize)

	// Get reader of the file to be uploaded.
	fileReader, err := os.Open(fileAbsolutePath)
	if err != nil {
		return nil, errors.Wrap(err, "Open file failed")
	}

	// Compute remaining parts to upload.
	for partId := 1; partId <= totalPartsCount; partId++ {
		if slices.Contains(partNumbers, partId) {
			continue
		}

		readSize := partSize
		if partId == totalPartsCount {
			readSize = lastPartSize
		}
		uploadInfos = append(uploadInfos, UploadInfo{
			Path:            fileAbsolutePath,
			Bucket:          bucket,
			Key:             key,
			Tags:            tags,
			UploadId:        checkpoint.UploadId,
			PartId:          partId,
			TotalPartsCount: totalPartsCount,
			ReadOffset:      int64(partId-1) * partSize,
			ReadSize:        readSize,
			FileReader:      fileReader,
			DB:              db,
		})
	}

	um.fileInfos[fileAbsolutePath].Uploaded = checkpoint.UploadedSize
	um.fileInfos[fileAbsolutePath].Status = UploadInProgress
	return uploadInfos, nil
}

func (um *UploadManager) scheduleUploads(ctx context.Context, uploadInfos <-chan UploadInfo, uploadCh chan<- UploadInfo, uploadResultCh <-chan UploadInfo, numThread int) {
	uploadInfoInProgress := make([]UploadInfo, 0)
	var previousUploadInfo *UploadInfo

	for {
		for i := len(uploadInfoInProgress) + 1; i <= numThread; i++ {
			// If there is a previous upload info, try to upload it first.
			if previousUploadInfo != nil {
				if um.canUpload(*previousUploadInfo, uploadInfoInProgress) {
					uploadCh <- *previousUploadInfo
					uploadInfoInProgress = append(uploadInfoInProgress, *previousUploadInfo)
					previousUploadInfo = nil
					continue
				} else {
					break
				}
			}
			if uploadInfo, ok := <-uploadInfos; ok {
				if um.canUpload(uploadInfo, uploadInfoInProgress) {
					uploadCh <- uploadInfo
					uploadInfoInProgress = append(uploadInfoInProgress, uploadInfo)
				} else {
					previousUploadInfo = &uploadInfo
					break
				}
			}
		}

		result := <-uploadResultCh

		uploadInfoInProgress = lo.Filter(uploadInfoInProgress, func(info UploadInfo, _ int) bool {
			return info.Path != result.Path || info.PartId != result.PartId
		})

		if um.fileInfos[result.Path].Status == UploadFailed {
			// Skip if some other part of the same file has failed.
			continue
		}

		if err := um.handleUploadResult(result); err != nil {
			um.debugF("Handle upload result failed: %v", err)

			// todo: retry, abort remaining parts on error, etc.
			um.addErr(result.Path, err)
		}
	}
}

func (um *UploadManager) handleUploadResult(result UploadInfo) error {
	// On error result
	if result.Err != nil {
		return result.Err
	}

	// On success single upload
	if result.UploadId == "" {
		um.fileInfos[result.Path].Status = UploadCompleted
		um.uploadWg.Done()
		return nil
	}

	// On success multipart upload
	var checkpoint MultipartCheckpointInfo
	if err := result.DB.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(multipartUploadsBucket))
		value := bucket.Get([]byte(mutipartUploadInfoKey))

		if value == nil {
			checkpoint = MultipartCheckpointInfo{
				UploadId: result.UploadId,
			}
		} else {
			if err := json.Unmarshal(value, &checkpoint); err != nil {
				return errors.Wrap(err, "unmarshal checkpoint info")
			}
		}

		checkpoint.UploadId = result.UploadId
		checkpoint.UploadedSize += result.ReadSize
		checkpoint.Parts = append(checkpoint.Parts, minio.CompletePart{
			PartNumber:     result.Result.PartNumber,
			ETag:           result.Result.ETag,
			ChecksumCRC32:  result.Result.ChecksumCRC32,
			ChecksumCRC32C: result.Result.ChecksumCRC32C,
			ChecksumSHA1:   result.Result.ChecksumSHA1,
			ChecksumSHA256: result.Result.ChecksumSHA256,
		})
		checkpointBytes, err := json.Marshal(checkpoint)
		if err != nil {
			return errors.Wrap(err, "marshal checkpoint info")
		}

		return bucket.Put([]byte(mutipartUploadInfoKey), checkpointBytes)
	}); err != nil {
		return errors.Wrapf(err, "update checkpoint info failed for %s", result.Path)
	}

	// Check if multipart upload is completed
	if len(checkpoint.Parts) == result.TotalPartsCount {
		defer func(DB *UploadDB) {
			if err := DB.Delete(); err != nil {
				um.debugF("Delete db failed: %v", err)
			}
		}(result.DB)
		defer func(FileReader *os.File) {
			if err := FileReader.Close(); err != nil {
				um.debugF("Close file failed: %v", err)
			}
		}(result.FileReader)

		um.fileInfos[result.Path].Status = MultipartCompletionInProgress

		slices.SortFunc(checkpoint.Parts, func(i, j minio.CompletePart) int {
			return i.PartNumber - j.PartNumber
		})

		opts := minio.PutObjectOptions{
			UserTags: result.Tags,
		}
		if opts.ContentType = mime.TypeByExtension(filepath.Ext(result.Path)); opts.ContentType == "" {
			opts.ContentType = "application/octet-stream"
		}

		_, err := minio.Core{Client: um.client}.CompleteMultipartUpload(context.Background(), result.Bucket, result.Key, result.UploadId, checkpoint.Parts, opts)
		if err != nil {
			return errors.Wrap(err, "complete multipart upload failed")
		}

		um.fileInfos[result.Path].Status = UploadCompleted
		um.uploadWg.Done()
	}

	return nil
}

// canUpload checks if the upload candidate is allowed to upload.
// It basically checks if the upload candidate is a multipart upload and if the part id is within the window size
// of the least in progress upload part id.
func (um *UploadManager) canUpload(uploadCandidate UploadInfo, uploadInfoInProgress []UploadInfo) bool {
	leastUploadingPartId := lo.Min(lo.FilterMap(uploadInfoInProgress, func(info UploadInfo, _ int) (int, bool) {
		if info.Path == uploadCandidate.Path {
			return info.PartId, true
		} else {
			return 0, false
		}
	}))

	if leastUploadingPartId == 0 {
		// Case 1: Single upload, no upload with the same path is in progress.
		// Case 2: Multipart upload, no other parts are in progress.
		// For both cases, we can upload directly.
		return true
	}

	windowSize := defaultWindowSize
	if windowSize < int(um.opts.sizeUint64) {
		windowSize = int(um.opts.sizeUint64)
	}
	threshold := leastUploadingPartId + windowSize/int(um.opts.sizeUint64)

	return uploadCandidate.PartId <= threshold
}

func (um *UploadManager) consumeSingleUploadInfo(ctx context.Context, uploadInfo UploadInfo) error {
	um.fileInfos[uploadInfo.Path].Status = UploadInProgress
	progress := &uploadProgressReader{
		fileInfo: um.fileInfos[uploadInfo.Path],
	}
	if _, err := um.client.FPutObject(ctx, uploadInfo.Bucket, uploadInfo.Key, uploadInfo.Path, minio.PutObjectOptions{
		Progress:         progress,
		UserTags:         uploadInfo.Tags,
		DisableMultipart: true,
	}); err != nil {
		return errors.Wrapf(err, "Put object failed")
	}
	return nil
}

func (um *UploadManager) consumeMultipartUploadInfo(ctx context.Context, uploadInfo UploadInfo) (minio.ObjectPart, error) {
	sectionReader := &uploadProgressSectionReader{
		SectionReader: io.NewSectionReader(uploadInfo.FileReader, uploadInfo.ReadOffset, uploadInfo.ReadSize),
		fileInfo:      um.fileInfos[uploadInfo.Path],
	}
	um.debugF("Uploading part %d of %s", uploadInfo.PartId, uploadInfo.Path)

	objPart, err := minio.Core{Client: um.client}.PutObjectPart(ctx, uploadInfo.Bucket, uploadInfo.Key, uploadInfo.UploadId, uploadInfo.PartId, sectionReader, uploadInfo.ReadSize, minio.PutObjectPartOptions{})
	if err != nil {
		um.debugF("Put object part %d of %s failed: %v", uploadInfo.PartId, uploadInfo.Path, err)
	} else {
		um.debugF("Put object part %d of %s succeeded", uploadInfo.PartId, uploadInfo.Path)
	}
	return objPart, err
}

// parseUrl parses the upload url to get the bucket, key and tags.
func (um *UploadManager) parseUrl(uploadUrl string) (string, string, map[string]string, error) {
	parsedUrl, err := url.Parse(uploadUrl)
	if err != nil {
		return "", "", nil, errors.Wrap(err, "parse upload url failed")
	}

	// Parse tags
	tagsMap, err := url.ParseQuery(parsedUrl.Query().Get("X-Amz-Tagging"))
	if err != nil {
		return "", "", nil, errors.Wrap(err, "parse tags failed")
	}
	tags := lo.MapValues(tagsMap, func(value []string, _ string) string {
		if len(value) == 0 {
			return ""
		}
		return value[0]
	})

	// Parse bucket and key
	pathParts := strings.SplitN(parsedUrl.Path, "/", 3)
	bucket := pathParts[1]
	key := pathParts[2]
	return bucket, key, tags, nil
}

// addFile adds a file to the upload manager.
func (um *UploadManager) addFile(path string) {
	um.fileList = append(um.fileList, path)
	um.fileInfos[path] = &FileInfo{
		Path: path,
	}
}

// debugF is used to print debug messages.
// cannot use logrus here because tea.Program overtakes the log output.
func (um *UploadManager) debugF(format string, args ...interface{}) {
	if um.isDebug {
		msg := fmt.Sprintf(format, args...)
		debugMsg := wordwrap.String(fmt.Sprintf("DEBUG: %s", msg), um.windowWidth)
		um.monitor.Println(debugMsg)
	}
}

// addErr adds an error to the manager.
func (um *UploadManager) addErr(path string, err error) {
	um.debugF("Upload %s failed with: %v", path, err)
	um.fileInfos[path].Status = UploadFailed
	um.errs[path] = err
	um.uploadWg.Done()
}

// printErrs prints all errors.
func (um *UploadManager) printErrs() {
	if len(um.errs) > 0 {
		fmt.Printf("\n%d files failed to upload\n", len(um.errs))
		for kPath, vErr := range um.errs {
			fmt.Printf("Upload %v failed with: \n%v\n\n", kPath, vErr)
		}
		return
	}
}

func (um *UploadManager) findAllUploadUrls(filesToUpload []string, recordName *name.Record, relativeDir string) map[string]string {
	ret := make(map[string]string)
	var files []*openv1alpha1resource.File

	for _, f := range filesToUpload {
		um.addFile(f)
		checksum, size, err := fs.CalSha256AndSize(f)
		if err != nil {
			um.addErr(f, errors.Wrapf(err, "unable to calculate sha256 for file"))
			continue
		}
		um.fileInfos[f].Size = size
		um.fileInfos[f].Sha256 = checksum

		relativePath, err := filepath.Rel(relativeDir, f)
		if err != nil {
			um.addErr(f, errors.Wrapf(err, "unable to get relative path"))
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
			um.uploadWg.Done()
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
			um.debugF("Generating upload urls for %d files", len(files))
			res, err := um.apiOpts.GenerateFileUploadUrls(context.TODO(), recordName, files)
			if err != nil {
				for _, file := range files {
					um.addErr(filepath.Join(relativeDir, file.Filename), errors.Wrapf(err, "unable to generate upload urls"))
				}
				continue
			}
			for k, v := range res {
				fileResource, _ := name.NewFile(k)
				ret[filepath.Join(relativeDir, fileResource.Filename)] = v
			}
			files = nil
		}
	}

	if len(files) > 0 {
		um.debugF("Generating upload urls for %d files", len(files))
		res, err := um.apiOpts.GenerateFileUploadUrls(context.TODO(), recordName, files)
		if err != nil {
			for _, file := range files {
				um.addErr(filepath.Join(relativeDir, file.Filename), errors.Wrapf(err, "unable to generate upload urls"))
			}
		}
		for k, v := range res {
			fileResource, _ := name.NewFile(k)
			ret[filepath.Join(relativeDir, fileResource.Filename)] = v
		}
	}

	return ret
}

// calculateUploadProgress is used to calculate the progress of a file upload
func (um *UploadManager) calculateUploadProgress(name string) float64 {
	status := um.fileInfos[name]
	if status.Size == 0 {
		return 100
	}
	return float64(status.Uploaded) * 100 / float64(status.Size)
}

func (um *UploadManager) Init() tea.Cmd {
	return tick()
}

func (um *UploadManager) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		um.windowWidth = msg.Width
	case tea.QuitMsg:
		return um, tea.Quit
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEscape, tea.KeyCtrlD:
			um.manualQuit = true
			return um, tea.Quit
		}
	case TickMsg:
		return um, tick()
	}
	return um, nil
}

func (um *UploadManager) View() string {
	s := "Upload Status:\n"
	skipCount := 0
	successCount := 0
	for _, k := range um.fileList {
		// Check if the file has been uploaded before
		statusStrLen := um.windowWidth - len(k) - 1
		switch um.fileInfos[k].Status {
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
			progress := um.calculateUploadProgress(k)
			barWidth := max(um.windowWidth-len(k)-12, 10)                       // Adjust for label and percentage, make sure it is at least 10
			progressCount := min(int(progress*float64(barWidth)/100), barWidth) // min used to prevent float rounding errors
			emptyBar := strings.Repeat("-", barWidth-progressCount)
			progressBar := strings.Repeat("█", progressCount)
			s += fmt.Sprintf("%s: [%s%s] %*.2f%%\n", k, progressBar, emptyBar, 6, progress)
		}
	}

	// Add summary of all file status
	s += "\n"
	s += fmt.Sprintf("Total: %d, Skipped: %d, Success: %d", len(um.fileList), skipCount, successCount)
	if successCount+skipCount < len(um.fileList) {
		s += fmt.Sprintf(", Remaining: %d", len(um.fileList)-successCount-skipCount)
	}
	s += "\n"
	s = wordwrap.String(s, um.windowWidth)
	return s
}

// TickMsg is a message that is sent to the update function every 0.5 second.
type TickMsg time.Time

// tick is a command that sends a TickMsg every 0.5 second.
func tick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// uploadProgressReader is a reader that sends progress updates to a channel.
type uploadProgressReader struct {
	fileInfo *FileInfo
}

func (r *uploadProgressReader) Read(b []byte) (int, error) {
	n := int64(len(b))
	r.fileInfo.Uploaded += n
	return int(n), nil
}

// uploadProgressSectionReader is a SectionReader that also sends progress updates to a channel.
type uploadProgressSectionReader struct {
	*io.SectionReader
	fileInfo *FileInfo
}

func (r *uploadProgressSectionReader) Read(b []byte) (int, error) {
	n, err := r.SectionReader.Read(b)
	atomic.AddInt64(&r.fileInfo.Uploaded, int64(n))
	return n, err
}
