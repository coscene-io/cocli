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

	"github.com/coscene-io/cocli/internal/config"
	"github.com/coscene-io/cocli/internal/prompts"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewRunCommand(cfgPath *string) *cobra.Command {
	var (
		params      = map[string]string{}
		skipParams  = false
		force       = false
		projectSlug = ""
	)

	cmd := &cobra.Command{
		Use:                   "run <action-resource-name/id> <record-resource-name/id> [-p <working-project-slug>] [-P <key1=value1>...] [--skip-params] [-f]",
		Short:                 "Create an action run.",
		DisableFlagsInUseLine: true,
		Run: func(cmd *cobra.Command, args []string) {
			// Get current profile.
			pm, _ := config.Provide(*cfgPath).GetProfileManager()
			proj, err := pm.ProjectName(cmd.Context(), projectSlug)
			if err != nil {
				log.Fatalf("unable to get project name: %v", err)
			}

			// Handle args and flags.
			// TODO: currently the parsing of action name is kind of hacky, need to improve this
			actionName, err := pm.ActionCli().ActionId2Name(context.TODO(), args[0], proj)
			if err != nil {
				log.Fatalf("failed to convert action id to name: %v", err)
			}
			recordName, err := pm.RecordCli().RecordId2Name(context.TODO(), args[1], proj)
			if err != nil {
				log.Fatalf("failed to convert record id to name: %v", err)
			}

			// Fetch action
			act, err := pm.ActionCli().GetByName(context.TODO(), actionName)
			if err != nil {
				log.Fatalf("failed to get action by name %s: %v", actionName, err)
			}

			if !skipParams {
				if cmd.Flags().Changed("param") {
					for k, v := range params {
						act.Spec.Parameters[k] = v
					}
				} else {
					// prompt to ask for parameters
					for k, v := range act.Spec.Parameters {
						act.Spec.Parameters[k] = prompts.PromptString(fmt.Sprintf("Enter value for parameter %s", k), v)
					}
				}
			}

			// Print final parameters
			fmt.Println("\nThe final parameters in the action run to be created:")
			for k, v := range act.Spec.Parameters {
				fmt.Printf("%s: %s\n", k, v)
			}

			// Prompt user for confirmation
			if !force {
				if !prompts.PromptYN("Confirm to run action?") {
					fmt.Println("Action run creation aborted.")
					return
				}
			}

			// Create action run
			err = pm.ActionCli().CreateActionRun(context.TODO(), act, recordName)
			if err != nil {
				log.Fatalf("failed to create action run: %v", err)
			}

			fmt.Println("Action run created successfully.")
		},
	}

	cmd.Flags().StringToStringVarP(&params, "param", "P", nil, "action parameters")
	cmd.Flags().BoolVar(&skipParams, "skip-params", false, "skip parameter input and use default values")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "force create action run without confirmation")
	cmd.Flags().StringVarP(&projectSlug, "project", "p", "", "the slug of the working project")

	_ = cmd.MarkFlagRequired("record")
	cmd.MarkFlagsMutuallyExclusive("skip-params", "param")

	return cmd
}
