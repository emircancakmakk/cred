package tui

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func editable(m model) model {
	m.commit = func(Change) (Snapshot, error) { return Snapshot{}, nil }
	return m
}

func dialogKey(m model, kind tea.KeyType) (model, tea.Cmd) {
	next, cmd := m.Update(tea.KeyMsg{Type: kind})
	return next.(model), cmd
}

func TestCRUDShortcutsAtEveryLevel(t *testing.T) {
	for _, level := range []int{0, 1, 2} {
		m := editable(mustModel(t, sampleProjects(), "", nil))
		for range level {
			m = update(t, m, "enter")
		}
		kind := []Kind{ProjectKind, ServiceKind, FieldKind}[level]
		m = update(t, m, "n")
		if kind == FieldKind {
			for range 6 {
				m, _ = dialogKey(m, tea.KeyDown)
			}
			m, _ = dialogKey(m, tea.KeyEnter)
		}
		if m.editor == nil || m.editor.change.Kind != kind || !m.editor.secret {
			t.Fatalf("new %s form missing", kind)
		}
		for _, r := range "neqds" {
			m = update(t, m, string(r))
		}
		if m.editor.name.Value() != "neqds" {
			t.Fatal("form typing triggered browser shortcuts")
		}
		m = update(t, m, "esc")
		if m.mode != screen(level) || m.editor != nil {
			t.Fatal("cancel changed navigation")
		}
		m = update(t, m, "e")
		if m.editor == nil || m.editor.name.Value() == "" || m.editor.change.Action != Edit {
			t.Fatal("edit selection missing")
		}
		m = update(t, m, "esc")
		m = update(t, m, "d")
		if m.deleting == nil || m.deleting.change.Kind != kind || m.deleting.confirm {
			t.Fatal("delete must default to cancel")
		}
		m, cmd := dialogKey(m, tea.KeyEnter)
		if cmd != nil || m.deleting != nil || m.saving {
			t.Fatal("default enter deleted an item")
		}
	}
}

func TestFieldEditorMasksAndTogglesSecret(t *testing.T) {
	m := editable(openVercel(t))
	m = update(t, m, "j")
	m = update(t, m, "s")
	m = update(t, m, "e")
	if m.editor.value.Value() != sampleSecret || strings.Contains(m.View(), sampleSecret) || m.reveal.name != "" {
		t.Fatal("editor leaked or lost existing secret")
	}
	m, _ = dialogKey(m, tea.KeyTab)
	m, _ = dialogKey(m, tea.KeyTab)
	m = update(t, m, " ")
	if m.editor.secret || !strings.Contains(m.View(), sampleSecret) {
		t.Fatal("secret toggle not applied")
	}
	m = update(t, m, " ")
	if !m.editor.secret || strings.Contains(m.View(), sampleSecret) {
		t.Fatal("secret not masked again")
	}
	m, _ = dialogKey(m, tea.KeyShiftTab)
	if m.editor.focus != 1 {
		t.Fatal("reverse tab failed")
	}
	m = update(t, m, "esc")
	if strings.Contains(m.View(), sampleSecret) {
		t.Fatal("cancel retained reveal")
	}
}

func TestDeleteExplicitChoiceAndCounts(t *testing.T) {
	m := editable(mustModel(t, sampleProjects(), "", nil))
	m = update(t, m, "d")
	if !strings.Contains(m.View(), "2 services and 3 fields") {
		t.Fatalf("missing descendant counts: %s", m.View())
	}
	m, _ = dialogKey(m, tea.KeyRight)
	m, cmd := dialogKey(m, tea.KeyEnter)
	if cmd == nil || !m.saving {
		t.Fatal("explicit delete did not start save")
	}
	m = m.finishSave(saved{change: m.deleting.change, err: errors.New("disk full")})
	if m.deleting == nil || !strings.Contains(m.View(), "disk full") || len(m.projects) != 2 {
		t.Fatal("failed delete changed snapshot")
	}
	m = update(t, m, "esc")
	if m.deleting != nil {
		t.Fatal("delete cancel failed")
	}
}

