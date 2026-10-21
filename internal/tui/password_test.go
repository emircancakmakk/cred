package tui

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func TestGeneratePasswordFromEveryFieldFocus(t *testing.T) {
	for _, action := range []string{"n", "e"} {
		for focus := 0; focus < 6; focus++ {
			m := editable(openVercel(t))
			m = update(t, m, action)
			if action == "n" {
				m, _ = dialogKey(m, tea.KeyEnter)
			}
			m.editor.setFocus(focus)
			m.editor.secret = false
			m.editor.value.EchoMode = textinput.EchoNormal
			m, _ = dialogKey(m, tea.KeyCtrlG)
			value := m.editor.value.Value()
			if len(value) != 24 || !m.editor.secret || m.editor.value.EchoMode != textinput.EchoPassword || m.editor.focus != 1 || !m.editor.value.Focused() || strings.Contains(m.View(), value) || m.saving {
				t.Fatal("generation failed to fill, focus and mask the draft")
			}
			m.editor.setFocus(3)
			m.editor.generate = func() (string, error) { return "replacement-generated-value", nil }
			m, _ = dialogKey(m, tea.KeyEnter)
			if m.editor.value.Value() != "replacement-generated-value" || m.editor.focus != 1 || m.saving {
				t.Fatal("Generate button failed to replace draft")
			}
			before := m.projects[0].Services[0].Fields[0].Value
			m = update(t, m, "esc")
			if m.editor != nil || m.projects[0].Services[0].Fields[0].Value != before || strings.Contains(m.View(), "replacement-generated-value") {
				t.Fatal("cancel changed snapshot or exposed draft")
			}
		}
	}
}

func TestGenerateFailurePreservesDraft(t *testing.T) {
	m := editable(openVercel(t))
	m = update(t, m, "e")
	m.editor.value.SetValue("existing-draft")
	m.editor.secret = false
	m.editor.value.EchoMode = textinput.EchoNormal
	m.editor.setFocus(3)
	m.editor.generate = func() (string, error) { return "partial-secret", errors.New("backend-secret") }
	m, _ = dialogKey(m, tea.KeyEnter)
	if m.editor.value.Value() != "existing-draft" || m.editor.secret || m.editor.focus != 3 || !strings.Contains(m.View(), "Could not generate password") || strings.Contains(m.View(), "backend-secret") || strings.Contains(m.View(), "partial-secret") {
		t.Fatal("generation failure lost draft or exposed backend error")
	}
	m.editor.generate = func() (string, error) { return "successful-retry", nil }
	m, _ = dialogKey(m, tea.KeyCtrlG)
	if m.editor.generateErr != "" || m.editor.value.Value() != "successful-retry" {
		t.Fatal("retry failed")
	}
}

func TestGenerationUnavailableForProjectAndService(t *testing.T) {
	m := editable(mustModel(t, sampleProjects(), "", nil))
	for range 2 {
		m = update(t, m, "e")
		m, _ = dialogKey(m, tea.KeyCtrlG)
		if strings.Contains(m.View(), "Generate") || strings.Contains(m.View(), "ctrl+g") || m.editor.value.Value() != "" {
			t.Fatal("non-field form offers generation")
		}
		m = update(t, m, "esc")
		m = update(t, m, "enter")
	}
}

func TestGeneratedPasswordReachesCommit(t *testing.T) {
	for _, action := range []string{"n", "e"} {
		m := editable(openVercel(t))
		m = update(t, m, action)
		if action == "n" {
			m, _ = dialogKey(m, tea.KeyEnter)
		}
		m.editor.name.SetValue("password")
		m, _ = dialogKey(m, tea.KeyCtrlG)
		value := m.editor.value.Value()
		m.editor.setFocus(m.editor.saveFocus())
		var received Change
		m.commit = func(c Change) (Snapshot, error) { received = c; return Snapshot{Projects: sampleProjects()}, nil }
		p := tea.NewProgram(exitAfterSave{m}, tea.WithInput(strings.NewReader("")), tea.WithOutput(io.Discard), tea.WithoutRenderer())
		done := make(chan error, 1)
		go func() { _, err := p.Run(); done <- err }()
		go p.Send(keyMsg("enter"))
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
			if received.Value != value || !received.Secret || received.NewName != "password" {
				t.Fatal("commit lost generated password")
			}
		case <-time.After(5 * time.Second):
			p.Kill()
			t.Fatal("save did not complete")
		}
	}
}

func TestEnterProgressionPreservesManualPassword(t *testing.T) {
	for _, action := range []string{"n", "e"} {
		m := editable(openVercel(t))
		m = update(t, m, action)
		if action == "n" {
			m, _ = dialogKey(m, tea.KeyEnter)
		}
		m.editor.value.SetValue("manual-password-fixture")
		m.editor.setFocus(1)
		m.editor.generate = func() (string, error) {
			t.Fatal("Enter progression must not generate a password")
			return "", nil
		}
		for range 2 {
			m, _ = dialogKey(m, tea.KeyEnter)
		}
		if m.editor.focus != m.editor.saveFocus() || m.editor.value.Value() != "manual-password-fixture" || m.saving {
			t.Fatal("Enter must reach Save and preserve the manual value")
		}
		m, cmd := dialogKey(m, tea.KeyEnter)
		if !m.saving || cmd == nil {
			t.Fatal("Enter on Save must start persistence")
		}
	}
}
