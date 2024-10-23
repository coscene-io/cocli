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
	"os/exec"

	"connectrpc.com/connect"
	"github.com/coscene-io/cocli/internal/config"
	"github.com/coscene-io/cocli/internal/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewViewCommand(cfgPath *string) *cobra.Command {
	var (
		goToWeb     = false
		projectSlug = ""
	)
	cmd := &cobra.Command{
		Use:                   "view <record-resource-name/id> [-p <working-project-slug>] [-w]",
		Aliases:               []string{"open"},
		Short:                 "View record.",
		Args:                  cobra.ExactArgs(1),
		DisableFlagsInUseLine: true,
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

			// Get record url.
			recordUrl, err := pm.GetRecordUrl(recordName)
			if err != nil {
				log.Fatalf("unable to get record url: %v", err)
			}

			fmt.Println("The record url is:", recordUrl)
			if goToWeb {
				err = exec.Command("open", recordUrl).Start()
				if err != nil {
					log.Fatalf("unable to open record in web browser: %v", err)
				}
			}
		},
	}

	cmd.Flags().BoolVarP(&goToWeb, "web", "w", false, "open record in web browser")
	cmd.Flags().StringVarP(&projectSlug, "project", "p", "", "the slug of the working project")

	return cmd
}
