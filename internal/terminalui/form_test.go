package terminalui

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func key(m *Form, k tea.KeyType) { m.Update(tea.KeyMsg{Type: k}) }
func text(m *Form, s string)     { m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s), Paste: true}) }
func TestFormKeyboardSecretsAndValidation(t *testing.T) {
	m := NewForm(FormSpec{Title: "Open vault", Submit: "Open", Fields: []Field{{Label: "Password", Sensitive: true, Required: true}}})
	key(m, tea.KeyEnter)
	if m.Accepted || m.Focus != 1 {
		t.Fatal("input Enter submitted")
	}
	key(m, tea.KeyEnter)
	if m.Focus != 0 || m.Error == "" {
		t.Fatal("empty password submitted")
	}
	original := "  Ab界🙂cd  "
	text(m, original)
	if strings.Contains(m.View(), "Ab界") || !strings.Contains(m.View(), "Hidden") {
		t.Fatal("password not masked")
	}
	key(m, tea.KeyCtrlR)
	if !strings.Contains(m.View(), "Ab界🙂cd") || !strings.Contains(m.View(), "Visible") {
		t.Fatal("reveal failed")
	}
	key(m, tea.KeyCtrlR)
	key(m, tea.KeyHome)
	key(m, tea.KeyDelete)
	key(m, tea.KeyEnd)
	key(m, tea.KeyBackspace)
	key(m, tea.KeyLeft)
	text(m, "Z")
	if m.Values()[0] != " Ab界🙂cdZ " {
		t.Fatal("editing changed value unexpectedly")
	}
	key(m, tea.KeyEnter)
	key(m, tea.KeyEnter)
	if !m.Accepted || m.View() != "" {
		t.Fatal("submit retained screen")
	}
	m.ClearSecrets()
	if m.Values()[0] != "" {
		t.Fatal("cleanup retained password")
	}
}
func TestFormBackAndCancelClearOnlySecrets(t *testing.T) {
	for _, cancel := range []bool{true, false} {
		m := NewForm(FormSpec{Title: "Details", Back: true, Fields: []Field{{Label: "Name", Value: "kept"}, {Label: "Password", Sensitive: true}}})
		key(m, tea.KeyTab)
		text(m, "private")
		key(m, tea.KeyCtrlR)
		if cancel {
			key(m, tea.KeyEsc)
		} else {
			key(m, tea.KeyTab)
			key(m, tea.KeyTab)
			key(m, tea.KeyEnter)
		}
		if m.Values()[0] != "kept" || m.Values()[1] != "" || (!m.Cancelled && !m.WentBack) {
			t.Fatal("back/cancel lost normal data or retained secret")
		}
	}
}
func TestFormAndSelectionFitAndScroll(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {40, 20}, {120, 30}, {24, 16}} {
		m := NewForm(FormSpec{Context: strings.Repeat("vault 界", 20), Title: "Create vault", Description: strings.Repeat("Long description 界. ", 12), Back: true, Fields: []Field{{Label: "Profile name", Value: strings.Repeat("密钥🙂", 80)}, {Label: "Password", Sensitive: true, Value: strings.Repeat("密钥🙂", 80)}}})
		m.Width, m.Height = size[0], size[1]
		for range 5 {
			view := m.View()
			if lipgloss.Width(view) > size[0] || lipgloss.Height(view) > size[1] {
				t.Fatalf("form exceeds %v:\n%s", size, view)
			}
			key(m, tea.KeyTab)
		}
		m.Error = strings.Repeat("Cannot save 界. ", 50) + "\nEND"
		for range 100 {
			key(m, tea.KeyPgDown)
		}
		if !strings.Contains(m.View(), "END") {
			t.Fatal("long error not scrollable")
		}
		previous := m.View()
		key(m, tea.KeyPgUp)
		if m.View() == previous {
			t.Fatal("scroll clamping blocked return")
		}
		selection := NewSelection(SelectionSpec{Title: "Delete vault", Description: strings.Repeat("Descendant details. ", 10), Options: []Option{{Label: "Cancel", Value: "n"}, {Label: "Delete", Value: "y", Destructive: true}}})
		selection.Width, selection.Height = size[0], size[1]
		if !strings.Contains(ansi.Strip(selection.View()), "> Cancel") {
			t.Fatal("unsafe default")
		}
		selection.Update(tea.KeyMsg{Type: tea.KeyEnter})
		if selection.Spec.Options[selection.Index].Value != "n" {
			t.Fatal("default Enter deleted")
		}
	}
}
func TestNonTerminalOutputIsByteExact(t *testing.T) {
	original := "Ready: vault\nNext step: use it\n界🙂\n"
	var out bytes.Buffer
	n, err := (Writer{&out}).Write([]byte(original))
	if err != nil || n != len(original) || out.String() != original {
		t.Fatal("pipe output changed")
	}
}
func TestOptionalEmptyValueAndErrorPreservation(t *testing.T) {
	m := NewForm(FormSpec{Title: "Value", Fields: []Field{{Label: "Value", Sensitive: true}}})
	key(m, tea.KeyEnter)
	key(m, tea.KeyEnter)
	if !m.Accepted || m.Values()[0] != "" {
		t.Fatal("empty field value rejected")
	}
	m = NewForm(FormSpec{Title: "Name", Fields: []Field{{Label: "Name", Value: "duplicate"}}, Validate: func([]string) error { return errors.New("Already exists.") }})
	key(m, tea.KeyEnter)
	key(m, tea.KeyEnter)
	if m.Accepted || m.Values()[0] != "duplicate" || !strings.Contains(m.View(), "Already exists.") {
		t.Fatal("validation lost inputs")
	}
}

func TestSelectionFreezesAfterConfirmation(t *testing.T) {
	m := NewSelection(SelectionSpec{Title: "Delete", Buttons: true, Options: []Option{{Label: "Cancel", Value: "n"}, {Label: "Delete", Value: "y", Destructive: true}}})
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.Index != 0 {
		t.Fatal("queued key changed an accepted safe choice")
	}
}
