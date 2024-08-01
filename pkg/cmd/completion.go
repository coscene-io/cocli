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
	"io"

	"github.com/coscene-io/cocli/pkg/cmd_utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewCompletionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "completion <shell>",
		Short:                 "Generate the autocompletion script for coscli for the specified shell. Supporting Zsh and Bash.",
		DisableFlagsInUseLine: true,
		Args:                  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			run, found := completionShells[args[0]]
			if !found {
				log.Fatalf("Unsupported shell type %q.", args[0])
			}

			if err := run(cmd.OutOrStdout(), cmd); err != nil {
				log.Fatalf("Failed to generate completion script: %v", err)
			}
		},
	}

	cmd_utils.DisableAuthCheck(cmd)

	return cmd
}

var (
	completionShells = map[string]func(out io.Writer, cmd *cobra.Command) error{
		"zsh":  runCompletionZsh,
		"bash": runCompletionBash,
	}
)

func runCompletionBash(out io.Writer, cmd *cobra.Command) error {
	return cmd.Root().GenBashCompletion(out)
}

func runCompletionZsh(out io.Writer, cmd *cobra.Command) error {
	zshHead := fmt.Sprintf("#compdef %[1]s\ncompdef _%[1]s %[1]s\n", cmd.Root().Name())
	_, err := out.Write([]byte(zshHead))
	if err != nil {
		return err
	}

	return cmd.Root().GenZshCompletion(out)
}
