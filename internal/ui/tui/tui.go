package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dpanel-dev/installer/internal/install"
)

type model struct {
	config *install.Config
	cursor int
	step   int
}

func InitialModel() model {
	return model{
		config: &install.Config{},
		step:   0,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	return fmt.Sprintf("\n  DPanel Installer TUI (Step %d)\n\n  Press 'q' to quit.\n", m.step)
}

func StartTUI() error {
	p := tea.NewProgram(InitialModel())
	_, err := p.Run()
	return err
}
