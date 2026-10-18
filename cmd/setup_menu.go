package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

type setupChoice int

const (
	setupLocal setupChoice = iota
	setupConnect
	setupShared
)

var errSetupCancelled = errors.New("setup cancelled")
var setupLabels = []string{"Create a local vault", "Connect to an existing team vault", "Create a shared vault in an empty Git repository"}

type setupModel struct {
	selected            setupChoice
	accepted, cancelled bool
}

func (setupModel) Init() tea.Cmd { return nil }
func (m setupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up", "k":
			m.selected = (m.selected + 2) % 3
		case "down", "j":
			m.selected = (m.selected + 1) % 3
		case "enter":
			m.accepted = true
			return m, tea.Quit
		case "esc", "ctrl+c":
			m.cancelled = true
			return m, tea.Quit
		}
	}
	return m, nil
}
func (m setupModel) View() string {
	var b strings.Builder
	b.WriteString("cred setup\n\nHow would you like to get started?\n\n")
	for i, label := range setupLabels {
		marker := "  "
		if setupChoice(i) == m.selected {
			marker = "> "
		}
		fmt.Fprintf(&b, "%s%s\n", marker, label)
	}
	b.WriteString("\n↑/↓ select · Enter continue · Esc cancel\n")
	return b.String()
}
func (a *App) chooseSetup() (setupChoice, error) {
	if a.setupMenu != nil {
		return a.setupMenu()
	}
	in, ok := a.In.(*os.File)
	if !ok || !term.IsTerminal(int(in.Fd())) {
		return 0, errors.New("interactive terminal required; use cred init <name> for direct local setup")
	}
	out, ok := a.errWriter().(*os.File)
	if !ok || !term.IsTerminal(int(out.Fd())) {
		return 0, errors.New("interactive terminal required for setup menu")
	}
	final, e := tea.NewProgram(setupModel{}, tea.WithInput(in), tea.WithOutput(out), tea.WithAltScreen()).Run()
	if e != nil {
		return 0, e
	}
	m, ok := final.(setupModel)
	if !ok || m.cancelled || !m.accepted {
		return 0, errSetupCancelled
	}
	return m.selected, nil
}
