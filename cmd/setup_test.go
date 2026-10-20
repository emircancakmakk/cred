package cmd

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/emircancakmakk/cred/internal/profiles"
	"github.com/emircancakmakk/cred/internal/vault"
)

func runWizard(t *testing.T, choice setupChoice, passwords, lines []string) (string, error) {
	t.Helper()
	var output bytes.Buffer
	script := &scriptPrompter{passwords: passwords, lines: lines}
	app := &App{Out: &output, Err: &output, Prompter: script, WorkFactor: 1, setupMenu: func() (setupChoice, error) { return choice, nil }}
	e := app.Execute([]string{"init"})
	if e == nil {
		if e = script.remaining(); e != nil {
			t.Fatal(e)
		}
	}
	assertNoLeak(t, output.String(), passwords...)
	return output.String(), e
}
func emptyRemote(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	remote := filepath.Join(root, "empty.git")
	gitTest(t, root, "init", "--bare", "--initial-branch=main", remote)
	return remote
}
func TestSetupMenuNavigationAndCancellation(t *testing.T) {
	m := setupModel{}
	for _, step := range []struct {
		key  tea.KeyType
		want setupChoice
	}{{tea.KeyUp, setupShared}, {tea.KeyDown, setupLocal}, {tea.KeyDown, setupConnect}, {tea.KeyDown, setupShared}} {
		next, _ := m.Update(tea.KeyMsg{Type: step.key})
		m = next.(setupModel)
		if m.selected != step.want {
			t.Fatalf("selected %d", m.selected)
		}
	}
	if !strings.Contains(m.View(), "> Create a shared vault") {
		t.Fatal(m.View())
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !next.(setupModel).accepted || cmd == nil {
		t.Fatal("Enter did not select")
	}
	for _, key := range []tea.KeyType{tea.KeyEsc, tea.KeyCtrlC} {
		next, cmd = m.Update(tea.KeyMsg{Type: key})
		if !next.(setupModel).cancelled || cmd == nil {
			t.Fatal("cancel did not quit")
		}
	}
	isolateProfiles(t)
	var output bytes.Buffer
	app := &App{Out: &output, Err: &output, setupMenu: func() (setupChoice, error) { return 0, errSetupCancelled }}
	if e := app.Execute([]string{"init"}); e != nil {
		t.Fatal(e)
	}
	dir, _ := profiles.Dir()
	if _, e := os.Stat(dir); !os.IsNotExist(e) {
		t.Fatal("cancel wrote configuration")
	}
	app.setupMenu = nil
	if e := app.Execute([]string{"init"}); e == nil || !strings.Contains(e.Error(), "interactive terminal required") {
		t.Fatal(e)
	}
}
func TestWizardLocalDefaultAndEnvironment(t *testing.T) {
	isolateProfiles(t)
	override := filepath.Join(t.TempDir(), "old.age")
	t.Setenv("CRED_VAULT", override)
	text, e := runWizard(t, setupLocal, []string{masterPassword, masterPassword}, []string{"", "y"})
	if e != nil {
		t.Fatal(e)
	}
	c, e := profiles.Load()
	if e != nil {
		t.Fatal(e)
	}
	path, _ := profiles.NamedPath("local")
	if c.Default != "local" || c.Vaults["local"].Path != path {
		t.Fatalf("config %+v", c)
	}
	if _, e = os.Stat(override); !os.IsNotExist(e) {
		t.Fatal("wizard used CRED_VAULT")
	}
	if !strings.Contains(text, "unset CRED_VAULT") || !strings.Contains(text, path) {
		t.Fatal(text)
	}
	_, e = runWizard(t, setupLocal, []string{masterPassword, masterPassword}, []string{"personal", ""})
	if e != nil {
		t.Fatal(e)
	}
	c, _ = profiles.Load()
	if c.Default != "local" {
		t.Fatal("empty answer changed default")
	}
	before, _ := os.ReadFile(path)
	_, e = runWizard(t, setupLocal, nil, []string{"local"})
	if e == nil || !strings.Contains(e.Error(), "profile already exists") {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("existing vault changed")
	}
}
func TestWizardConnectAndShared(t *testing.T) {
	isolateProfiles(t)
	existing := gitFixture(t)
	_, e := runWizard(t, setupConnect, []string{masterPassword}, []string{"existing", existing, "y"})
	if e != nil {
		t.Fatal(e)
	}
	c, _ := profiles.Load()
	if c.Default != "existing" || c.Vaults["existing"].Type != "git" {
		t.Fatalf("config %+v", c)
	}
	repo := emptyRemote(t)
	_, e = runWizard(t, setupShared, []string{masterPassword, masterPassword}, []string{"new-team", repo, ""})
	if e != nil {
		t.Fatal(e)
	}
	c, _ = profiles.Load()
	p := c.Vaults["new-team"]
	if p.Initializing || p.Type != "git" || c.Default != "existing" {
		t.Fatalf("config %+v", c)
	}
	if len(c.Vaults) != 2 {
		t.Fatal("unexpected seed profile")
	}
	if _, e = os.Stat(p.Path + ".pending"); !os.IsNotExist(e) {
		t.Fatal("pending retained on success")
	}
	g, e := profiles.OpenGit(repo, filepath.Join(t.TempDir(), "check"))
	if e != nil {
		t.Fatal(e)
	}
	v, e := vault.Load(g.Path(), []byte(masterPassword))
	if e != nil || v == nil || len(v.Projects) != 0 {
		t.Fatalf("vault %v %v", v, e)
	}
}
func TestWizardValidation(t *testing.T) {
	tests := []struct {
		name             string
		choice           setupChoice
		passwords, lines []string
		want             string
	}{
		{"bad profile", setupLocal, nil, []string{"../bad"}, "profile name"},
		{"missing Git name", setupConnect, nil, []string{""}, "profile name"},
		{"missing repository", setupConnect, nil, []string{"team", ""}, "repository URL"},
		{"short password", setupLocal, []string{"short"}, []string{"personal"}, "at least 12"},
		{"mismatch", setupLocal, []string{masterPassword, "other-password"}, []string{"personal"}, "do not match"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateProfiles(t)
			_, e := runWizard(t, tt.choice, tt.passwords, tt.lines)
			if e == nil || !strings.Contains(e.Error(), tt.want) {
				t.Fatal(e)
			}
			c, _ := profiles.Load()
			if len(c.Vaults) != 0 {
				t.Fatal("failed setup registered a profile")
			}
		})
	}
	isolateProfiles(t)
	repo := gitFixture(t)
	_, e := runWizard(t, setupShared, nil, []string{"team", repo})
	if e == nil || !strings.Contains(e.Error(), "empty repository") {
		t.Fatal(e)
	}
	_, e = runWizard(t, setupConnect, []string{"wrong-password", "wrong-password", "wrong-password"}, []string{"wrong", repo})
	if !errors.Is(e, vault.ErrWrongPassword) {
		t.Fatal(e)
	}
	c, _ := profiles.Load()
	if len(c.Vaults) != 0 {
		t.Fatal("invalid Git setup registered a profile")
	}
}