func TestSaveFailurePreservesInputsAndSuccessRestoresContext(t *testing.T) {
	m := editable(openVercel(t))
	m = update(t, m, "e")
	m.editor.name.SetValue("renamed")
	m.editor.value.SetValue("replacement")
	for range m.editor.saveFocus() {
		m, _ = dialogKey(m, tea.KeyTab)
	}
	m, cmd := dialogKey(m, tea.KeyEnter)
	if cmd == nil || !m.saving {
		t.Fatal("save not started")
	}
	c := m.editor.change
	c.NewName, c.Value, c.Secret = "renamed", "replacement", true
	m = m.finishSave(saved{change: c, err: errors.New("write failed")})
	if m.saving || m.editor.name.Value() != "renamed" || m.editor.value.Value() != "replacement" || !strings.Contains(m.View(), "write failed") {
		t.Fatal("failed save lost inputs")
	}
	if selectedTitle(m) != "email" {
		t.Fatal("failed save changed list")
	}
	projects := sampleProjects()
	for i := range projects[0].Services {
		if projects[0].Services[i].Name == "vercel" {
			projects[0].Services[i].Fields[1] = Field{Name: "renamed", Value: "replacement", Secret: true}
		}
	}
	m = m.finishSave(saved{change: c, snapshot: Snapshot{Projects: projects}})
	if m.editor != nil || m.mode != screenFields || selectedTitle(m) != "renamed" || m.crumb() != "filiz  /  vercel" {
		t.Fatalf("wrong restored context %s %s", m.crumb(), selectedTitle(m))
	}
	if strings.Contains(m.View(), "replacement") {
		t.Fatal("saved secret visible")
	}
}

func TestEmptyListCreationAndValidation(t *testing.T) {
	m := editable(mustModel(t, nil, "", nil))
	m = update(t, m, "e")
	m = update(t, m, "d")
	if m.editor != nil || m.deleting != nil {
		t.Fatal("empty selection allowed edit/delete")
	}
	m = update(t, m, "n")
	m, _ = dialogKey(m, tea.KeyEnter)
	m, _ = dialogKey(m, tea.KeyEnter)
	if m.saving || m.editor.focus != 0 || !strings.Contains(m.View(), "Name is required") {
		t.Fatal("blank name saved")
	}
	m.editor.name.SetValue("first")
	c := Change{Action: Create, Kind: ProjectKind, NewName: "first"}
	m = m.finishSave(saved{change: c, snapshot: Snapshot{Projects: []Project{{Name: "first"}}}})
	if selectedTitle(m) != "first" {
		t.Fatal("new project not selected")
	}
	m = update(t, m, "enter")
	m = update(t, m, "n")
	if m.editor.change.Kind != ServiceKind || m.editor.change.Project != "first" {
		t.Fatal("empty project cannot create service")
	}
}

func TestDeleteLastItemKeepsParentList(t *testing.T) {
	m := editable(mustModel(t, []Project{{Name: "p", Services: []Service{{Name: "s", Fields: []Field{{Name: "f"}}}}}}, "p", nil))
	m = update(t, m, "enter")
	m = update(t, m, "d")
	m = m.finishSave(saved{change: m.deleting.change, snapshot: Snapshot{Projects: []Project{{Name: "p", Services: []Service{{Name: "s"}}}}}})
	if m.mode != screenFields || !strings.Contains(m.View(), "No fields") {
		t.Fatal("delete last field lost parent context")
	}
	m = update(t, m, "n")
	if m.editor == nil || m.editor.change.Kind != FieldKind {
		t.Fatal("cannot create after deleting last field")
	}
}

// Run the actual Bubble Tea Exec lifecycle as well as individual model updates.
type exitAfterSave struct{ model }

func (m exitAfterSave) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.model.Update(msg)
	out := exitAfterSave{next.(model)}
	if _, ok := msg.(saved); ok {
		return out, tea.Quit
	}
	return out, cmd
}

func TestProgramExecPersistsAndReturnsToBrowser(t *testing.T) {
	m := mustModel(t, nil, "", nil)
	var received Change
	m.commit = func(c Change) (Snapshot, error) {
		received = c
		return Snapshot{Projects: []Project{{Name: c.NewName}}}, nil
	}
	p := tea.NewProgram(exitAfterSave{m}, tea.WithInput(strings.NewReader("")), tea.WithOutput(io.Discard), tea.WithoutRenderer())
	type outcome struct {
		model tea.Model
		err   error
	}
	done := make(chan outcome, 1)
	go func() { final, err := p.Run(); done <- outcome{final, err} }()
	go func() { p.Send(keyMsg("n")); p.Send(keyMsg("first")); p.Send(keyMsg("enter")); p.Send(keyMsg("enter")) }()
	select {
	case result := <-done:
		if result.err != nil {
			t.Fatal(result.err)
		}
		final := result.model.(exitAfterSave).model
		if received.Action != Create || received.NewName != "first" || final.editor != nil || selectedTitle(final) != "first" {
			t.Fatalf("save lifecycle failed: %#v, %s", received, final.View())
		}
	case <-time.After(5 * time.Second):
		p.Kill()
		t.Fatal("save lifecycle did not complete")
	}
}

