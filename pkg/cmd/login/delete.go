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
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewDeleteCommand(cfgPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "delete <profile-name>",
		Short:                 "Delete a login profile.",
		DisableFlagsInUseLine: true,
		Args:                  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cfg := config.Provide(*cfgPath)
			pm, _ := cfg.GetProfileManager()

			if len(pm.Profiles) == 1 {
				log.Fatalf("Cannot delete the last profile. Use 'coscli login set' to change the current profile.")
			}

			if err := pm.DeleteProfile(args[0]); err != nil {
				log.Fatalf("Failed to delete login profile %s: %v", args[0], err)
			}

			if err := cfg.Persist(pm); err != nil {
				log.Fatalf("Failed to persist profile manager: %v", err)
			}

			fmt.Println("Profile deleted.")

			// Print the current profile
			curProfile := pm.GetCurrentProfile()
			fmt.Printf("Current Profile is:\n%s\n", curProfile)
		},
	}

	return cmd
}