// Return a PATH override that denies pushes while delegating all other Git work.
func blockPush(t *testing.T) string {
	t.Helper()
	original := os.Getenv("PATH")
	realGit, e := exec.LookPath("git")
	if e != nil {
		t.Fatal(e)
	}
	bin := t.TempDir()
	script := "#!/bin/sh\nfor arg do\n if [ \"$arg\" = push ]; then printf '%s\\n' '! [remote rejected] HEAD -> main (pre-receive hook declined)' >&2; exit 1; fi\ndone\nexec '" + realGit + "' \"$@\"\n"
	if e = os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+original)
	return original
}
func TestSharedFirstPushRecovery(t *testing.T) {
	for _, peer := range []string{"empty", "same-password", "different-password"} {
		t.Run(peer, func(t *testing.T) {
			isolateProfiles(t)
			_ = gitFixture(t)
			repo := emptyRemote(t)
			original := blockPush(t)
			text, e := runWizard(t, setupShared, []string{masterPassword, masterPassword}, []string{"team", repo})
			if e == nil || !strings.Contains(e.Error(), "cred vault retry team") || strings.Contains(text, "Ready:") {
				t.Fatalf("failure %q %v", text, e)
			}
			c, e := profiles.Load()
			if e != nil {
				t.Fatal(e)
			}
			p := c.Vaults["team"]
			if !p.Initializing || c.Default != "" {
				t.Fatalf("config %+v", c)
			}
			if _, e = vault.Load(p.Path+".pending", []byte(masterPassword)); e != nil {
				t.Fatal(e)
			}
			status := statusOK(t, "--vault", "team", "status")
			if !strings.Contains(status, "Setup: incomplete") {
				t.Fatal(status)
			}
			_, _, e = runCmd(t, []string{"vault", "use", "team"}, nil, nil)
			if e == nil {
				t.Fatal("incomplete setup became default")
			}
			t.Setenv("PATH", original)
			if peer != "empty" {
				other, e := profiles.OpenGit(repo, filepath.Join(t.TempDir(), "peer"))
				if e != nil {
					t.Fatal(e)
				}
				v := vault.New()
				_ = v.Add("peer", "s", "f", vault.Field{Value: "retained-peer-secret", Secret: true})
				pass := masterPassword
				if peer == "different-password" {
					pass = "different-master-password"
				}
				if e = other.Publish([]byte(pass), v, fastOptions()); e != nil {
					t.Fatal(e)
				}
			}
			_, _, e = runCmd(t, []string{"vault", "retry", "team"}, []string{masterPassword}, nil)
			if peer == "different-password" {
				if !errors.Is(e, vault.ErrWrongPassword) {
					t.Fatal(e)
				}
				c, _ = profiles.Load()
				if !c.Vaults["team"].Initializing {
					t.Fatal("failed recovery completed setup")
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			c, _ = profiles.Load()
			if c.Vaults["team"].Initializing {
				t.Fatal("setup marker not cleared")
			}
			if _, e = os.Stat(p.Path + ".pending"); !os.IsNotExist(e) {
				t.Fatal("pending not cleaned")
			}
			if peer == "same-password" {
				v, e := vault.Load(filepath.Join(p.RepoDir(), "vault.age"), []byte(masterPassword))
				if e != nil {
					t.Fatal(e)
				}
				if _, e = v.GetField("peer", "s", "f"); e != nil {
					t.Fatal("peer edit lost")
				}
			}
		})
	}
}

func TestSharedRejectsTagsWithoutDefaultBranch(t *testing.T) {
	isolateProfiles(t)
	repo := gitFixture(t)
	head := gitTest(t, repo, "rev-parse", "refs/heads/main")
	gitTest(t, repo, "update-ref", "refs/tags/retained", head)
	gitTest(t, repo, "update-ref", "-d", "refs/heads/main")
	_, e := runWizard(t, setupShared, nil, []string{"team", repo})
	if e == nil || !strings.Contains(e.Error(), "empty repository") {
		t.Fatal(e)
	}
	if got := gitTest(t, repo, "rev-parse", "refs/tags/retained"); got != head {
		t.Fatal("tag changed")
	}
	c, _ := profiles.Load()
	if len(c.Vaults) != 0 {
		t.Fatal("rejected repo registered")
	}
}
func TestWizardProgressAndDirectShortcuts(t *testing.T) {
	isolateProfiles(t)
	repo := gitFixture(t)
	active := false
	phases := []string{}
	var output bytes.Buffer
	app := &App{Out: &output, Err: &output, WorkFactor: 1, progressObserver: phaseRecorder(t, &active, &phases), setupMenu: func() (setupChoice, error) {
		if active {
			t.Fatal("spinner in menu")
		}
		return setupConnect, nil
	}}
	script := &scriptPrompter{passwords: []string{masterPassword}, lines: []string{"team", repo, ""}}
	app.Prompter = &idlePrompter{t, script, &active}
	if e := app.Execute([]string{"init"}); e != nil {
		t.Fatal(e)
	}
	if active {
		t.Fatal("spinner after wizard")
	}
	app.setupMenu = func() (setupChoice, error) { t.Fatal("shortcut opened menu"); return 0, nil }
	script = &scriptPrompter{passwords: []string{masterPassword, masterPassword}}
	app.Prompter = &idlePrompter{t, script, &active}
	if e := app.Execute([]string{"--vault", "personal", "init"}); e != nil {
		t.Fatal(e)
	}
	if e := script.remaining(); e != nil {
		t.Fatal(e)
	}
	script = &scriptPrompter{passwords: []string{masterPassword}}
	app.Prompter = &idlePrompter{t, script, &active}
	if e := app.Execute([]string{"connect", "other-team", repo}); e != nil {
		t.Fatal(e)
	}
	if e := script.remaining(); e != nil {
		t.Fatal(e)
	}
	c, _ := profiles.Load()
	if c.Default != "" {
		t.Fatal("shortcut changed default")
	}
}
