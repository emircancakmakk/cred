package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

const sampleSecret = "SUPER-SECRET-VALUE"

func TestProjectListShowsCounts(t *testing.T) {
	m := mustModel(t, sampleProjects(), "", &fakeCopier{})
	view := m.View()
	if strings.Contains(view, sampleSecret) || strings.Contains(view, "DO-SECRET") {
		t.Fatalf("project list leaked a secret:\n%s", view)
	}
	for _, text := range []string{"filiz", "2 services", "other", "1 service", "enter select", "q quit"} {
		if !strings.Contains(view, text) {
			t.Fatalf("project list missing %q:\n%s", text, view)
		}
	}
	if selectedTitle(m) != "filiz" {
		t.Fatalf("selected %q", selectedTitle(m))
	}

	m = update(t, m, "enter")
	view = m.View()
	if !strings.Contains(view, "digital-ocean") || !strings.Contains(view, "vercel") {
		t.Fatalf("services:\n%s", view)
	}
	if strings.Contains(view, sampleSecret) {
		t.Fatalf("services leaked a secret:\n%s", view)
	}

	m = update(t, m, "esc")
	if m.mode != screenProjects || selectedTitle(m) != "filiz" {
		t.Fatalf("back mode %d selected %q", m.mode, selectedTitle(m))
	}
	_, cmd := step(m, "left")
	assertQuit(t, cmd)
	_, cmd = step(m, "q")
	assertQuit(t, cmd)
}

func TestSelectorMasksUntilShown(t *testing.T) {
	m := sample(t)
	view := m.View()
	if strings.Contains(view, sampleSecret) || strings.Contains(view, "DO-SECRET") {
		t.Fatalf("service list leaked a secret:\n%s", view)
	}
	if !strings.Contains(view, "digital-ocean") || !strings.Contains(view, "vercel") {
		t.Fatalf("services:\n%s", view)
	}

	m = update(t, m, "s")
	if strings.Contains(m.View(), "DO-SECRET") || strings.Contains(m.View(), sampleSecret) {
		t.Fatal("s on the service list revealed a field")
	}

	m = update(t, m, "enter")
	view = m.View()
	if !strings.Contains(view, "token") || !strings.Contains(view, secretMask) {
		t.Fatalf("field list:\n%s", view)
	}
	if strings.Contains(view, "DO-SECRET") || strings.Contains(view, sampleSecret) {
		t.Fatalf("field list leaked a secret:\n%s", view)
	}

	m = update(t, m, "esc")
	if m.mode != screenServices || selectedTitle(m) != "digital-ocean" {
		t.Fatalf("back selected %q mode %d", selectedTitle(m), m.mode)
	}
	if strings.Contains(m.View(), "DO-SECRET") {
		t.Fatal("secret stayed visible after going back")
	}

	m = update(t, m, "down")
	m = update(t, m, "enter")
	view = m.View()
	if !strings.Contains(view, "a@b.c") || !strings.Contains(view, secretMask) {
		t.Fatalf("vercel fields:\n%s", view)
	}
	if strings.Contains(view, sampleSecret) {
		t.Fatalf("password visible before s:\n%s", view)
	}

	before := m.reveal.id
	m = update(t, m, "s")
	if m.reveal.id != before {
		t.Fatal("s on a plain field started a reveal")
	}
	if strings.Contains(m.View(), sampleSecret) {
		t.Fatal("s on email revealed the password")
	}

	m = update(t, m, "j")
	m, cmd := step(m, "s")
	if !strings.Contains(m.View(), sampleSecret) {
		t.Fatalf("revealed:\n%s", m.View())
	}
	if cmd == nil {
		t.Fatal("expected a hide timer")
	}

	stale := m.reveal.id
	m = update(t, m, "s")
	next, _ := m.Update(hideReveal{id: stale})
	m = next.(model)
	if !strings.Contains(m.View(), sampleSecret) {
		t.Fatal("an older timer hid the newer reveal")
	}
	next, _ = m.Update(hideReveal{id: m.reveal.id})
	m = next.(model)
	if strings.Contains(m.View(), sampleSecret) || !strings.Contains(m.View(), secretMask) {
		t.Fatalf("hidden:\n%s", m.View())
	}
}

