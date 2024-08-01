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

	"github.com/coscene-io/cocli/internal/config"
	"github.com/coscene-io/cocli/internal/name"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewCopyCommand(cfgPath *string) *cobra.Command {
	var (
		projectSlug = ""
		dstProject  = ""
		dstRecord   = ""
	)

	cmd := &cobra.Command{
		Use:                   "copy <record-resource-name/id> [-p <working-project-slug>] [-P <dst-project-slug>] [-R <dst-record-name/id>]",
		Short:                 "Copy a record to target project/record",
		DisableFlagsInUseLine: true,
		Args:                  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Get current profile.
			pm, _ := config.Provide(*cfgPath).GetProfileManager()
			proj, err := pm.ProjectName(context.TODO(), projectSlug)
			if err != nil {
				log.Fatalf("unable to get project name: %v", err)
			}

			// Handle args and flags.
			recordName, err := pm.RecordCli().RecordId2Name(context.TODO(), args[0], proj)
			if err != nil {
				log.Fatalf("unable to get record name from %s: %v", args[0], err)
			}
			var (
				dstProjectName *name.Project
				dstRecordName  *name.Record
			)
			if len(dstProject) != 0 {
				dstProjectName, err = pm.ProjectName(context.TODO(), dstProject)
				if err != nil {
					log.Fatalf("unable to get project name: %v", err)
				}
			}
			if len(dstRecord) != 0 {
				dstRecordName, err = pm.RecordCli().RecordId2Name(context.TODO(), dstRecord, proj)
				if err != nil {
					log.Fatalf("unable to get record name from %s: %v", dstRecord, err)
				}
			}

			// Copy record.
			var copiedRecordName *name.Record
			if len(dstProject) != 0 {
				copied, err := pm.RecordCli().Copy(context.TODO(), recordName, dstProjectName)
				if err != nil {
					log.Fatalf("failed to copy record: %v", err)
				}

				fmt.Printf("Record successfully copied to %s.\n", copied.Name)
				copiedRecordName, _ = name.NewRecord(copied.Name)
			}

			if len(dstRecord) != 0 {
				filesToCopy, err := pm.RecordCli().ListAllFiles(context.TODO(), recordName)
				if err != nil {
					log.Fatalf("failed to list record files: %v", err)
				}
				err = pm.RecordCli().CopyFiles(context.TODO(), recordName, dstRecordName, filesToCopy)
				if err != nil {
					log.Fatalf("failed to copy record files: %v", err)
				}

				fmt.Printf("Record files successfully copied to %s.\n", dstRecordName.String())
				copiedRecordName = dstRecordName
			}

			copiedRecordUrl, err := pm.GetRecordUrl(copiedRecordName)
			if err != nil {
				log.Errorf("unable to get record url: %v", err)
			} else {
				fmt.Println("The copied record url is:", copiedRecordUrl)
			}
		},
	}

	cmd.Flags().StringVarP(&projectSlug, "project", "p", "", "the slug of the working project")
	cmd.Flags().StringVarP(&dstProject, "dst-project", "P", dstProject, "Destination project slug")
	cmd.Flags().StringVarP(&dstRecord, "dst-record", "R", dstRecord, "Destination record name")

	cmd.MarkFlagsMutuallyExclusive("dst-project", "dst-record")

	return cmd
}
