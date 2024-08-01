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

package login

import (
	"fmt"

	"github.com/coscene-io/cocli/internal/config"
	"github.com/coscene-io/cocli/internal/constants"
	"github.com/coscene-io/cocli/pkg/cmd_utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewSetCommand(cfgPath *string) *cobra.Command {
	var (
		name        = ""
		endpoint    = ""
		token       = ""
		projectSlug = ""
	)

	cmd := &cobra.Command{
		Use:                   "set [-n <profile-name>] [-e <endpoint>] [-t <token>] [-p <project-slug>]",
		Short:                 "Set the current login profile, add a new one if no login profile exists.",
		DisableFlagsInUseLine: true,
		Args:                  cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			cfg := config.Provide(*cfgPath)
			pm, _ := cfg.GetProfileManager()

			if pm.IsEmpty() {
				// Fill default values if pm is empty
				if name == "" {
					name = "saas"
				}

				if endpoint == "" {
					endpoint = constants.BaseApiEndpoint
				}
			}

			if err := pm.SetProfile(&config.Profile{
				Name:        name,
				EndPoint:    endpoint,
				Token:       token,
				ProjectSlug: projectSlug,
			}); err != nil {
				log.Fatalf("Failed to set login profile: %v", err)
			}

			if err := cfg.Persist(pm); err != nil {
				log.Fatalf("Failed to persist profile manager: %v", err)
			}

			fmt.Println("Profile set successful.")

			// Print the current profile
			curProfile := pm.GetCurrentProfile()
			fmt.Printf("Current Profile is:\n%s\n", curProfile)
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "name of the login profile.")
	cmd.Flags().StringVarP(&endpoint, "endpoint", "e", "", "coScene API server endpoint.")
	cmd.Flags().StringVarP(&token, "token", "t", "", "access token for coScene API server.")
	cmd.Flags().StringVarP(&projectSlug, "project", "p", "", "the slug of the working project")

	cmd_utils.DisableAuthCheck(cmd)

	return cmd
}
