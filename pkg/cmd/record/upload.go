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
	"path/filepath"
	"time"

	"github.com/coscene-io/cocli/internal/config"
	"github.com/coscene-io/cocli/pkg/cmd_utils/upload_utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewUploadCommand(cfgPath *string) *cobra.Command {
	var (
		isRecursive       = false
		includeHidden     = false
		projectSlug       = ""
		uploadManagerOpts = &upload_utils.UploadManagerOpts{}
		timeout           time.Duration
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

			fmt.Println("-------------------------------------------------------------")
			fmt.Printf("Uploading files to record: %s\n", recordName.RecordID)

			// create minio client and upload manager first.
			um, err := upload_utils.NewUploadManagerFromConfig(proj, timeout,
				&upload_utils.ApiOpts{SecurityTokenInterface: pm.SecurityTokenCli(), FileInterface: pm.FileCli()}, uploadManagerOpts)
			if err != nil {
				log.Fatalf("unable to create upload manager: %v", err)
			}

			// Upload files
			if err := um.Run(cmd.Context(), recordName, &upload_utils.FileOpts{Path: filePath, Recursive: isRecursive, IncludeHidden: includeHidden}); err != nil {
				log.Fatalf("Unable to upload files: %v", err)
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
	cmd.Flags().IntVarP(&uploadManagerOpts.Threads, "parallel", "P", 4, "number of uploads (could be part) in parallel")
	cmd.Flags().StringVarP(&uploadManagerOpts.PartSize, "part-size", "s", "128Mib", "each part size")
	cmd.Flags().DurationVar(&timeout, "response-timeout", 5*time.Minute, "server response time out")

	return cmd
}
