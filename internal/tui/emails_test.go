package tui

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func emailModel(t *testing.T) model {
	t.Helper()
	m := editable(openVercel(t))
	m.emails = []SavedEmail{{ID: "one", Address: "personal@example.com"}, {ID: "two", Address: "work@example.com"}}
	return m
}
func emailKey(m model, name string) model {
	filtering := name == "/" || (m.manager != nil && m.manager.list.SettingFilter()) || (m.editor != nil && m.editor.emailPicker != nil && m.editor.emailPicker.list.SettingFilter())
	next, cmd := m.Update(keyMsg(name))
	m = next.(model)
	if filtering {
		m = absorbFilter(m, cmd)
	}
	return m
}
func emailReview(t *testing.T, m model, raw string) model {
	t.Helper()
	m = emailKey(m, "n")
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(raw), Paste: true})
	m = next.(model)
	m, _ = dialogKey(m, tea.KeyTab)
	return emailKey(m, "enter")
}

func TestEmailManagementNavigationAndBatchReview(t *testing.T) {
	for _, level := range []int{0, 1, 2} {
		m := editable(mustModel(t, sampleProjects(), "", nil))
		for range level {
			m = emailKey(m, "enter")
		}
		mode, selected := m.mode, selectedTitle(m)
		if !strings.Contains(m.View(), "m emails") {
			t.Fatal("management is not discoverable")
		}
		m = emailKey(m, "m")
		if !strings.Contains(m.View(), "No saved email") {
			t.Fatal("missing empty state")
		}
		for _, key := range []string{"e", "d"} {
			m = emailKey(m, key)
			if m.manager.single != nil || m.manager.deleting != nil {
				t.Fatal("empty collection allowed edit/delete")
			}
		}
		raw := "first@example.com\nSECOND@EXAMPLE.COM;FIRST@example.com,invalid address"
		m = emailReview(t, m, raw)
		b := m.manager.bulk
		if !b.review || len(b.batch.Added) != 2 || len(b.batch.Skipped) != 1 || len(b.batch.Invalid) != 1 {
			t.Fatalf("wrong review: %+v", b.batch)
		}
		m = emailKey(m, "enter")
		if m.saving || m.manager.err == "" {
			t.Fatal("invalid batch saved")
		}
		m, _ = dialogKey(m, tea.KeyTab)
		m = emailKey(m, "enter")
		if b.review || b.input.Value() != raw {
			t.Fatal("Back lost raw draft")
		}
		m = emailKey(m, "esc")
		if m.manager.bulk != nil || len(m.emails) != 0 {
			t.Fatal("cancel changed collection")
		}
		m = emailKey(m, "esc")
		if m.manager != nil || m.mode != mode || selectedTitle(m) != selected {
			t.Fatal("management changed browser location")
		}
	}
}
func TestEmailBatchEmptyDuplicatesAndNewline(t *testing.T) {
	for _, raw := range []string{"", " ;,\n", "PERSONAL@example.com;work@example.com"} {
		m := emailKey(emailModel(t), "m")
		before := append([]SavedEmail(nil), m.emails...)
		m = emailReview(t, m, raw)
		m = emailKey(m, "enter")
		if m.saving || !strings.Contains(m.manager.err, "No new addresses") || !reflect.DeepEqual(before, m.emails) {
			t.Fatal("empty/duplicate batch saved")
		}
		m, _ = dialogKey(m, tea.KeyTab)
		m, _ = dialogKey(m, tea.KeyTab)
		m = emailKey(m, "enter")
		if m.manager.bulk != nil {
			t.Fatal("Cancel button did not cancel review")
		}
	}
	m := emailKey(emailKey(emailModel(t), "m"), "n")
	m.manager.bulk.input.SetValue("first@example.com")
	m = emailKey(m, "enter")
	if m.manager.bulk.review || m.manager.bulk.input.Value() != "first@example.com\n" {
		t.Fatal("Enter did not insert newline")
	}
	// Pasting more than the widget's default 400-character limit retains every entry.
	raw := strings.Repeat("person@example.com\n", 150) + "last@example.com"
	m.manager.bulk.input.SetValue(raw)
	if m.manager.bulk.input.Value() != raw {
		t.Fatal("large batch was truncated")
	}
}
func TestSavedEmailEditingDeletionAndFailures(t *testing.T) {
	m := emailKey(emailModel(t), "m")
	m = emailKey(m, "e")
	m.manager.single.Inputs[0].SetValue("WORK@example.com")
	m = emailKey(emailKey(m, "enter"), "enter")
	if m.saving || !strings.Contains(m.View(), "already saved") {
		t.Fatal("duplicate edit accepted")
	}
	m.manager.single.Inputs[0].SetValue("invalid")
	m = emailKey(m, "enter")
	if m.saving || !strings.Contains(m.View(), "plain email") {
		t.Fatal("invalid edit accepted")
	}
	m.manager.single.Inputs[0].SetValue("replacement@example.com")
	m, cmd := dialogKey(m, tea.KeyEnter)
	if !m.saving || cmd == nil {
		t.Fatal("valid edit did not save")
	}
	c := Change{Kind: EmailKind, Action: Edit, Name: "one", Value: "replacement@example.com"}
	m = m.finishSave(saved{change: c, err: errors.New("disk full")})
	if m.manager.single.Accepted || m.manager.single.Values()[0] != "replacement@example.com" || !strings.Contains(m.View(), "disk full") {
		t.Fatal("save error lost edit")
	}
	m = emailKey(m, "esc")
	if m.manager.single != nil || m.emails[0].Address != "personal@example.com" {
		t.Fatal("cancel edited collection")
	}
	m = emailKey(m, "d")
	if m.manager.confirm || !strings.Contains(m.View(), "Existing credential fields") {
		t.Fatal("unsafe delete default")
	}
	m = emailKey(m, "enter")
	if m.manager.deleting != nil || m.saving {
		t.Fatal("default Enter deleted address")
	}
	m = emailKey(m, "d")
	m, _ = dialogKey(m, tea.KeyRight)
	m, cmd = dialogKey(m, tea.KeyEnter)
	if !m.saving || cmd == nil {
		t.Fatal("confirmed delete did not save")
	}
	m = m.finishSave(saved{change: Change{Kind: EmailKind, Action: Delete, Name: "one"}, err: errors.New("write failed")})
	if m.manager.deleting == nil || len(m.emails) != 2 {
		t.Fatal("failed delete changed collection")
	}
	m = emailKey(m, "esc")
}
func TestSavedEmailPickerCreateEditManualAndCancel(t *testing.T) {
	for _, action := range []string{"n", "e"} {
		m := emailKey(emailModel(t), action)
		if action == "n" {
			m = emailKey(m, "enter")
		}
		e := m.editor
		e.name.SetValue("EMAIL")
		e.value.SetValue("draft@example.com")
		e.setFocus(1)
		secret := e.secret
		m, _ = dialogKey(m, tea.KeyCtrlE)
		if e.emailPicker == nil {
			t.Fatal("email picker missing")
		}
		m = emailKey(m, "esc")
		if e.emailPicker != nil || e.value.Value() != "draft@example.com" || e.focus != 1 || e.secret != secret {
			t.Fatal("picker cancel changed draft")
		}
		e.setFocus(4)
		m = emailKey(m, "enter")
		m = emailKey(m, "/")
		for _, r := range "work" {
			m = emailKey(m, string(r))
		}
		m = emailKey(m, "enter")
		if len(e.emailPicker.list.VisibleItems()) != 1 {
			t.Fatal("picker search did not filter")
		}
		m = emailKey(m, "enter")
		if e.emailPicker != nil || e.value.Value() != "work@example.com" || e.focus != 1 || e.secret != secret || m.saving {
			t.Fatal("selection did not populate reviewable value")
		}
		e.value.SetValue("manual@example.com")
		e.setFocus(e.saveFocus())
		_, received := executeEmailSave(t, m)
		if received.Value != "manual@example.com" || received.NewName != "EMAIL" || received.Secret != secret {
			t.Fatal("manual override did not reach persistence")
		}
	}
	m := emailKey(emailModel(t), "e")
	m.emails = nil
	m, _ = dialogKey(m, tea.KeyCtrlE)
	if !strings.Contains(m.View(), "No saved email") {
		t.Fatal("picker empty state missing")
	}
	m = emailKey(m, "enter")
	m = emailKey(m, "esc")
	if m.editor == nil || m.editor.emailPicker != nil {
		t.Fatal("empty picker cancelled editor")
	}
	m.editor.name.SetValue("username")
	m, _ = dialogKey(m, tea.KeyCtrlE)
	if m.editor.emailPicker != nil || strings.Contains(m.View(), "Choose saved email") {
		t.Fatal("non-email field offers picker")
	}
}
func TestEmailManagerFilterAndSaveRefresh(t *testing.T) {
	m := emailKey(emailModel(t), "m")
	m = emailKey(m, "/")
	for _, r := range "work" {
		m = emailKey(m, string(r))
	}
	m = emailKey(m, "enter")
	m = emailKey(m, "e")
	if m.manager.editingID != "two" {
		t.Fatal("edit ignored filtered selection")
	}
	projects := sampleProjects()
	projects[0].Services[0].Fields = append(projects[0].Services[0].Fields, Field{Name: "remote", Value: "team"})
	snapshot := Snapshot{Projects: projects, Emails: []SavedEmail{{ID: "two", Address: "changed@example.com"}}}
	m = m.finishSave(saved{change: Change{Kind: EmailKind, Action: Edit, Name: "two"}, snapshot: snapshot})
	if len(m.emails) != 1 || m.manager == nil || m.manager.single != nil || m.manager.list.SelectedItem().(emailItem).ID != "two" {
		t.Fatal("save did not refresh collection")
	}
	m = emailKey(m, "esc")
	if m.mode != screenFields || m.crumb() != "filiz  /  vercel" || selectedTitle(m) != "email" {
		t.Fatal("save lost browser context")
	}
	if len(m.projects[0].Services[1].Fields) != 3 {
		t.Fatal("save lost concurrent project changes")
	}
}

