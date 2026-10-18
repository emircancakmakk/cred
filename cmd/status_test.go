package cmd

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/emircancakmakk/cred/internal/profiles"
	"github.com/emircancakmakk/cred/internal/vault"
)

func isolateProfiles(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("CRED_VAULT", "")
}
func statusOK(t *testing.T, args ...string) string {
	t.Helper()
	out, errout, err := runCmd(t, args, nil, nil)
	if err != nil {
		t.Fatalf("status: %v %s", err, errout)
	}
	if errout != "" {
		t.Fatalf("unexpected non-terminal output %q", errout)
	}
	return out
}
func TestStatusLocalNoPasswordOrMutation(t *testing.T) {
	isolateProfiles(t)
	p, _ := profiles.NamedPath("local")
	text := statusOK(t, "status")
	for _, want := range []string{"Profile: local", "Type: local", "Selected by: default profile", "Vault: " + p, "Vault file: missing", "not initialized"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %q", want, text)
		}
	}
	if _, e := os.Stat(filepath.Dir(p)); !os.IsNotExist(e) {
		t.Fatalf("status created profile directory: %v", e)
	}
	_, _, e := runCmd(t, []string{"init", "personal"}, []string{masterPassword, masterPassword}, nil)
	if e != nil {
		t.Fatal(e)
	}
	c, _ := profiles.Load()
	p = c.Vaults["personal"].Path
	before, _ := os.ReadFile(p)
	text = statusOK(t, "--vault", "personal", "status")
	if !strings.Contains(text, "Selected by: --vault") || !strings.Contains(text, "Vault file: present") {
		t.Fatal(text)
	}
	after, _ := os.ReadFile(p)
	if !bytes.Equal(before, after) {
		t.Fatal("status changed vault")
	}
	override := filepath.Join(t.TempDir(), "override.age")
	t.Setenv("CRED_VAULT", override)
	text = statusOK(t, "status")
	if !strings.Contains(text, "Selected by: CRED_VAULT") || !strings.Contains(text, override) {
		t.Fatal(text)
	}
	text = statusOK(t, "--vault", "personal", "status")
	if strings.Contains(text, override) {
		t.Fatal("override beat explicit profile")
	}
	_, _, e = runCmd(t, []string{"--vault", "personal", "status", "--remote"}, nil, nil)
	if e == nil || !strings.Contains(e.Error(), "requires a Git vault") {
		t.Fatal(e)
	}
	_, _, e = runCmd(t, []string{"--vault", "unknown", "status"}, nil, nil)
	if e == nil {
		t.Fatal("unknown profile accepted")
	}
}

