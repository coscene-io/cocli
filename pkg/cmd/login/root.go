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
	"github.com/spf13/cobra"
)

func NewRootCommand(cfgPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login to coScene",
	}

	cmd.AddCommand(NewAddCommand(cfgPath))
	cmd.AddCommand(NewCurrentCommand(cfgPath))
	cmd.AddCommand(NewDeleteCommand(cfgPath))
	cmd.AddCommand(NewListCommand(cfgPath))
	cmd.AddCommand(NewSetCommand(cfgPath))
	cmd.AddCommand(NewSwitchCommand(cfgPath))

	return cmd
}
