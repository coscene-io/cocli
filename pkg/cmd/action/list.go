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
	"os"

	openv1alpha1resource "buf.build/gen/go/coscene-io/coscene-openapi/protocolbuffers/go/coscene/openapi/dataplatform/v1alpha1/resources"
	"github.com/coscene-io/cocli/api"
	"github.com/coscene-io/cocli/internal/config"
	"github.com/coscene-io/cocli/internal/name"
	"github.com/coscene-io/cocli/internal/printer"
	"github.com/coscene-io/cocli/internal/printer/printable"
	"github.com/coscene-io/cocli/internal/printer/table"
	mapset "github.com/deckarep/golang-set/v2"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewListCommand(cfgPath *string) *cobra.Command {
	var (
		projectSlug  = ""
		verbose      = false
		outputFormat = ""
	)

	cmd := &cobra.Command{
		Use:                   "list [-v] [-p <working-project-slug>]",
		Short:                 "List actions in the current project",
		DisableFlagsInUseLine: true,
		Args:                  cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			// Get current profile.
			pm, _ := config.Provide(*cfgPath).GetProfileManager()
			proj, err := pm.ProjectName(cmd.Context(), projectSlug)
			if err != nil {
				log.Fatalf("unable to get project name: %v", err)
			}

			// List all actions.
			actions, err := pm.ActionCli().ListAllActions(context.TODO(), &api.ListActionsOptions{
				Parent: proj.String(),
			})
			if err != nil {
				log.Fatalf("unable to list actions: %v", err)
			}

			systemActions, err := pm.ActionCli().ListAllActions(context.TODO(), &api.ListActionsOptions{
				Parent: "",
			})
			if err != nil {
				log.Fatalf("unable to list system actions: %v", err)
			}

			allActions := append(actions, systemActions...)

			// Convert users to nicknames.
			convertActionUsers(allActions, pm)

			// Print listed actions.
			err = printer.Printer(outputFormat, &printer.Options{TableOpts: &table.PrintOpts{
				Verbose: verbose,
			}}).PrintObj(printable.NewAction(allActions), os.Stdout)
			if err != nil {
				log.Fatalf("failed to print actions: %v", err)
			}
		},
	}

	cmd.Flags().StringVarP(&projectSlug, "project", "p", "", "the slug of the working project")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "output format (table|json)")

	return cmd
}

func convertActionUsers(actions []*openv1alpha1resource.Action, pm *config.ProfileManager) {
	// Search for all users in actions authors.
	usersSet := mapset.NewSet[name.User]()
	for _, a := range actions {
		if userName, err := name.NewUser(a.Author); err == nil {
			usersSet.Add(*userName)
		}
	}

	// Batch get users
	usersMap, err := pm.UserCli().BatchGetUsers(context.TODO(), usersSet)
	if err != nil {
		log.Fatalf("failed to batch get users: %v", err)
	}

	// Convert users to nicknames
	for _, a := range actions {
		if _, err := name.NewUser(a.Author); err == nil {
			if u, ok := usersMap[a.Author]; ok {
				a.Author = *u.Nickname
			}
		}
	}
}