// snapshotTree checks both content and modification times, including Git index,
// refs, config and lock files. Status must not refresh the index or fetch refs.
func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	e := filepath.WalkDir(root, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		result[path] = fmt.Sprintf("%x/%d", sha256.Sum256(b), info.ModTime().UnixNano())
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	return result
}
func TestStatusGitReadOnlyAndRemote(t *testing.T) {
	isolateProfiles(t)
	repo := gitFixture(t)
	_, _, e := runCmd(t, []string{"connect", "team", repo}, []string{masterPassword}, nil)
	if e != nil {
		t.Fatal(e)
	}
	c, _ := profiles.Load()
	p := c.Vaults["team"]
	dir, _ := profiles.Dir()
	before := snapshotTree(t, dir)
	text := statusOK(t, "--vault", "team", "status")
	for _, want := range []string{"Checkout state: present", "Branch: main", "Working copy: clean", "Remote: not checked", "Pending change: missing"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q: %s", want, text)
		}
	}
	text = statusOK(t, "--vault", "team", "status", "--remote")
	if !strings.Contains(text, "commits match") {
		t.Fatal(text)
	}
	if !reflect.DeepEqual(before, snapshotTree(t, dir)) {
		t.Fatal("status modified local state")
	}
	other, e := profiles.OpenGit(repo, filepath.Join(t.TempDir(), "other"))
	if e != nil {
		t.Fatal(e)
	}
	v, e := vault.Load(other.Path(), []byte(masterPassword))
	if e != nil {
		t.Fatal(e)
	}
	_ = v.Add("new", "s", "f", vault.Field{Value: "not printed", Secret: true})
	if e = other.Publish([]byte(masterPassword), v, fastOptions()); e != nil {
		t.Fatal(e)
	}
	text = statusOK(t, "--vault", "team", "status", "--remote")
	if !strings.Contains(text, "commits differ") {
		t.Fatal(text)
	}
	if !reflect.DeepEqual(before, snapshotTree(t, dir)) {
		t.Fatal("remote status fetched or modified local state")
	}
	if e = os.WriteFile(p.Path+".pending", []byte("opaque"), 0600); e != nil {
		t.Fatal(e)
	}
	text = statusOK(t, "--vault", "team", "status")
	if !strings.Contains(text, "Pending change: present") || !strings.Contains(text, "cred vault retry team") {
		t.Fatal(text)
	}
	if e = os.WriteFile(filepath.Join(p.RepoDir(), "vault.age"), []byte("changed"), 0600); e != nil {
		t.Fatal(e)
	}
	text = statusOK(t, "--vault", "team", "status")
	if !strings.Contains(text, "Working copy: uncommitted changes") {
		t.Fatal(text)
	}
	release, e := profiles.Lock(p.Path + ".lock")
	if e != nil {
		t.Fatal(e)
	}
	defer release()
	text = statusOK(t, "--vault", "team", "status")
	if !strings.Contains(text, "State: busy") || strings.Contains(text, "Working copy:") {
		t.Fatal(text)
	}
}
func TestStatusMissingCheckoutDoesNotClone(t *testing.T) {
	isolateProfiles(t)
	path, _ := profiles.NamedPath("team")
	c := profiles.Config{Vaults: map[string]profiles.Profile{"team": {Type: "git", Repo: "git@example.invalid:private/repo.git", Path: path}}}
	if e := profiles.Save(c); e != nil {
		t.Fatal(e)
	}
	// With no Git executable, the default status still reports the missing checkout.
	t.Setenv("PATH", t.TempDir())
	text := statusOK(t, "--vault", "team", "status")
	if !strings.Contains(text, "Checkout state: missing") {
		t.Fatal(text)
	}
	if _, e := os.Stat(filepath.Dir(path)); !os.IsNotExist(e) {
		t.Fatalf("status created checkout %v", e)
	}
	_, _, e := runCmd(t, []string{"--vault", "team", "status", "--remote"}, nil, nil)
	if e == nil || !strings.Contains(e.Error(), "install Git") {
		t.Fatal(e)
	}
}
func TestStatusRemoteMissingBranchAndInvalidConfig(t *testing.T) {
	isolateProfiles(t)
	repo := gitFixture(t)
	_, _, e := runCmd(t, []string{"connect", "team", repo}, []string{masterPassword}, nil)
	if e != nil {
		t.Fatal(e)
	}
	gitTest(t, repo, "update-ref", "-d", "refs/heads/main")
	_, _, e = runCmd(t, []string{"--vault", "team", "status", "--remote"}, nil, nil)
	if e == nil || !strings.Contains(e.Error(), "branch is missing") {
		t.Fatal(e)
	}
	dir, _ := profiles.Dir()
	if e = os.WriteFile(filepath.Join(dir, "profiles.json"), []byte("{"), 0600); e != nil {
		t.Fatal(e)
	}
	_, _, e = runCmd(t, []string{"status"}, nil, nil)
	if e == nil {
		t.Fatal("invalid config accepted")
	}
}
func TestProgressCleanupAndNonTerminal(t *testing.T) {
	var b bytes.Buffer
	stop := startProgress(&b, true, "Updating vault")
	stop()
	stop()
	if !strings.Contains(b.String(), "Updating vault") || !strings.HasSuffix(b.String(), "\r\x1b[2K") {
		t.Fatalf("spinner %q", b.String())
	}
	before := b.String()
	time.Sleep(110 * time.Millisecond)
	if b.String() != before {
		t.Fatal("spinner wrote after cleanup")
	}
	var quiet bytes.Buffer
	app := &App{Err: &quiet}
	stop = app.stage("Sharing changes")
	stop()
	if quiet.Len() != 0 {
		t.Fatal("spinner in redirected output")
	}
}

