package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/emircancakmakk/cred/internal/profiles"
	"github.com/emircancakmakk/cred/internal/terminalui"
	"github.com/emircancakmakk/cred/internal/vault"
)

func wizardKey(m *setupWizard, key tea.KeyType) tea.Cmd {
	_, cmd := m.Update(tea.KeyMsg{Type: key})
	return cmd
}
func wizardText(m *setupWizard, text string) {
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text), Paste: true})
}
func wizardSubmit(m *setupWizard) tea.Cmd {
	m.form.Focus = len(m.form.Inputs)
	return wizardKey(m, tea.KeyEnter)
}
func TestSetupWizardBackAndCancelBeforeCommit(t *testing.T) {
	isolateProfiles(t)
	m := newSetupWizard(&App{}, setupRequest{})
	wizardKey(m, tea.KeyEnter)
	m.form.Inputs[0].SetValue("personal")
	m.form.Focus = len(m.form.Inputs) + 1
	wizardKey(m, tea.KeyEnter)
	if m.step != 0 {
		t.Fatal("Back did not return to type selection")
	}
	wizardKey(m, tea.KeyEnter)
	if m.form.Values()[0] != "personal" {
		t.Fatal("Back lost name")
	}
	wizardSubmit(m)
	wizardText(m, masterPassword)
	wizardKey(m, tea.KeyEnter)
	wizardText(m, masterPassword)
	wizardSubmit(m)
	if m.step != 3 || strings.Contains(m.View(), masterPassword) {
		t.Fatal("review missing or leaked password")
	}
	m.form.Focus = 1
	wizardKey(m, tea.KeyEnter)
	if m.step != 2 || m.password != nil || m.form.Values()[0] != "" {
		t.Fatal("Back retained password")
	}
	wizardKey(m, tea.KeyEsc)
	dir, _ := profiles.Dir()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("cancel before review committed setup")
	}
}
func driveWizard(t *testing.T, m *setupWizard, cmd tea.Cmd, answer string) {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for cmd != nil {
		ready := make(chan tea.Msg, 1)
		go func(c tea.Cmd) { ready <- c() }(cmd)
		var msg tea.Msg
		select {
		case msg = <-ready:
		case <-deadline:
			t.Fatal("wizard operation timed out")
		}
		_, cmd = m.Update(msg)
		if m.question != nil && m.form != nil {
			wizardText(m, answer)
			cmd = wizardSubmit(m)
		}
		if !m.busy {
			return
		}
	}
}
func TestSetupWizardCreatesLocalAndSelectsDefault(t *testing.T) {
	isolateProfiles(t)
	app := &App{Out: io.Discard, Err: io.Discard, WorkFactor: 1}
	m := newSetupWizard(app, setupRequest{Fixed: true, Name: "personal"})
	if m.step != 2 {
		t.Fatal("shortcut asked for known details")
	}
	wizardText(m, masterPassword)
	wizardKey(m, tea.KeyEnter)
	wizardText(m, masterPassword)
	wizardSubmit(m)
	config, _ := profiles.Load()
	if len(config.Vaults) != 0 {
		t.Fatal("review wrote configuration")
	}
	driveWizard(t, m, wizardSubmit(m), "")
	if m.err != nil || m.step != 5 {
		t.Fatalf("setup failed: %v", m.err)
	}
	config, _ = profiles.Load()
	profile, ok := config.Vaults["personal"]
	if !ok {
		t.Fatal("local vault not registered")
	}
	if _, err := vault.Load(profile.Path, []byte(masterPassword)); err != nil {
		t.Fatal(err)
	}
	m.selection.Index = 1
	driveWizard(t, m, wizardKey(m, tea.KeyEnter), "")
	config, _ = profiles.Load()
	if config.Default != "personal" {
		t.Fatal("default not selected")
	}
}
func TestSetupWizardConnectRetriesInsideSameProgram(t *testing.T) {
	isolateProfiles(t)
	repo := gitFixture(t)
	app := &App{Out: io.Discard, Err: io.Discard, WorkFactor: 1}
	m := newSetupWizard(app, setupRequest{Choice: setupConnect, Fixed: true, Name: "team", Repo: repo})
	wizardText(m, "wrong-password")
	wizardSubmit(m)
	driveWizard(t, m, wizardSubmit(m), masterPassword)
	if m.err != nil || m.step != 5 {
		t.Fatalf("connect failed: %v", m.err)
	}
	config, _ := profiles.Load()
	if config.Vaults["team"].Type != "git" {
		t.Fatal("team vault not registered")
	}
}
func TestSetupWizardSharedAndDefaultFailurePreserveVault(t *testing.T) {
	isolateProfiles(t)
	repo := emptyRemote(t)
	app := &App{Out: io.Discard, Err: io.Discard, WorkFactor: 1}
	m := newSetupWizard(app, setupRequest{Choice: setupShared, Fixed: true, Name: "team", Repo: repo})
	wizardText(m, masterPassword)
	wizardKey(m, tea.KeyEnter)
	wizardText(m, masterPassword)
	wizardSubmit(m)
	driveWizard(t, m, wizardSubmit(m), "")
	if m.err != nil || m.step != 5 {
		t.Fatalf("shared setup failed: %v", m.err)
	}
	config, _ := profiles.Load()
	profile := config.Vaults["team"]
	m.Update(wizardResult{profile: profile, err: errors.New("config unavailable"), defaultOnly: true})
	if !strings.Contains(m.View(), "default selection failed") {
		t.Fatal("default failure missing")
	}
	if _, err := vault.Load(profile.RepoDir()+"/vault.age", []byte(masterPassword)); err != nil {
		t.Fatal("default failure lost vault")
	}
}
func TestSetupWizardCancelsOutstandingQuestionSafely(t *testing.T) {
	m := newSetupWizard(&App{}, setupRequest{Fixed: true, Name: "personal"})
	m.busy = true
	spec := terminalui.FormSpec{Title: "Password", Fields: []terminalui.Field{{Label: "Password", Sensitive: true}}}
	reply := make(chan wizardAnswer, 1)
	m.Update(wizardQuestion{form: &spec, reply: reply})
	cmd := wizardKey(m, tea.KeyCtrlC)
	if cmd == nil {
		t.Fatal("cancel stopped listening for worker cleanup")
	}
	answer := <-reply
	if !errors.Is(answer.err, terminalui.ErrCancelled) {
		t.Fatal("worker not notified")
	}
	m.Update(wizardResult{err: terminalui.ErrCancelled})
	if !m.done {
		t.Fatal("cleanup did not finish cancellation")
	}
}

