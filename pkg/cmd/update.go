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

	"github.com/coscene-io/cocli"
	"github.com/coscene-io/cocli/internal/constants"
	"github.com/coscene-io/cocli/pkg/cmd_utils"
	"github.com/pkg/errors"
	"github.com/sanbornm/go-selfupdate/selfupdate"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "update",
		Short:                 "Update coscli version",
		DisableFlagsInUseLine: true,
		Args:                  cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			var updater = &selfupdate.Updater{
				CurrentVersion: cocli.GetVersion(),
				ApiURL:         constants.DownloadBaseUrl,
				BinURL:         constants.DownloadBaseUrl,
				CmdName:        constants.CLIName,
				ForceCheck:     true,
				OnSuccessfulUpdate: func() {
					fmt.Println("Successfully updated to the latest version")
				},
			}

			newVersion, err := updater.UpdateAvailable()
			if err != nil {
				log.Fatal("Failed to check for update:", err)
			}

			updater.OnSuccessfulUpdate = func() {
				fmt.Println("Successfully updated to version", newVersion)
			}

			err = updater.Update()
			if errors.Is(err, os.ErrPermission) {
				log.Fatal("Permission denied. Please run with sudo or as root.")
			} else if err != nil {
				log.Fatal("Failed to update:", err)
			}
		},
	}

	cmd_utils.DisableAuthCheck(cmd)

	return cmd
}
