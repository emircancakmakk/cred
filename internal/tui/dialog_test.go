package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestEditorActionsAndFocus(t *testing.T) {
	for _, level := range []int{0, 1, 2} {
		t.Run(fmt.Sprint(level), func(t *testing.T) {
			m := editable(mustModel(t, sampleProjects(), "", nil))
			for range level {
				m = update(t, m, "enter")
			}
			m = update(t, m, "e")
			saveFocus := m.editor.saveFocus()
			for focus := 1; focus <= saveFocus; focus++ {
				m = update(t, m, "enter")
				if m.saving || m.editor.focus != focus {
					t.Fatalf("enter saved before Save, focus=%d", focus)
				}
			}
			m, _ = dialogKey(m, tea.KeyTab)
			if m.editor.focus != saveFocus+1 {
				t.Fatal("Cancel missing from focus order")
			}
			m, _ = dialogKey(m, tea.KeyTab)
			if m.editor.focus != 0 || !m.editor.name.Focused() {
				t.Fatal("Tab did not wrap to Name")
			}
			m, _ = dialogKey(m, tea.KeyShiftTab)
			m, cmd := dialogKey(m, tea.KeyEnter)
			if cmd != nil || m.editor != nil || m.saving {
				t.Fatal("Cancel attempted to save")
			}

			m = update(t, m, "e")
			for range saveFocus {
				m, _ = dialogKey(m, tea.KeyTab)
			}
			m, cmd = dialogKey(m, tea.KeyEnter)
			if cmd == nil || !m.saving {
				t.Fatal("Save did not start persistence")
			}
			m, cmd = dialogKey(m, tea.KeyEnter)
			if cmd != nil || !m.saving || !strings.Contains(m.View(), "Saving…") {
				t.Fatal("duplicate save was not blocked")
			}
		})
	}
}

func TestDialogFitsTerminalAndKeepsFocusVisible(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {40, 20}, {120, 30}, {24, 16}} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			m := editable(openVercel(t)).resize(size[0], size[1])
			m.vaultLabel = strings.Repeat("Long vault 界 ", 12)
			m = update(t, m, "e")
			m.editor.name.SetValue(strings.Repeat("密钥🙂", 60))
			for _, label := range []string{"> Name", "> Value", "> [ ] Secret", "> [ Save ]", "> [ Cancel ]"} {
				view := ansi.Strip(m.View())
				assertDialogBounds(t, view, size)
				if !strings.Contains(view, label) {
					t.Fatalf("focus %q not visible:\n%s", label, view)
				}
				m, _ = dialogKey(m, tea.KeyTab)
			}
			m = m.finishSave(saved{err: errors.New(strings.Repeat("Unable to save 界. ", 30) + "\nEND-OF-ERROR")})
			view := m.View()
			for range 100 {
				m = update(t, m, "pgdown")
			}
			scrolled := ansi.Strip(m.View())
			assertDialogBounds(t, scrolled, size)
			if scrolled == ansi.Strip(view) || !strings.Contains(scrolled, "END-OF-ERROR") {
				t.Fatalf("long error not scrollable:\n%s", scrolled)
			}
			m = update(t, m, "pgup")
			if ansi.Strip(m.View()) == scrolled {
				t.Fatal("overscrolling prevented scrolling back")
			}
			m, _ = dialogKey(m, tea.KeyTab)
			if m.dialogScroll != 0 || !strings.Contains(ansi.Strip(m.View()), "> Value") {
				t.Fatal("focus change did not restore viewport")
			}
		})
	}
}

func TestDeletePanelFitsAndShowsSelectedAction(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {40, 20}, {120, 30}, {24, 16}} {
		m := editable(mustModel(t, sampleProjects(), "", nil)).resize(size[0], size[1])
		m = update(t, m, "d")
		assertDialogBounds(t, m.View(), size)
		if !strings.Contains(m.View(), "> [ Cancel ]") {
			t.Fatal("Cancel not visibly selected")
		}
		m, _ = dialogKey(m, tea.KeyRight)
		assertDialogBounds(t, m.View(), size)
		if !strings.Contains(m.View(), "> [ Delete ]") {
			t.Fatalf("Delete not visibly selected: %s", m.View())
		}
	}
}

