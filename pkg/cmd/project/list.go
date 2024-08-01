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

package project

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
		verbose      = false
		outputFormat = ""
	)

	cmd := &cobra.Command{
		Use:                   "list [-v]",
		Short:                 "List projects in the current organization",
		DisableFlagsInUseLine: true,
		Args:                  cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			// Get current profile.
			pm, _ := config.Provide(*cfgPath).GetProfileManager()

			// List records in project.
			projects, err := pm.ProjectCli().ListAllUserProjects(context.Background(), &api.ListProjectsOptions{})
			if err != nil {
				log.Fatalf("unable to list projects: %v", err)
			}

			// Print listed projects.
			err = printer.Printer(outputFormat, &printer.Options{TableOpts: &table.PrintOpts{
				Verbose: verbose,
			}}).PrintObj(printable.NewProject(projects), os.Stdout)
			if err != nil {
				log.Fatalf("unable to print projects: %v", err)
			}
		},
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "output format (table|json)")

	return cmd
}
