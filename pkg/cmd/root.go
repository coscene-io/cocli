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

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/coscene-io/cocli"
	"github.com/coscene-io/cocli/internal/config"
	"github.com/coscene-io/cocli/internal/constants"
	"github.com/coscene-io/cocli/pkg/cmd/action"
	"github.com/coscene-io/cocli/pkg/cmd/login"
	"github.com/coscene-io/cocli/pkg/cmd/project"
	"github.com/coscene-io/cocli/pkg/cmd/record"
	"github.com/coscene-io/cocli/pkg/cmd_utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cfgPath := ""

	cmd := &cobra.Command{
		Use:     constants.CLIName,
		Short:   "",
		Version: cocli.GetVersion(),
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// check if cfgPath exists and isFile
			cfgPathInfo, err := os.Stat(cfgPath)

			if os.IsNotExist(err) {
				// cfgPath does not exist, prompt user to create it
				if cfgPath != constants.DefaultConfigPath {
					log.Fatalf("Config file does not exist at %s, aborting.", cfgPath)
				}

				fmt.Println("Initializing config file at", cfgPath)

				err = os.MkdirAll(filepath.Dir(cfgPath), 0755)
				if err != nil {
					log.Fatalf("failed to mkdir all for cfgPath: %v", err)
				}
				_, err = os.Create(cfgPath)
				if err != nil {
					log.Fatalf("failed to create config file: %v", err)
				}
			} else if err != nil {
				log.Fatalf("Error checking config file: %v", err)
			} else if cfgPathInfo.IsDir() {
				log.Fatalf("Config file path is a directory: %s", cfgPath)
			}

			cfg := config.Provide(cfgPath)
			pm, err := cfg.GetProfileManager()
			if err != nil {
				log.Fatalf("Failed to get profile manager from config: %v", err)
			}

			// Auth Check
			if cmd_utils.IsAuthCheckEnabled(cmd) {
				if pm.IsEmpty() {
					fmt.Println("Config file is empty, please run `cocli login set` to initialize your login profile.")
					os.Exit(0)
				}
				if !pm.CheckAuth() {
					if err = pm.Auth(); err != nil {
						log.Fatalf("Failed to authenticate current login profile: %v", err)
					}

					if err = cfg.Persist(pm); err != nil {
						log.Fatalf("Failed to persist profile manager: %v", err)
					}
				}
			}
		},
	}

	cmd.PersistentFlags().StringVar(&cfgPath, "config", constants.DefaultConfigPath, "config file path")

	cmd.AddCommand(NewCompletionCommand())
	cmd.AddCommand(action.NewRootCommand(&cfgPath))
	cmd.AddCommand(login.NewRootCommand(&cfgPath))
	cmd.AddCommand(project.NewRootCommand(&cfgPath))
	cmd.AddCommand(record.NewRootCommand(&cfgPath))
	cmd.AddCommand(NewUpdateCommand())

	return cmd
}
