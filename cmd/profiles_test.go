package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emircancakmakk/cred/internal/profiles"
	"github.com/emircancakmakk/cred/internal/vault"
)

func gitTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	b, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %v %s", args, e, b)
	}
	return strings.TrimSpace(string(b))
}
func gitFixture(t *testing.T) string {
	t.Helper()
	t.Setenv("GIT_AUTHOR_NAME", "Cred Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "cred@example.invalid")
	t.Setenv("GIT_COMMITTER_NAME", "Cred Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "cred@example.invalid")
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	gitTest(t, root, "init", "--bare", "--initial-branch=main", remote)
	seed := filepath.Join(root, "seed")
	gitTest(t, root, "clone", remote, seed)
	v := vault.New()
	_ = v.Add("p", "s", "a", vault.Field{Value: "old", Secret: true})
	_ = v.Add("p", "s", "b", vault.Field{Value: "old", Secret: true})
	if e := vault.Save(filepath.Join(seed, "vault.age"), []byte(masterPassword), v, fastOptions()); e != nil {
		t.Fatal(e)
	}
	gitTest(t, seed, "add", "vault.age")
	gitTest(t, seed, "commit", "-m", "Initial encrypted vault")
	gitTest(t, seed, "push", "origin", "main")
	return remote
}
func TestGitConcurrentSave(t *testing.T) {
	for _, mode := range []string{"different", "team", "mine", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			repo := gitFixture(t)
			one, e := profiles.OpenGit(repo, filepath.Join(t.TempDir(), "repo"))
			if e != nil {
				t.Fatal(e)
			}
			two, e := profiles.OpenGit(repo, filepath.Join(t.TempDir(), "repo"))
			if e != nil {
				t.Fatal(e)
			}
			password := []byte(masterPassword)
			base, e := vault.Load(one.Path(), password)
			if e != nil {
				t.Fatal(e)
			}
			local, remote := vault.Clone(base), vault.Clone(base)
			_ = local.Set("p", "s", "a", vault.Field{Value: "mine", Secret: true})
			field := "a"
			if mode == "different" {
				field = "b"
			}
			_ = remote.Set("p", "s", field, vault.Field{Value: "team", Secret: true})
			if e = one.Publish(password, remote, fastOptions()); e != nil {
				t.Fatal(e)
			}
			answer := map[string]string{"team": "1", "mine": "2", "cancel": "3"}[mode]
			lines := []string{}
			if mode != "different" {
				lines = append(lines, answer)
			}
			var output bytes.Buffer
			pending := filepath.Join(t.TempDir(), "vault.age")
			app := &App{gitSession: two, base: base, profile: profiles.Profile{Type: "git", Path: pending}, WorkFactor: 1, Prompter: &scriptPrompter{lines: lines}, Err: &output}
			e = app.saveGit(password, local)
			if mode == "cancel" {
				if e == nil {
					t.Fatal("cancel succeeded")
				}
				if _, e = vault.Load(pending+".pending", password); e != nil {
					t.Fatal(e)
				}
				if _, e = vault.Load(pending+".pending.base", password); e != nil {
					t.Fatal(e)
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			if _, e = os.Stat(pending + ".pending"); !os.IsNotExist(e) {
				t.Fatal("pending retained")
			}
			check, e := profiles.OpenGit(repo, filepath.Join(t.TempDir(), "repo"))
			if e != nil {
				t.Fatal(e)
			}
			result, e := vault.Load(check.Path(), password)
			if e != nil {
				t.Fatal(e)
			}
			a, _ := result.GetField("p", "s", "a")
			want := "mine"
			if mode == "team" {
				want = "team"
			}
			if a.Value != want {
				t.Fatalf("a=%s", a.Value)
			}
			if mode == "different" {
				b, _ := result.GetField("p", "s", "b")
				if b.Value != "team" {
					t.Fatal("lost remote field")
				}
			}
			if got := gitTest(t, check.Dir, "ls-tree", "--name-only", "HEAD"); got != "vault.age" {
				t.Fatalf("unexpected tracked files: %s", got)
			}
			assertNoLeak(t, output.String(), "mine", "team-secret")
		})
	}
}
func TestProfilesLocalAndGit(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("CRED_VAULT", "")
	repo := gitFixture(t)
	_, _, e := runCmd(t, []string{"init", "personal"}, []string{masterPassword, masterPassword}, nil)
	if e != nil {
		t.Fatal(e)
	}
	_, _, e = runCmd(t, []string{"connect", "team", repo}, []string{masterPassword}, nil)
	if e != nil {
		t.Fatal(e)
	}
	_, _, e = runCmd(t, []string{"vault", "use", "personal"}, nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	_, _, e = runCmd(t, []string{"--vault", "team", "set", "p", "s", "a"}, []string{masterPassword, "changed"}, []string{""})
	if e != nil {
		t.Fatal(e)
	}
	_, _, e = runCmd(t, []string{"ls"}, []string{masterPassword}, nil)
	if e != nil {
		t.Fatal(e)
	}
	stdout, _, e := runCmd(t, []string{"p", "s", "a", "--vault=team", "--show"}, []string{masterPassword}, nil)
	if e != nil || stdout != "changed\n" {
		t.Fatalf("%q %v", stdout, e)
	}
	c, e := profiles.Load()
	if e != nil || c.Default != "personal" {
		t.Fatalf("config %v %v", c, e)
	}
	release, e := profiles.Lock(c.Vaults["personal"].Path + ".lock")
	if e != nil {
		t.Fatal(e)
	}
	defer release()
	_, _, e = runCmd(t, []string{"ls"}, nil, nil)
	if e == nil || !strings.Contains(e.Error(), "busy") {
		t.Fatalf("lock: %v", e)
	}
}

func TestBootstrapAndRetry(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("CRED_VAULT", "")
	// Establish a deterministic Git identity, then create a separate empty remote.
	_ = gitFixture(t)
	root := t.TempDir()
	repo := filepath.Join(root, "empty.git")
	gitTest(t, root, "init", "--bare", "--initial-branch=main", repo)
	_, _, e := runCmd(t, []string{"init", "personal"}, []string{masterPassword, masterPassword}, nil)
	if e != nil {
		t.Fatal(e)
	}
	_, _, e = runCmd(t, []string{"connect", "team", repo, "--from", "personal"}, []string{masterPassword}, nil)
	if e != nil {
		t.Fatal(e)
	}
	c, e := profiles.Load()
	if e != nil {
		t.Fatal(e)
	}
	p := c.Vaults["team"]
	g, e := profiles.OpenGit(repo, filepath.Join(t.TempDir(), "repo"))
	if e != nil {
		t.Fatal(e)
	}
	password := []byte(masterPassword)
	base, e := vault.Load(g.Path(), password)
	if e != nil {
		t.Fatal(e)
	}
	desired := vault.Clone(base)
	_ = desired.Add("p", "s", "a", vault.Field{Value: "retained", Secret: true})
	if e = vault.Save(p.Path+".pending.base", password, base, fastOptions()); e != nil {
		t.Fatal(e)
	}
	if e = vault.Save(p.Path+".pending", password, desired, fastOptions()); e != nil {
		t.Fatal(e)
	}
	_, _, e = runCmd(t, []string{"--vault", "team", "ls"}, nil, nil)
	if e == nil || !strings.Contains(e.Error(), "retry") {
		t.Fatalf("pending not blocked: %v", e)
	}
	_, _, e = runCmd(t, []string{"vault", "retry", "team"}, []string{masterPassword}, nil)
	if e != nil {
		t.Fatal(e)
	}
	stdout, _, e := runCmd(t, []string{"--vault", "team", "p", "s", "a", "--show"}, []string{masterPassword}, nil)
	if e != nil || stdout != "retained\n" {
		t.Fatalf("recovery %q %v", stdout, e)
	}
}

func TestGitUnavailableRetainsChange(t *testing.T) {
	repo := gitFixture(t)
	g, e := profiles.OpenGit(repo, filepath.Join(t.TempDir(), "repo"))
	if e != nil {
		t.Fatal(e)
	}
	password := []byte(masterPassword)
	base, e := vault.Load(g.Path(), password)
	if e != nil {
		t.Fatal(e)
	}
	desired := vault.Clone(base)
	_ = desired.Set("p", "s", "a", vault.Field{Value: "retained-secret", Secret: true})
	// Simulate loss of repository access after the initial read.
	gitTest(t, g.Dir, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "unavailable.git"))
	path := filepath.Join(t.TempDir(), "vault.age")
	app := &App{gitSession: g, base: base, profile: profiles.Profile{Type: "git", Path: path}, WorkFactor: 1}
	e = app.saveGit(password, desired)
	if e == nil || !strings.Contains(e.Error(), "not shared") {
		t.Fatalf("expected publication failure: %v", e)
	}
	pending, e := vault.Load(path+".pending", password)
	if e != nil {
		t.Fatal(e)
	}
	field, e := pending.GetField("p", "s", "a")
	if e != nil || field.Value != "retained-secret" {
		t.Fatalf("lost pending %v %v", field, e)
	}
	if _, e = vault.Load(path+".pending.base", password); e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile(path + ".pending")
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(raw, []byte("retained-secret")) {
		t.Fatal("plaintext persisted")
	}
}

func TestDefaultLocalProfileUsesUniformLayout(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("CRED_VAULT", "")
	_, _, e := runCmd(t, []string{"init", "local"}, []string{masterPassword, masterPassword}, nil)
	if e != nil {
		t.Fatal(e)
	}
	c, e := profiles.Load()
	if e != nil {
		t.Fatal(e)
	}
	want, e := profiles.NamedPath("local")
	if e != nil {
		t.Fatal(e)
	}
	if c.Vaults["local"].Path != want {
		t.Fatalf("local path %q", c.Vaults["local"].Path)
	}
	defaultPath, e := vault.DefaultPath()
	if e != nil || defaultPath != want {
		t.Fatalf("default path %q %v", defaultPath, e)
	}
	_, _, e = runCmd(t, []string{"init", "personal"}, []string{masterPassword, masterPassword}, nil)
	if e != nil {
		t.Fatal(e)
	}
	_, _, e = runCmd(t, []string{"vault", "use", "personal"}, nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	_, _, e = runCmd(t, []string{"vault", "use", "local"}, nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	stdout, _, e := runCmd(t, []string{"vault", "list"}, nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Count(stdout, "local (local)") != 1 || !strings.Contains(stdout, "* local (local)") || strings.Contains(stdout, "legacy") {
		t.Fatalf("profiles: %q", stdout)
	}
	d, _ := profiles.Dir()
	if _, e = os.Stat(filepath.Join(d, "vault.age")); !os.IsNotExist(e) {
		t.Fatalf("old layout used: %v", e)
	}
}

func TestPersistentGitFetchAndDirtyCheckout(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("CRED_VAULT", "")
	repo := gitFixture(t)
	_, _, e := runCmd(t, []string{"connect", "team", repo}, []string{masterPassword}, nil)
	if e != nil {
		t.Fatal(e)
	}
	c, e := profiles.Load()
	if e != nil {
		t.Fatal(e)
	}
	p := c.Vaults["team"]
	sentinel := filepath.Join(p.RepoDir(), ".git", "reuse-marker")
	if e = os.WriteFile(sentinel, []byte("keep"), 0600); e != nil {
		t.Fatal(e)
	}
	// Another user publishes a new project after connect.
	other, e := profiles.OpenGit(repo, filepath.Join(t.TempDir(), "other"))
	if e != nil {
		t.Fatal(e)
	}
	password := []byte(masterPassword)
	updated, e := vault.Load(other.Path(), password)
	if e != nil {
		t.Fatal(e)
	}
	_ = updated.Add("remote-project", "s", "f", vault.Field{Value: "remote-secret", Secret: true})
	if e = other.Publish(password, updated, fastOptions()); e != nil {
		t.Fatal(e)
	}
	stdout, _, e := runCmd(t, []string{"--vault", "team", "ls"}, []string{masterPassword}, nil)
	if e != nil || !strings.Contains(stdout, "remote-project") {
		t.Fatalf("fetch %q %v", stdout, e)
	}
	if _, e = os.Stat(sentinel); e != nil {
		t.Fatal("checkout was replaced")
	}
	// A local unpublished change is retained outside the checkout.
	_, _, e = runCmd(t, []string{"--vault", "team", "set", "p", "s", "a"}, []string{masterPassword, "new-secret"}, []string{""})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(sentinel); e != nil {
		t.Fatal("write replaced checkout")
	}
	info, e := os.Stat(filepath.Join(p.RepoDir(), "vault.age"))
	if e != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("vault permissions %v %v", info, e)
	}
	if e = os.WriteFile(filepath.Join(p.RepoDir(), "vault.age"), []byte("manual edit"), 0600); e != nil {
		t.Fatal(e)
	}
	_, _, e = runCmd(t, []string{"--vault", "team", "ls"}, nil, nil)
	if e == nil || !strings.Contains(e.Error(), "uncommitted") {
		t.Fatalf("dirty checkout: %v", e)
	}
	raw, e := os.ReadFile(filepath.Join(p.RepoDir(), "vault.age"))
	if e != nil || string(raw) != "manual edit" {
		t.Fatalf("dirty checkout overwritten: %q %v", raw, e)
	}
}