func TestTerminalInterfaceHelper(t *testing.T) {
	if os.Getenv("CRED_TEST_TERMINAL") != "1" {
		t.Skip("PTY subprocess helper")
	}
	isolateProfiles(t)
	app := NewApp(os.Stdin, os.Stdout, os.Stderr)
	app.WorkFactor = 1
	app.Copier = &fakeCopier{}
	if err := app.Execute([]string{"init"}); err != nil {
		t.Fatal(err)
	}
	if err := app.Execute([]string{"--vault", "ptyvault"}); err != nil {
		t.Fatal(err)
	}
	fmt.Fprintln(os.Stdout, "PTY_DONE")
}

func TestTerminalInterfacePTY(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PTY test uses Unix terminal APIs")
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is required for PTY verification")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, python, "testdata/terminal_smoke.py", os.Args[0])
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("terminal smoke: %v\n%s", err, output)
	}
}

func TestSetupWizardResultAndFailureCancellationKeepOutcome(t *testing.T) {
	m := newSetupWizard(&App{}, setupRequest{Fixed: true, Name: "personal"})
	m.Update(wizardResult{profile: profiles.Profile{Type: "local", Path: "/tmp/example.age"}})
	wizardKey(m, tea.KeyEsc)
	if !m.done || m.cancelled {
		t.Fatal("completed vault was reported as cancelled")
	}
	failure := errors.New("recovery data retained")
	m = newSetupWizard(&App{}, setupRequest{Fixed: true, Name: "personal"})
	m.Update(wizardResult{err: failure})
	wizardKey(m, tea.KeyEsc)
	if !m.done || m.cancelled || m.err != failure {
		t.Fatal("closing failure swallowed recovery error")
	}
}