func TestEmailScreensBoundsAndScroll(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {40, 20}, {120, 30}, {24, 16}} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			m := emailModel(t).resize(size[0], size[1])
			for i := range 40 {
				m.emails = append(m.emails, SavedEmail{ID: fmt.Sprint(i), Address: fmt.Sprintf("%s%d@example.com", strings.Repeat("long", 15), i)})
			}
			m = emailKey(m, "m")
			for range len(m.emails) - 1 {
				_ = m.View()
				m = emailKey(m, "down")
			}
			view := ansi.Strip(m.View())
			assertDialogBounds(t, view, size)
			if !strings.Contains(view, "> ") {
				t.Fatalf("selected row not visible:\n%s", view)
			}
			m = emailKey(m, "d")
			assertDialogBounds(t, m.View(), size)
			m = emailKey(m, "esc")
			m = emailReview(t, m, strings.Repeat("long@example.com\n", 30)+"invalid")
			assertDialogBounds(t, m.View(), size)
			for range 30 {
				m = emailKey(m, "pgup")
				_ = m.View()
			}
			if !strings.Contains(m.View(), "to add") {
				t.Fatalf("review summary not reachable:\n%s", m.View())
			}
			for range 30 {
				m = emailKey(m, "pgdown")
				_ = m.View()
			}
			assertDialogBounds(t, m.View(), size)
			if !strings.Contains(m.View(), "Cancel") {
				t.Fatal("review actions not reachable")
			}
			m = emailKey(m, "esc")
			m = emailKey(m, "esc")
			m = emailKey(m, "e")
			m, _ = dialogKey(m, tea.KeyCtrlE)
			assertDialogBounds(t, m.View(), size)
			m = emailKey(m, "esc")
			if m.editor.emailPicker != nil {
				t.Fatal("narrow picker could not cancel")
			}
		})
	}
	for _, size := range [][2]int{{10, 4}, {20, 9}} {
		m := emailModel(t).resize(size[0], size[1])
		m = emailKey(m, "m")
		assertDialogBounds(t, m.View(), size)
		m = emailKey(m, "n")
		assertDialogBounds(t, m.View(), size)
		m = emailKey(m, "esc")
		m = emailKey(m, "esc")
		if m.manager != nil {
			t.Fatal("tiny terminal blocks cancellation")
		}
	}
}