func TestCopyDoesNotPrintTheValue(t *testing.T) {
	m := openVercel(t)
	m = update(t, m, "j")
	fake := &fakeCopier{}
	m.copier = fake
	m = update(t, m, "enter")
	view := m.View()
	if fake.text != sampleSecret {
		t.Fatalf("copied %q", fake.text)
	}
	if !strings.Contains(view, "copied password") || strings.Contains(view, sampleSecret) {
		t.Fatalf("status:\n%s", view)
	}
	if m.lastCopied != "filiz/vercel/password" {
		t.Fatalf("label %q", m.lastCopied)
	}

	m.copier = &fakeCopier{err: errCopy}
	m = update(t, m, "enter")
	view = m.View()
	if !strings.Contains(view, "copy failed") || strings.Contains(view, sampleSecret) {
		t.Fatalf("failure:\n%s", view)
	}
}

func TestBackAndQuit(t *testing.T) {
	m := openVercel(t)
	_, cmd := step(m, "esc")
	if cmd != nil {
		t.Fatal("esc on fields should go back")
	}
	m = update(t, m, "esc")
	if m.mode != screenServices || selectedTitle(m) != "vercel" {
		t.Fatalf("cursor %q mode %d", selectedTitle(m), m.mode)
	}
	_, cmd = step(m, "q")
	assertQuit(t, cmd)

	m = update(t, m, "left")
	if m.mode != screenProjects {
		t.Fatalf("left mode %d", m.mode)
	}
	_, cmd = step(m, "esc")
	assertQuit(t, cmd)

	m = openVercel(t)
	_, cmd = step(m, "q")
	assertQuit(t, cmd)
	_, cmd = step(m, "ctrl+c")
	assertQuit(t, cmd)
}

func TestFilterHidesOtherRows(t *testing.T) {
	m := sample(t)
	m = update(t, m, "/")
	if !m.serviceList.SettingFilter() {
		t.Fatal("filter did not open")
	}
	m = update(t, m, "v")
	m = update(t, m, "e")
	m = update(t, m, "r")
	if m.serviceList.FilterValue() != "ver" {
		t.Fatalf("filter %q", m.serviceList.FilterValue())
	}
	m = update(t, m, "enter")
	view := m.View()
	if m.mode != screenServices {
		t.Fatalf("enter applied the filter by opening a service, mode %d", m.mode)
	}
	if !strings.Contains(view, "vercel") || strings.Contains(view, "digital-ocean") {
		t.Fatalf("filtered services:\n%s", view)
	}

	m = update(t, m, "enter")
	if m.mode != screenFields || !strings.Contains(m.View(), "a@b.c") {
		t.Fatalf("opened %q\n%s", selectedTitle(m), m.View())
	}
	item, ok := m.fieldList.Items()[1].(fieldItem)
	if !ok || item.FilterValue() != "password" || strings.Contains(item.FilterValue(), sampleSecret) {
		t.Fatalf("filter value %q", item.FilterValue())
	}
	m.fieldList.SetFilterText("SUPER")
	view = m.View()
	if strings.Contains(view, sampleSecret) || strings.Contains(view, "password") {
		t.Fatalf("secret text matched a field:\n%s", view)
	}
}

func TestEscCancelsFilter(t *testing.T) {
	m := sample(t)
	m = update(t, m, "/")
	m = update(t, m, "v")
	m = update(t, m, "esc")
	if m.serviceList.SettingFilter() || m.mode != screenServices {
		t.Fatalf("filter %v mode %d", m.serviceList.SettingFilter(), m.mode)
	}
	if !strings.Contains(m.View(), "digital-ocean") {
		t.Fatalf("filter cancel:\n%s", m.View())
	}
}

func TestEmptyLists(t *testing.T) {
	m := mustModel(t, nil, "", nil)
	if !strings.Contains(m.View(), "No projects") {
		t.Fatalf("projects:\n%s", m.View())
	}
	m = mustModel(t, []Project{{Name: "filiz"}}, "filiz", nil)
	if !strings.Contains(m.View(), "No services") {
		t.Fatalf("services:\n%s", m.View())
	}
	m = update(t, m, "enter")
	if strings.Contains(m.View(), "copy failed") {
		t.Fatal("enter on an empty service list tried to copy")
	}

	m = mustModel(t, []Project{{
		Name:     "filiz",
		Services: []Service{{Name: "vercel"}},
	}}, "filiz", nil)
	m = update(t, m, "enter")
	if !strings.Contains(m.View(), "No fields") {
		t.Fatalf("fields:\n%s", m.View())
	}
	m = update(t, m, "enter")
	if strings.Contains(m.View(), "copy failed") {
		t.Fatal("enter on an empty field list tried to copy")
	}
}