// A rejected push must not trigger fetch/reconciliation for policy or identity
// failures; only an explicit non-fast-forward rejection may take that path.
func TestPolicyPushFailureStopsWithoutFetch(t *testing.T) {
	isolateProfiles(t)
	repo := gitFixture(t)
	g, e := profiles.OpenGit(repo, filepath.Join(t.TempDir(), "repo"))
	if e != nil {
		t.Fatal(e)
	}
	realGit, e := exec.LookPath("git")
	if e != nil {
		t.Fatal(e)
	}
	bin := t.TempDir()
	log := filepath.Join(t.TempDir(), "fetch-called")
	script := "#!/bin/sh\nfor arg do\n case \"$arg\" in\n push) printf '%s\\n' '! [remote rejected] HEAD -> main (pre-receive hook declined) TOKEN-SECRET' >&2; exit 1 ;;\n fetch) : > '" + log + "' ;;\n esac\ndone\nexec '" + realGit + "' \"$@\"\n"
	if e = os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	password := []byte(masterPassword)
	base, e := vault.Load(g.Path(), password)
	if e != nil {
		t.Fatal(e)
	}
	desired := vault.Clone(base)
	_ = desired.Set("p", "s", "a", vault.Field{Value: "local-secret", Secret: true})
	path := filepath.Join(t.TempDir(), "vault.age")
	app := &App{gitSession: g, base: base, vaultName: "team", profile: profiles.Profile{Type: "git", Path: path}, WorkFactor: 1}
	e = app.saveGit(password, desired)
	if e == nil || !strings.Contains(e.Error(), "server rejected") {
		t.Fatal(e)
	}
	if strings.Contains(e.Error(), "TOKEN-SECRET") || strings.Contains(e.Error(), "local-secret") {
		t.Fatal("secret leaked")
	}
	if _, e = os.Stat(log); !os.IsNotExist(e) {
		t.Fatal("retried a policy rejection")
	}
	if _, e = vault.Load(path+".pending", password); e != nil {
		t.Fatal(e)
	}
}

type idlePrompter struct {
	t      *testing.T
	script *scriptPrompter
	active *bool
}