func TestBrowserHelpMatchesContext(t *testing.T) {
	m := mustModel(t, sampleProjects(), "", nil)
	if strings.Contains(m.footer(), "n new") || strings.Contains(m.footer(), "s show") {
		t.Fatal("read-only project list advertises unavailable actions")
	}
	m = editable(m).resize(40, 20)
	for _, line := range strings.Split(m.footer(), "\n") {
		if lipgloss.Width(line) > 40 {
			t.Fatal("help exceeds terminal width")
		}
	}
	m = update(t, m, "/")
	if strings.Contains(m.footer(), "n new") || !strings.Contains(m.footer(), "enter apply") {
		t.Fatal("filter help does not match current actions")
	}
}

func assertDialogBounds(t *testing.T, view string, size [2]int) {
	t.Helper()
	if lipgloss.Width(view) > size[0] || lipgloss.Height(view) > size[1] {
		t.Fatalf("dialog %dx%d exceeds terminal %v:\n%s", lipgloss.Width(view), lipgloss.Height(view), size, view)
	}
}

func TestDialogInputUsesOneLineWithoutChangingValue(t *testing.T) {
	m := editable(openVercel(t))
	m = update(t, m, "e")
	value := strings.Repeat("界🙂é", 100)
	m.editor.name.SetValue(value)
	m.editor.name.SetCursor(150)
	view := dialogInput("Name", m.editor.name, true, false, 36)
	if lipgloss.Height(view) != 4 || lipgloss.Width(view) > 36 {
		t.Fatalf("input escaped viewport:\n%s", view)
	}
	if m.editor.name.Value() != value || m.editor.name.Position() != 150 {
		t.Fatal("rendering changed input contents or cursor")
	}
}

func TestTinyDialogRequestsResize(t *testing.T) {
	for _, size := range [][2]int{{10, 4}, {20, 9}, {24, 10}} {
		m := editable(openVercel(t)).resize(size[0], size[1])
		m = update(t, m, "e")
		assertDialogBounds(t, m.View(), size)
		m = update(t, m, "esc")
		if m.editor != nil {
			t.Fatal("tiny terminal blocked cancellation")
		}
	}
}

func TestFieldRevealDoesNotChangeSecretFlag(t *testing.T) {
	m := editable(openVercel(t))
	m = update(t, m, "j")
	m = update(t, m, "e")
	m, _ = dialogKey(m, tea.KeyTab)
	m, _ = dialogKey(m, tea.KeyCtrlR)
	if !m.editor.secret || !strings.Contains(m.View(), sampleSecret) {
		t.Fatal("explicit reveal changed secrecy or failed")
	}
	m, _ = dialogKey(m, tea.KeyCtrlR)
	if !m.editor.secret || strings.Contains(m.View(), sampleSecret) {
		t.Fatal("hide changed secrecy or leaked value")
	}
	m = update(t, m, "esc")
	if strings.Contains(m.View(), sampleSecret) {
		t.Fatal("cancel retained revealed value")
	}
}

func TestBrowserLongUnicodeRowsFitInOneLine(t *testing.T) {
	m := editable(mustModel(t, []Project{{Name: strings.Repeat("密钥🙂", 40), Services: []Service{{Name: "service", Fields: []Field{{Name: strings.Repeat("字段🙂", 40), Value: strings.Repeat("密钥🙂", 100), Secret: true}}}}}}, "", nil)).resize(40, 20)
	m = update(t, m, "enter")
	m = update(t, m, "enter")
	m = update(t, m, "s")
	assertDialogBounds(t, m.View(), [2]int{40, 20})
	for _, item := range m.active().Items() {
		var row strings.Builder
		m.rows.Render(&row, m.active(), 0, item)
		if lipgloss.Width(row.String()) > 36 || lipgloss.Height(row.String()) != 1 {
			t.Fatalf("browser row escaped viewport: %s", row.String())
		}
	}
}
