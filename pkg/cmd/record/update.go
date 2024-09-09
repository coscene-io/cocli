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

	openv1alpha1resource "buf.build/gen/go/coscene-io/coscene-openapi/protocolbuffers/go/coscene/openapi/dataplatform/v1alpha1/resources"
	"connectrpc.com/connect"
	"github.com/coscene-io/cocli/internal/config"
	"github.com/coscene-io/cocli/internal/utils"
	mapset "github.com/deckarep/golang-set/v2"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewUpdateCommand(cfgPath *string) *cobra.Command {
	var (
		title           = ""
		description     = ""
		updateLabelStrs []string
		appendLabelStrs []string
		deleteLabelStrs []string
		projectSlug     = ""
	)

	cmd := &cobra.Command{
		Use:                   "update <record-resource-name/id> [-p <working-project-slug>] [-t <title>] [-d <description>] [-l <append-labels>...] [--update-labels <update-labels>...] [--delete-labels <delete-labels>...]",
		Short:                 "Update record.",
		DisableFlagsInUseLine: true,
		Args:                  cobra.ExactArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) {
			if cmd.Flags().Changed("update-labels") && len(updateLabelStrs) == 0 {
				updateLabelStrs = append(updateLabelStrs, "")
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			// Get current profile.
			pm, _ := config.Provide(*cfgPath).GetProfileManager()
			proj, err := pm.ProjectName(cmd.Context(), projectSlug)
			if err != nil {
				log.Fatalf("unable to get project name: %v", err)
			}

			// Handle args and flags.
			recordName, err := pm.RecordCli().RecordId2Name(context.TODO(), args[0], proj)
			if utils.IsConnectErrorWithCode(err, connect.CodeNotFound) {
				fmt.Printf("failed to find record: %s in project: %s\n", args[0], proj)
				return
			} else if err != nil {
				log.Fatalf("unable to get record name from %s: %v", args[0], err)
			}

			labels := make([]*openv1alpha1resource.Label, 0)
			labelSet := mapset.NewSet[string]()
			if len(appendLabelStrs) > 0 || len(deleteLabelStrs) > 0 {
				deleteLabelSet := mapset.NewSet[string]()
				for _, lbl := range deleteLabelStrs {
					deleteLabelSet.Add(lbl)
				}

				// Get record to get labels
				rcd, err := pm.RecordCli().Get(context.TODO(), recordName)
				if err != nil {
					log.Fatalf("Failed to get record: %v", err)
				}

				for _, lbl := range rcd.Labels {
					if deleteLabelSet.Contains(lbl.DisplayName) {
						continue
					}
					labelSet.Add(lbl.DisplayName)
					labels = append(labels, lbl)
				}

				for _, labelStr := range appendLabelStrs {
					if labelSet.Contains(labelStr) {
						continue
					}
					appendLabel, err := pm.LabelCli().GetByDisplayNameOrCreate(context.TODO(), labelStr, recordName.Project())
					if err != nil {
						log.Fatalf("Failed to get or create label %s: %v", labelStr, err)
					}
					labels = append(labels, appendLabel)
				}
			}

			if len(updateLabelStrs) == 1 && updateLabelStrs[0] == "" {
				// Clear all labels
				labels = make([]*openv1alpha1resource.Label, 0)
			} else {
				for _, lbl := range updateLabelStrs {
					updateLabel, err := pm.LabelCli().GetByDisplayNameOrCreate(context.TODO(), lbl, recordName.Project())
					if err != nil {
						log.Fatalf("Failed to get or create label %s: %v", lbl, err)
					}
					labels = append(labels, updateLabel)
				}
			}

			// Create field mask
			var paths []string
			if title != "" {
				paths = append(paths, "title")
			}
			if description != "" {
				paths = append(paths, "description")
			}
			if len(appendLabelStrs) > 0 || len(updateLabelStrs) > 0 || len(deleteLabelStrs) > 0 {
				paths = append(paths, "labels")
			}

			// Update record.
			err = pm.RecordCli().Update(context.TODO(), recordName, title, description, labels, paths)
			if err != nil {
				log.Fatalf("Failed to update record: %v", err)
			}

			fmt.Printf("Successfully updated record %s\n", recordName)
		},
	}

	cmd.Flags().StringVarP(&title, "title", "t", "", "title of the record.")
	cmd.Flags().StringVarP(&description, "description", "d", "", "description of the record.")
	cmd.Flags().StringSliceVar(&updateLabelStrs, "update-labels", []string{}, "update labels of the record. if contains only one empty string, clear all labels.")
	cmd.Flags().StringSliceVar(&deleteLabelStrs, "delete-labels", []string{}, "delete labels from the record.")
	cmd.Flags().StringSliceVarP(&appendLabelStrs, "append-labels", "l", []string{}, "append labels to the record.")
	cmd.Flags().StringVarP(&projectSlug, "project", "p", "", "the slug of the working project")

	cmd.MarkFlagsMutuallyExclusive("append-labels", "update-labels", "delete-labels")

	return cmd
}
