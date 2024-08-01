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

	"github.com/coscene-io/cocli/api"
	"github.com/coscene-io/cocli/internal/config"
	"github.com/coscene-io/cocli/internal/printer"
	"github.com/coscene-io/cocli/internal/printer/printable"
	"github.com/coscene-io/cocli/internal/printer/table"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewListCommand(cfgPath *string) *cobra.Command {
	var (
		projectSlug    = ""
		verbose        = false
		includeArchive = false
		outputFormat   = ""
	)

	cmd := &cobra.Command{
		Use:                   "list [-v] [-p <working-project-slug>] [--include-archive]",
		Short:                 "List records in the project.",
		DisableFlagsInUseLine: true,
		Args:                  cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			// Get current profile.
			pm, _ := config.Provide(*cfgPath).GetProfileManager()
			proj, err := pm.ProjectName(cmd.Context(), projectSlug)
			if err != nil {
				log.Fatalf("unable to get project name: %v", err)
			}

			// List records in project.
			records, err := pm.RecordCli().ListAll(context.TODO(), &api.ListRecordsOptions{
				Project:        proj,
				IncludeArchive: includeArchive,
			})
			if err != nil {
				log.Fatalf("unable to list records: %v", err)
			}

			// Print listed records.
			var omitFields []string
			if !includeArchive {
				omitFields = append(omitFields, "ARCHIVED")
			}
			err = printer.Printer(outputFormat, &printer.Options{TableOpts: &table.PrintOpts{
				Verbose:    verbose,
				OmitFields: omitFields,
			}}).PrintObj(printable.NewRecord(records), os.Stdout)
			if err != nil {
				log.Fatalf("unable to print records: %v", err)
			}
		},
	}

	cmd.Flags().StringVarP(&projectSlug, "project", "p", "", "the slug of the working project")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	cmd.Flags().BoolVar(&includeArchive, "include-archive", false, "include archived records")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "output format (table|json)")

	return cmd
}
