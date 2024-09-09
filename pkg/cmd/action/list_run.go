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

package action

import (
	"context"
	"fmt"
	"os"

	openv1alpha1resource "buf.build/gen/go/coscene-io/coscene-openapi/protocolbuffers/go/coscene/openapi/dataplatform/v1alpha1/resources"
	"connectrpc.com/connect"
	"github.com/coscene-io/cocli/api"
	"github.com/coscene-io/cocli/internal/config"
	"github.com/coscene-io/cocli/internal/name"
	"github.com/coscene-io/cocli/internal/printer"
	"github.com/coscene-io/cocli/internal/printer/printable"
	"github.com/coscene-io/cocli/internal/printer/table"
	"github.com/coscene-io/cocli/internal/utils"
	mapset "github.com/deckarep/golang-set/v2"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewListRunCommand(cfgPath *string) *cobra.Command {
	var (
		projectSlug    = ""
		verbose        = false
		recordNameOrId = ""
		outputFormat   = ""
	)

	cmd := &cobra.Command{
		Use:                   "list-run [-v] [-r <record-resource-name/id>] [-p <working-project-slug>]",
		Short:                 "List action-runs in the current project",
		DisableFlagsInUseLine: true,
		Args:                  cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			// Get current profile.
			pm, _ := config.Provide(*cfgPath).GetProfileManager()
			proj, err := pm.ProjectName(cmd.Context(), projectSlug)
			if err != nil {
				log.Fatalf("unable to get project name: %v", err)
			}

			// Handle args and flags.
			listRunOpts := &api.ListActionRunsOptions{
				Parent: proj.String(),
			}
			if recordNameOrId != "" {
				recordName, err := pm.RecordCli().RecordId2Name(context.TODO(), recordNameOrId, proj)
				if utils.IsConnectErrorWithCode(err, connect.CodeNotFound) {
					fmt.Printf("failed to find record: %s in project: %s\n", recordNameOrId, proj)
					return
				} else if err != nil {
					log.Fatalf("unable to get record name from %s: %v", recordNameOrId, err)
				}
				listRunOpts.RecordNames = []*name.Record{recordName}
			}

			// List all actionRuns.
			actionRuns, err := pm.ActionCli().ListAllActionRuns(context.TODO(), listRunOpts)
			if err != nil {
				log.Fatalf("unable to list action runs: %v", err)
			}

			// Convert users to nicknames.
			convertActionRunUsers(actionRuns, pm)

			// Print listed actions.
			err = printer.Printer(outputFormat, &printer.Options{TableOpts: &table.PrintOpts{
				Verbose: verbose,
			}}).PrintObj(printable.NewActionRun(actionRuns), os.Stdout)
			if err != nil {
				log.Fatalf("unable to print action runs: %v", err)
			}
		},
	}

	cmd.Flags().StringVarP(&projectSlug, "project", "p", "", "the slug of the working project")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	cmd.Flags().StringVarP(&recordNameOrId, "record", "r", "", "designated record name or id")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "output format (table|json)")

	return cmd
}

func convertActionRunUsers(actionRuns []*openv1alpha1resource.ActionRun, pm *config.ProfileManager) {
	// Search for all users in actionRuns creators.
	usersSet := mapset.NewSet[name.User]()
	for _, a := range actionRuns {
		switch t := a.Creator.(type) {
		case *openv1alpha1resource.ActionRun_User:
			if userName, err := name.NewUser(t.User); err == nil {
				usersSet.Add(*userName)
			}
		}
	}

	// Batch get users
	usersMap, err := pm.UserCli().BatchGetUsers(context.TODO(), usersSet)
	if err != nil {
		log.Fatalf("unable to batch get users: %v", err)
	}

	// Convert users to nicknames
	for _, a := range actionRuns {
		switch t := a.Creator.(type) {
		case *openv1alpha1resource.ActionRun_User:
			if _, err := name.NewUser(t.User); err == nil {
				if u, ok := usersMap[t.User]; ok {
					a.Creator = &openv1alpha1resource.ActionRun_User{
						User: *u.Nickname,
					}
				}
			}
		}
	}
}
