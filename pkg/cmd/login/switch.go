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

	tea "github.com/charmbracelet/bubbletea"
	"github.com/coscene-io/cocli/internal/config"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
)

func NewSwitchCommand(cfgPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "switch",
		Short:                 "Switch to another login profile.",
		DisableFlagsInUseLine: true,
		Args:                  cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			cfg := config.Provide(*cfgPath)
			pm, _ := cfg.GetProfileManager()

			profile, err := promptForProfile(pm.GetProfiles(), pm.GetCurrentProfile().Name)
			if err != nil {
				log.Fatalf("Failed to prompt for select profile: %v", err)
			}

			err = pm.SwitchProfile(profile.Name)
			if err != nil {
				log.Fatalf("Failed to switch to profile %s: %v", profile.Name, err)
			}

			if err = cfg.Persist(pm); err != nil {
				log.Fatalf("Failed to persist profile manager: %v", err)
			}

			curProfile := pm.GetCurrentProfile()
			fmt.Printf("Successfully switched to profile:\n%s\n", curProfile)
		},
	}

	return cmd
}

type selectProfileModel struct {
	profiles   []*config.Profile
	initCursor int
	cursor     int
	selected   int
}

func (m selectProfileModel) Init() tea.Cmd {
	return nil
}

func (m selectProfileModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.profiles)-1 {
				m.cursor++
			}
		case "enter":
			m.selected = m.cursor
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m selectProfileModel) View() string {
	if m.selected >= 0 {
		return ""
	}
	var s string
	s += "Use the arrow keys to navigate, press enter to select a profile, and press q to quit.\n\n"
	for i, choice := range m.profiles {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}
		initIndicator := ""
		if m.initCursor == i {
			initIndicator = "(*)"
		}
		s += fmt.Sprintf("%s %s %s\n", cursor, choice.Name, initIndicator)
	}

	s += fmt.Sprintf("\n--------- Info ----------\n%s", m.profiles[m.cursor])

	return s
}

func promptForProfile(profiles []*config.Profile, currentProfile string) (*config.Profile, error) {
	profileNames := lo.Map(profiles, func(p *config.Profile, _ int) string { return p.Name })
	p := tea.NewProgram(selectProfileModel{
		profiles:   profiles,
		initCursor: slices.Index(profileNames, currentProfile),
		cursor:     slices.Index(profileNames, currentProfile),
		selected:   -1,
	})
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}
	selected := finalModel.(selectProfileModel).selected
	if selected < 0 {
		return nil, fmt.Errorf("prompt failed")
	}
	return profiles[selected], nil
}
