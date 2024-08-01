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

// stringModel is a bubbletea model for prompting user to enter a string.
type stringModel struct {
	promptMsg     string
	enteredString string
	defaultValue  string
	windowWidth   int
	quit          bool
}

func (m stringModel) Init() tea.Cmd {
	return nil
}

func (m stringModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyRunes:
			m.enteredString += string(msg.Runes)
		case tea.KeyBackspace:
			if len(m.enteredString) > 0 {
				m.enteredString = m.enteredString[:len(m.enteredString)-1]
			}
		case tea.KeyEnter:
			if m.enteredString == "" {
				m.enteredString = m.defaultValue
			}
			return m, tea.Quit
		case tea.KeyCtrlC, tea.KeyEscape, tea.KeyCtrlD:
			fmt.Println("Quitting...")
			m.quit = true
			return m, tea.Quit
		default:
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.windowWidth = msg.Width
	}
	return m, nil
}

func (m stringModel) View() string {
	typeToChange := "(type to change)"
	if m.enteredString != "" {
		typeToChange = " " + typeToChange
	}
	value := m.defaultValue + typeToChange
	if m.enteredString != "" {
		value = m.enteredString
	}
	return wordwrap.String(fmt.Sprintf("%s:\n%s\n", m.promptMsg, value), m.windowWidth)
}

func PromptString(promptMsg string, defaultValue string) string {
	p := tea.NewProgram(stringModel{promptMsg: promptMsg, defaultValue: defaultValue})
	finalModel, err := p.Run()
	if err != nil {
		log.Fatalf("Error running string prompt: %v", err)
	}
	if finalModel.(stringModel).quit {
		os.Exit(1)
	}
	return finalModel.(stringModel).enteredString
}
