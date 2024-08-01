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

package prompts

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/reflow/wordwrap"
	log "github.com/sirupsen/logrus"
)

// ynModel is a bubbletea model for prompting user to enter a y/n.
type ynModel struct {
	promptMsg   string
	confirmed   bool
	enteredKey  string
	windowWidth int
	quit        bool
}

func (m ynModel) Init() tea.Cmd {
	return nil
}

func (m ynModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "ctrl+d":
			fmt.Println("Quitting...")
			m.quit = true
			return m, tea.Quit
		case "y":
			m.enteredKey = "y"
			m.confirmed = true
			return m, tea.Quit
		case "n":
			m.enteredKey = "n"
			m.confirmed = false
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.windowWidth = msg.Width
	}
	return m, nil
}

func (m ynModel) View() string {
	return wordwrap.String(fmt.Sprintf("%s (y/n) %s\n", m.promptMsg, m.enteredKey), m.windowWidth)
}

func PromptYN(promptMsg string) bool {
	p := tea.NewProgram(ynModel{promptMsg: promptMsg})
	finalModel, err := p.Run()
	if err != nil {
		log.Fatalf("Error running y/n prompt: %v", err)
	}
	if finalModel.(ynModel).quit {
		os.Exit(1)
	}
	return finalModel.(ynModel).confirmed
}