func TestEditingNamePreservesImportedValue(t *testing.T) {
	original := "first line\nsecond line\tvalue"
	m := editable(mustModel(t, []Project{{Name: "p", Services: []Service{{Name: "s", Fields: []Field{{Name: "key", Value: original, Secret: true}}}}}}, "p", nil))
	m = update(t, m, "enter")
	m = update(t, m, "e")
	m.editor.name.SetValue("renamed")
	var received Change
	m.commit = func(c Change) (Snapshot, error) { received = c; return Snapshot{}, nil }
	p := tea.NewProgram(exitAfterSave{m}, tea.WithInput(strings.NewReader("")), tea.WithOutput(io.Discard), tea.WithoutRenderer())
	done := make(chan error, 1)
	go func() { _, err := p.Run(); done <- err }()
	go func() {
		for range m.editor.saveFocus() {
			p.Send(tea.KeyMsg{Type: tea.KeyTab})
		}
		p.Send(keyMsg("enter"))
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
		if received.Value != original || received.NewName != "renamed" {
			t.Fatal("renaming changed original imported value")
		}
	case <-time.After(5 * time.Second):
		p.Kill()
		t.Fatal("save did not complete")
	}
}

func TestEditableFilterUsesSelectedTarget(t *testing.T) {
	m := editable(sample(t))
	m = update(t, m, "/")
	for _, r := range "vercel" {
		m = update(t, m, string(r))
	}
	if m.editor != nil || m.deleting != nil {
		t.Fatal("filter text triggered CRUD")
	}
	m = update(t, m, "enter")
	m = update(t, m, "e")
	if m.editor == nil || m.editor.change.Name != "vercel" {
		t.Fatal("edit targeted wrong filtered service")
	}
	m = update(t, m, "esc")
	m = update(t, m, "d")
	if m.deleting == nil || m.deleting.change.Name != "vercel" {
		t.Fatal("delete targeted wrong filtered service")
	}
}

func TestNewFieldPicker(t *testing.T) {
	for i, name := range []string{"email", "password", "username", "url", "api_key", "token", ""} {
		m := editable(openVercel(t))
		m = update(t, m, "n")
		if m.editor.picker == nil {
			t.Fatal("new field picker missing")
		}
		for range i {
			m, _ = dialogKey(m, tea.KeyTab)
		}
		m, _ = dialogKey(m, tea.KeyEnter)
		e := m.editor
		if e.picker != nil || e.name.Value() != name || !e.secret {
			t.Fatal("incorrect preset")
		}
		if name == "" {
			if e.focus != 0 {
				t.Fatal("custom name not focused")
			}
			e.name.SetValue("   ")
			for range 4 {
				m, _ = dialogKey(m, tea.KeyEnter)
			}
			if m.saving || !strings.Contains(m.View(), "Name is required") {
				t.Fatal("blank name saved")
			}
		} else if e.focus != 1 || !e.value.Focused() {
			t.Fatal("preset must focus value")
		}
		// The chosen name remains editable and is used by the save request.
		savedName := "custom-key"
		if i >= 3 && name != "" {
			savedName = name
		}
		e.name.SetValue(savedName)
		e.value.SetValue("picker-secret")
		if strings.Contains(m.View(), "picker-secret") {
			t.Fatal("new secret leaked")
		}
		e.setFocus(e.saveFocus())
		var received Change
		m.commit = func(c Change) (Snapshot, error) {
			received = c
			return Snapshot{}, errors.New("duplicate field")
		}
		p := tea.NewProgram(exitAfterSave{m}, tea.WithInput(strings.NewReader("")), tea.WithOutput(io.Discard), tea.WithoutRenderer())
		done := make(chan error, 1)
		go func() { _, err := p.Run(); done <- err }()
		go p.Send(keyMsg("enter"))
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
			if received.NewName != savedName || received.Value != "picker-secret" || !received.Secret {
				t.Fatalf("wrong field sent to persistence: %#v", received)
			}
			if m.editor.name.Value() != savedName || !strings.Contains(m.View(), "duplicate field") {
				t.Fatal("save failure lost chosen name")
			}
		case <-time.After(5 * time.Second):
			p.Kill()
			t.Fatal("preset save did not complete")
		}
	}
}

func TestFieldPickerCancellationAndBounds(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {40, 20}, {120, 30}} {
		m := editable(openVercel(t)).resize(size[0], size[1])
		m = update(t, m, "n")
		assertDialogBounds(t, m.View(), size)
		if !strings.Contains(m.View(), "Custom field") {
			t.Fatal("custom option missing")
		}
		m = update(t, m, "esc")
		if m.editor != nil || m.mode != screenFields || m.saving {
			t.Fatal("cancel changed browser")
		}
	}
}
