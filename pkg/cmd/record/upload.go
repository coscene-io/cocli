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

	openv1alpha1resource "buf.build/gen/go/coscene-io/coscene-openapi/protocolbuffers/go/coscene/openapi/dataplatform/v1alpha1/resources"
	"github.com/coscene-io/cocli/api"
	"github.com/coscene-io/cocli/internal/config"
	"github.com/coscene-io/cocli/internal/fs"
	"github.com/coscene-io/cocli/internal/name"
	"github.com/coscene-io/cocli/pkg/cmd_utils"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
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
			generateSecurityTokenRes, err := pm.SecurityTokenCli().GenerateSecurityToken(context.Background(), proj.String())
			if err != nil {
				log.Fatalf("unable to generate security token: %v", err)
			}
			mc, err := minio.New(generateSecurityTokenRes.Endpoint, &minio.Options{
				Creds:  credentials.NewStaticV4(generateSecurityTokenRes.GetAccessKeyId(), generateSecurityTokenRes.GetAccessKeySecret(), generateSecurityTokenRes.GetSessionToken()),
				Secure: true,
				Region: "",
			})
			if err != nil {
				log.Fatalf("unable to create minio client: %v", err)
			}
			um, err := cmd_utils.NewUploadManager(mc)
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
						log.Errorf("Unable to parse %v as file resource name", fileResourceName)
						continue
					}

					fileAbsolutePath := path.Join(relativeDir, fileResource.Filename)

					err = cmd_utils.UploadFileThroughUrl(um, fileAbsolutePath, uploadUrl)
					if err != nil {
						log.Errorf("Unable to upload file %v, error: %+v", fileAbsolutePath, err)
						continue
					}
				}
			}

			um.Wait()
			if um.Errs != nil {
				fmt.Printf("\n%d files failed to upload\n", len(um.Errs))
				for _, uploadErr := range um.Errs {
					fmt.Printf("Upload %v failed with: %v\n", uploadErr.Path, uploadErr.Err)
				}
				return
			}

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

	return cmd
}

func generateUploadUrlBatches(fileClient api.FileInterface, filesGenerator <-chan string, recordName *name.Record, relativeDir string, um *cmd_utils.UploadManager) <-chan map[string]string {
	ret := make(chan map[string]string)
	go func() {
		defer close(ret)
		var files []*openv1alpha1resource.File
		for f := range filesGenerator {
			checksum, size, err := fs.CalSha256AndSize(f)
			if err != nil {
				log.Errorf("unable to calculate sha256 for file: %v", err)
				continue
			}

			relativePath, err := filepath.Rel(relativeDir, f)
			if err != nil {
				log.Errorf("unable to get relative path: %v", err)
				continue
			}

			// Check if the file already exists in the record.
			getFileRes, err := fileClient.GetFile(context.TODO(), name.File{
				ProjectID: recordName.ProjectID,
				RecordID:  recordName.RecordID,
				Filename:  relativePath,
			}.String())
			if err == nil && getFileRes.Sha256 == checksum && getFileRes.Size == size {
				um.AddUploadedFile(f)
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
					log.Errorf("Failed to generate upload urls: %v", err)
					continue
				}
				ret <- res
				files = nil
			}
		}

		if len(files) > 0 {
			res, err := fileClient.GenerateFileUploadUrls(context.TODO(), recordName, files)
			if err != nil {
				log.Errorf("Failed to generate upload urls: %v", err)
				return
			}
			ret <- res
		}
	}()

	return ret

}