func (p *idlePrompter) ReadPassword(label string) ([]byte, error) {
	p.t.Helper()
	if *p.active {
		p.t.Fatal("spinner active during password prompt")
	}
	return p.script.ReadPassword(label)
}
func (p *idlePrompter) ReadLine(label string) (string, error) {
	p.t.Helper()
	if *p.active {
		p.t.Fatal("spinner active during choice prompt")
	}
	return p.script.ReadLine(label)
}
func phaseRecorder(t *testing.T, active *bool, phases *[]string) profiles.Observer {
	return func(label string) func() {
		t.Helper()
		if *active {
			t.Fatal("overlapping progress phases")
		}
		*active = true
		*phases = append(*phases, label)
		stopped := false
		return func() {
			if !stopped {
				*active = false
				stopped = true
			}
		}
	}
}
func TestProgressStopsBeforePromptsTUIAndErrors(t *testing.T) {
	isolateProfiles(t)
	active := false
	phases := []string{}
	var output bytes.Buffer
	script := &scriptPrompter{passwords: []string{masterPassword, masterPassword}}
	app := &App{Err: &output, Out: &output, WorkFactor: 1, progressObserver: phaseRecorder(t, &active, &phases)}
	app.Prompter = &idlePrompter{t, script, &active}
	if e := app.Execute([]string{"init", "personal"}); e != nil {
		t.Fatal(e)
	}
	if active {
		t.Fatal("progress not cleaned after save")
	}
	script = &scriptPrompter{passwords: []string{masterPassword}}
	app.Prompter = &idlePrompter{t, script, &active}
	tuiCalled := false
	app.RunTUI = func(string) error {
		tuiCalled = true
		if active {
			t.Fatal("spinner active in TUI")
		}
		return nil
	}
	if e := app.Execute([]string{"--vault", "personal"}); e != nil {
		t.Fatal(e)
	}
	if !tuiCalled {
		t.Fatal("TUI not called")
	}
	script = &scriptPrompter{passwords: []string{"wrong-password"}}
	app.Prompter = &idlePrompter{t, script, &active}
	if e := app.Execute([]string{"--vault", "personal", "ls"}); e == nil {
		t.Fatal("wrong password accepted")
	}
	if active {
		t.Fatal("progress not cleaned after error")
	}
	base := vault.New()
	local, remote := vault.Clone(base), vault.Clone(base)
	_ = local.Add("p", "s", "f", vault.Field{Value: "mine"})
	_ = remote.Add("p", "s", "f", vault.Field{Value: "team"})
	for _, choice := range []string{"2", "3"} {
		script = &scriptPrompter{lines: []string{choice}}
		app.Prompter = &idlePrompter{t, script, &active}
		_, e := app.mergeVault(base, local, remote)
		if choice == "3" && e == nil {
			t.Fatal("cancel accepted")
		}
		if choice == "2" && e != nil {
			t.Fatal(e)
		}
		if active {
			t.Fatal("progress not cleaned after merge")
		}
	}
	if !strings.Contains(strings.Join(phases, ","), "Unlocking vault") || !strings.Contains(strings.Join(phases, ","), "Merging changes") {
		t.Fatal(phases)
	}
}
func TestGitObserverClosesAllPhases(t *testing.T) {
	isolateProfiles(t)
	repo := gitFixture(t)
	active := false
	phases := []string{}
	observer := phaseRecorder(t, &active, &phases)
	g, e := profiles.OpenGit(repo, filepath.Join(t.TempDir(), "checkout"), observer)
	if e != nil {
		t.Fatal(e)
	}
	if e = g.Update(false); e != nil {
		t.Fatal(e)
	}
	password := []byte(masterPassword)
	v, e := vault.Load(g.Path(), password)
	if e != nil {
		t.Fatal(e)
	}
	_ = v.Set("p", "s", "a", vault.Field{Value: "new"})
	if e = g.Publish(password, v, fastOptions()); e != nil {
		t.Fatal(e)
	}
	gitTest(t, g.Dir, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing.git"))
	if e = g.Publish(password, v, fastOptions()); e == nil {
		t.Fatal("missing remote accepted")
	}
	if active {
		t.Fatal("Git failure left spinner active")
	}
	for _, phase := range []string{"Cloning vault", "Updating vault", "Encrypting vault", "Committing encrypted vault", "Sharing changes"} {
		if !strings.Contains(strings.Join(phases, ","), phase) {
			t.Fatalf("missing phase %s", phase)
		}
	}
}

func TestOfflineStatusNeverRunsNetworkOrWrites(t *testing.T) {
	isolateProfiles(t)
	repo := gitFixture(t)
	_, _, e := runCmd(t, []string{"connect", "team", repo}, []string{masterPassword}, nil)
	if e != nil {
		t.Fatal(e)
	}
	realGit, e := exec.LookPath("git")
	if e != nil {
		t.Fatal(e)
	}
	bin := t.TempDir()
	log := filepath.Join(t.TempDir(), "forbidden-called")
	script := "#!/bin/sh\nfor arg do\n case \"$arg\" in\n clone|fetch|pull|reset|push|ls-remote) : > '" + log + "'; exit 1 ;;\n esac\ndone\nexec '" + realGit + "' \"$@\"\n"
	if e = os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	text := statusOK(t, "--vault", "team", "status")
	if !strings.Contains(text, "Remote: not checked") {
		t.Fatal(text)
	}
	if _, e = os.Stat(log); !os.IsNotExist(e) {
		t.Fatal("offline status ran a network/mutating command")
	}
}
