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
	"os"

	"github.com/coscene-io/cocli/internal/config"
	"github.com/coscene-io/cocli/internal/printer"
	"github.com/coscene-io/cocli/internal/printer/printable"
	"github.com/coscene-io/cocli/internal/printer/table"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewListMomentsCommand(cfgPath *string) *cobra.Command {
	var (
		verbose      = false
		outputFormat = ""
		projectSlug  = ""
	)

	cmd := &cobra.Command{
		Use:                   "list-moments <record-resource-name/id> [-v] [-p <working-project-slug>]",
		Short:                 "List moments in the record",
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

			// List moments in record.
			moments, err := pm.RecordCli().ListAllEvents(cmd.Context(), recordName)
			if err != nil {
				log.Fatalf("unable to list moments: %v", err)
			}

			if err = printer.Printer(outputFormat, &printer.Options{TableOpts: &table.PrintOpts{
				Verbose: verbose,
			}}).PrintObj(printable.NewEvent(moments), os.Stdout); err != nil {
				log.Fatalf("unable to print projects: %v", err)
			}

		},
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "output format (table|json)")
	cmd.Flags().StringVarP(&projectSlug, "project", "p", "", "the slug of the working project")

	_ = cmd.MarkFlagRequired("record")

	return cmd
}