func TestLongPlainValueIsClipped(t *testing.T) {
	long := strings.Repeat("a", 50)
	m := mustModel(t, []Project{{
		Name: "filiz",
		Services: []Service{{
			Name:   "vercel",
			Fields: []Field{{Name: "host", Value: long, Secret: false}},
		}},
	}}, "filiz", nil)
	m = update(t, m, "enter")
	view := m.View()
	if strings.Contains(view, long) || !strings.Contains(view, "…") {
		t.Fatalf("plain value:\n%s", view)
	}
}

func TestWindowFollowsCursor(t *testing.T) {
	services := make([]Service, 8)
	for i := range services {
		services[i] = Service{Name: fmt.Sprintf("svc-%02d", i)}
	}
	m := mustModel(t, []Project{{Name: "filiz", Services: services}}, "filiz", nil)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 8})
	m = next.(model)
	for range 6 {
		m = update(t, m, "down")
	}
	view := m.View()
	if strings.Contains(view, "svc-00") {
		t.Fatalf("first row still visible:\n%s", view)
	}
	if selectedTitle(m) != "svc-06" {
		t.Fatalf("cursor %q\n%s", selectedTitle(m), view)
	}
}

func TestKeyBindingsMatch(t *testing.T) {
	if !key.Matches(keyMsg("k").(tea.KeyMsg), keyUp) || !key.Matches(keyMsg("j").(tea.KeyMsg), keyDown) {
		t.Fatal("j/k bindings")
	}
	if !key.Matches(keyMsg("left").(tea.KeyMsg), keyBack) || !key.Matches(keyMsg("/").(tea.KeyMsg), keyFilter) {
		t.Fatal("back or filter binding")
	}
}

func sampleProjects() []Project {
	return []Project{
		{
			Name: "filiz",
			Services: []Service{
				{
					Name: "vercel",
					Fields: []Field{
						{Name: "password", Value: sampleSecret, Secret: true},
						{Name: "email", Value: "a@b.c", Secret: false},
					},
				},
				{
					Name: "digital-ocean",
					Fields: []Field{
						{Name: "token", Value: "DO-SECRET", Secret: true},
					},
				},
			},
		},
		{
			Name: "other",
			Services: []Service{
				{Name: "github", Fields: []Field{{Name: "token", Value: "OTHER-SECRET", Secret: true}}},
			},
		},
	}
}

func sample(t *testing.T) model {
	t.Helper()
	return mustModel(t, sampleProjects(), "filiz", &fakeCopier{})
}

func openVercel(t *testing.T) model {
	t.Helper()
	m := sample(t)
	m = update(t, m, "down")
	return update(t, m, "enter")
}

func mustModel(t *testing.T, projects []Project, start string, copier Copier) model {
	t.Helper()
	m, err := newModel(projects, start, copier)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func update(t *testing.T, m model, name string) model {
	t.Helper()
	next, _ := step(m, name)
	return next
}

func step(m model, name string) (model, tea.Cmd) {
	filtering := m.active().SettingFilter() || name == "/"
	next, cmd := m.Update(keyMsg(name))
	m = next.(model)
	if filtering {
		m = absorbFilter(m, cmd)
	}
	return m, cmd
}

func absorbFilter(m model, cmd tea.Cmd) model {
	if cmd == nil {
		return m
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return applyFilter(m, msg)
	}
	for _, item := range batch {
		if item == nil {
			continue
		}
		m = applyFilter(m, runQuick(item))
	}
	return m
}

func applyFilter(m model, msg tea.Msg) model {
	if _, ok := msg.(list.FilterMatchesMsg); !ok {
		return m
	}
	next, _ := m.Update(msg)
	return next.(model)
}

func runQuick(cmd tea.Cmd) tea.Msg {
	ch := make(chan tea.Msg, 1)
	go func() { ch <- cmd() }()
	select {
	case msg := <-ch:
		return msg
	case <-time.After(40 * time.Millisecond):
		return nil
	}
}

func selectedTitle(m model) string {
	item := m.active().SelectedItem()
	titled, ok := item.(interface{ Title() string })
	if !ok {
		return ""
	}
	return titled.Title()
}

func keyMsg(name string) tea.Msg {
	switch name {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(name)}
	}
}

func assertQuit(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("quit message %T", cmd())
	}
}

type fakeCopier struct {
	text string
	err  error
}

func (f *fakeCopier) Copy(text string) error {
	f.text = text
	return f.err
}

var errCopy = errString("clipboard failed")

type errString string

func (e errString) Error() string { return string(e) }