// Exercise Bubble Tea's Exec release/restore lifecycle for batch saving.
func TestEmailBatchProgramExec(t *testing.T) {
	m := emailKey(emailModel(t), "m")
	raw := "new@example.com;other@example.com"
	m = emailReview(t, m, raw)
	var received Change
	m.commit = func(c Change) (Snapshot, error) {
		received = c
		return Snapshot{Projects: sampleProjects(), Emails: []SavedEmail{{ID: "new", Address: "new@example.com"}, {ID: "other", Address: "other@example.com"}}}, nil
	}
	p := tea.NewProgram(exitAfterSave{m}, tea.WithInput(strings.NewReader("")), tea.WithOutput(io.Discard), tea.WithoutRenderer())
	done := make(chan error, 1)
	go func() {
		final, err := p.Run()
		if err == nil {
			m = final.(exitAfterSave).model
		}
		done <- err
	}()
	go p.Send(keyMsg("enter"))
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
		if received.Kind != EmailKind || received.Action != Create || received.Value != raw || m.manager.bulk != nil || len(m.emails) != 2 {
			t.Fatal("batch save lifecycle failed")
		}
	case <-time.After(5 * time.Second):
		p.Kill()
		t.Fatal("batch save did not complete")
	}
}

func executeEmailSave(t *testing.T, m model) (model, Change) {
	t.Helper()
	var received Change
	m.commit = func(c Change) (Snapshot, error) {
		received = c
		return Snapshot{Projects: sampleProjects(), Emails: m.emails}, nil
	}
	p := tea.NewProgram(exitAfterSave{m}, tea.WithInput(strings.NewReader("")), tea.WithOutput(io.Discard), tea.WithoutRenderer())
	type outcome struct {
		model tea.Model
		err   error
	}
	done := make(chan outcome, 1)
	go func() { final, err := p.Run(); done <- outcome{final, err} }()
	go p.Send(keyMsg("enter"))
	select {
	case result := <-done:
		if result.err != nil {
			t.Fatal(result.err)
		}
		return result.model.(exitAfterSave).model, received
	case <-time.After(5 * time.Second):
		p.Kill()
		t.Fatal("save did not complete")
		return m, received
	}
}

