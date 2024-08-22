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

package record

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	openv1alpha1resource "buf.build/gen/go/coscene-io/coscene-openapi/protocolbuffers/go/coscene/openapi/dataplatform/v1alpha1/resources"
	"github.com/coscene-io/cocli/api"
	"github.com/coscene-io/cocli/internal/config"
	"github.com/coscene-io/cocli/internal/fs"
	"github.com/coscene-io/cocli/internal/name"
	"github.com/coscene-io/cocli/pkg/cmd_utils/upload_utils"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	processBatchSize = 20
)

func NewUploadCommand(cfgPath *string) *cobra.Command {
	var (
		isRecursive   = false
		includeHidden = false
		projectSlug   = ""
		multiOpts     = &upload_utils.MultipartOpts{}
		timeout       time.Duration
	)

	cmd := &cobra.Command{
		Use:                   "upload <record-resource-name/id> <directory> [-p <working-project-slug>] [-R] [-H]",
		Short:                 "Upload files in directory to a record in coScene.",
		DisableFlagsInUseLine: true,
		Args:                  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			// Get current profile.
			pm, _ := config.Provide(*cfgPath).GetProfileManager()
			proj, err := pm.ProjectName(cmd.Context(), projectSlug)
			if err != nil {
				log.Fatalf("unable to get project name: %v", err)
			}

			// Handle args and flags.
			recordName, err := pm.RecordCli().RecordId2Name(context.TODO(), args[0], proj)
			if err != nil {
				log.Fatalf("unable to get record name from %s: %v", args[0], err)
			}

			filePath, err := filepath.Abs(args[1])
			if err != nil {
				log.Fatalf("unable to get absolute path: %v", err)
			}
			relativeDir := filePath
			fileInfo, err := os.Stat(filePath)
			if err != nil {
				log.Fatalf("invalid file path: %s", filePath)
			}
			if !fileInfo.IsDir() {
				relativeDir = filepath.Dir(filePath)
			}

			fmt.Println("-------------------------------------------------------------")
			fmt.Printf("Uploading files to record: %s\n", recordName.RecordID)

			// create minio client and upload manager first.
			um, err := upload_utils.NewUploadManagerFromConfig(pm, proj, timeout, multiOpts)
			if err != nil {
				log.Fatalf("unable to create upload manager: %v", err)
			}

			// Upload files
			files := fs.GenerateFiles(filePath, isRecursive, includeHidden)
			fileUploadUrlBatches := generateUploadUrlBatches(pm.FileCli(), files, recordName, relativeDir, um)

			for fileUploadUrls := range fileUploadUrlBatches {
				for fileResourceName, uploadUrl := range fileUploadUrls {
					fileResource, err := name.NewFile(fileResourceName)
					if err != nil {
						um.AddErr(fileResourceName, errors.Wrapf(err, "unable to parse file resource name"))
						continue
					}

					fileAbsolutePath := path.Join(relativeDir, fileResource.Filename)

					if err = um.UploadFileThroughUrl(fileAbsolutePath, uploadUrl); err != nil {
						um.AddErr(fileAbsolutePath, errors.Wrapf(err, "unable to upload file"))
						continue
					}
				}
			}

			um.Wait()

			recordUrl, err := pm.GetRecordUrl(recordName)
			if err == nil {
				fmt.Println("View record at:", recordUrl)
			} else {
				log.Errorf("unable to get record url: %v", err)
			}
		},
	}

	cmd.Flags().BoolVarP(&isRecursive, "recursive", "R", false, "upload files in the current directory recursively")
	cmd.Flags().BoolVarP(&includeHidden, "include-hidden", "H", false, "include hidden files (\"dot\" files) in the upload")
	cmd.Flags().StringVarP(&projectSlug, "project", "p", "", "the slug of the working project")
	cmd.Flags().UintVarP(&multiOpts.Threads, "parallel", "P", 4, "upload number of parts in parallel")
	cmd.Flags().StringVarP(&multiOpts.Size, "part-size", "s", "128Mib", "each part size")
	cmd.Flags().DurationVar(&timeout, "response-timeout", 5*time.Minute, "server response time out")

	return cmd
}

func generateUploadUrlBatches(fileClient api.FileInterface, filesGenerator <-chan string, recordName *name.Record, relativeDir string, um *upload_utils.UploadManager) <-chan map[string]string {
	ret := make(chan map[string]string)
	go func() {
		defer close(ret)
		var files []*openv1alpha1resource.File
		for f := range filesGenerator {
			um.StatusMonitor.Send(upload_utils.AddFileMsg{
				Name: f,
			})
			checksum, size, err := fs.CalSha256AndSize(f)
			if err != nil {
				um.AddErr(f, errors.Wrapf(err, "unable to calculate sha256 for file"))
				continue
			}
			um.FileInfos[f] = upload_utils.FileInfo{
				Path:   f,
				Size:   size,
				Sha256: checksum,
			}
			um.StatusMonitor.Send(upload_utils.UpdateStatusMsg{
				Name:  f,
				Total: size,
			})

			relativePath, err := filepath.Rel(relativeDir, f)
			if err != nil {
				um.AddErr(f, errors.Wrapf(err, "unable to get relative path"))
				continue
			}

			// Check if the file already exists in the record.
			getFileRes, err := fileClient.GetFile(context.TODO(), name.File{
				ProjectID: recordName.ProjectID,
				RecordID:  recordName.RecordID,
				Filename:  relativePath,
			}.String())
			if err == nil && getFileRes.Sha256 == checksum && getFileRes.Size == size {
				um.StatusMonitor.Send(upload_utils.UpdateStatusMsg{
					Name:   f,
					Status: upload_utils.PreviouslyUploaded,
				})
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
				res, err := fileClient.GenerateFileUploadUrls(context.TODO(), recordName, files)
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
			res, err := fileClient.GenerateFileUploadUrls(context.TODO(), recordName, files)
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