func TestSelectedEmailReachesPersistenceAndDoesNotLink(t *testing.T) {
	for _, action := range []string{"n", "e"} {
		m := emailKey(emailModel(t), action)
		if action == "n" {
			m = emailKey(m, "enter")
		}
		m, _ = dialogKey(m, tea.KeyCtrlE)
		m = emailKey(m, "down")
		m = emailKey(m, "enter")
		m.editor.setFocus(m.editor.saveFocus())
		_, received := executeEmailSave(t, m)
		if received.Kind != FieldKind || received.Value != "work@example.com" {
			t.Fatal("selection did not reach persistence")
		}
		m.emails[1].Address = "renamed@example.com"
		if received.Value != "work@example.com" {
			t.Fatal("field retained a reference to collection")
		}
	}
}

func TestEmailBatchSaveErrorPreservesReviewAndInput(t *testing.T) {
	m := emailKey(emailModel(t), "m")
	raw := "new@example.com;other@example.com"
	m = emailReview(t, m, raw)
	m, cmd := dialogKey(m, tea.KeyEnter)
	if !m.saving || cmd == nil {
		t.Fatal("batch save did not start")
	}
	m = m.finishSave(saved{change: Change{Kind: EmailKind, Action: Create}, err: errors.New("save failed")})
	if m.saving || !m.manager.bulk.review || m.manager.bulk.input.Value() != raw || len(m.emails) != 2 || !strings.Contains(m.View(), "save failed") {
		t.Fatal("batch save failure lost draft")
	}
	m = emailKey(m, "esc")
	if m.manager.bulk != nil || m.manager == nil {
		t.Fatal("failed batch could not cancel")
	}
}

func TestOversizedEmailPasteRetainsPreviousDraft(t *testing.T) {
	m := emailKey(emailKey(emailModel(t), "m"), "n")
	m.manager.bulk.input.SetValue("keep@example.com")
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(strings.Repeat("person@example.com\n", 10001)), Paste: true})
	m = next.(model)
	if m.manager.bulk.input.Value() != "keep@example.com" || !strings.Contains(m.manager.err, "9,999 lines") || m.saving {
		t.Fatal("oversized paste silently truncated the draft")
	}
}
