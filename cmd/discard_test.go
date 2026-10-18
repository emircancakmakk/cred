package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emircancakmakk/cred/internal/profiles"
	"github.com/emircancakmakk/cred/internal/vault"
)

func pendingGit(t *testing.T) (string, profiles.Profile) {
	t.Helper()
	isolateProfiles(t)
	repo := gitFixture(t)
	_, _, e := runCmd(t, []string{"connect", "team", repo}, []string{masterPassword}, nil)
	if e != nil {
		t.Fatal(e)
	}
	c, _ := profiles.Load()
	c.Default = "team"
	if e = profiles.Save(c); e != nil {
		t.Fatal(e)
	}
	p := c.Vaults["team"]
	g, e := profiles.OpenGit(repo, p.RepoDir())
	if e != nil {
		t.Fatal(e)
	}
	base, e := vault.Load(g.Path(), []byte(masterPassword))
	if e != nil {
		t.Fatal(e)
	}
	desired := vault.Clone(base)
	_ = desired.Set("p", "s", "a", vault.Field{Value: "LOCAL-DISCARD-SECRET", Secret: true})
	// Include backup generations to exercise all known recovery paths.
	for range 2 {
		if e = vault.Save(p.Path+".pending.base", []byte(masterPassword), base, fastOptions()); e != nil {
			t.Fatal(e)
		}
		if e = vault.Save(p.Path+".pending", []byte(masterPassword), desired, fastOptions()); e != nil {
			t.Fatal(e)
		}
	}
	original := blockPush(t)
	if e = g.Publish([]byte(masterPassword), desired, fastOptions()); e == nil {
		t.Fatal("push unexpectedly succeeded")
	}
	t.Setenv("PATH", original)
	return repo, p
}
func discardOK(t *testing.T, name string, passwords, lines []string) string {
	t.Helper()
	out, errout, e := runCmd(t, []string{"vault", "discard", name}, passwords, lines)
	if e != nil {
		t.Fatalf("discard: %v %s", e, errout)
	}
	if out != "" {
		t.Fatalf("unexpected stdout %q", out)
	}
	assertNoLeak(t, errout, "LOCAL-DISCARD-SECRET", masterPassword)
	return errout
}
func pendingContents(t *testing.T, p profiles.Profile) map[string][]byte {
	t.Helper()
	result := map[string][]byte{}
	for _, path := range recoveryPaths(p) {
		data, e := os.ReadFile(path)
		if errors.Is(e, os.ErrNotExist) {
			continue
		}
		if e != nil {
			t.Fatal(e)
		}
		result[path] = data
	}
	return result
}
func assertPendingUnchanged(t *testing.T, before map[string][]byte) {
	t.Helper()
	for path, want := range before {
		got, e := os.ReadFile(path)
		if e != nil || !bytes.Equal(got, want) {
			t.Fatalf("pending data changed at %s: %v", path, e)
		}
	}
}
func TestDiscardConfirmationAndNoOp(t *testing.T) {
	for _, answer := range []string{"", "n", "no", "other"} {
		t.Run("cancel-"+answer, func(t *testing.T) {
			_, p := pendingGit(t)
			before := snapshotTree(t, filepath.Dir(p.Path))
			t.Setenv("PATH", t.TempDir())
			text := discardOK(t, "team", nil, []string{answer})
			if !strings.Contains(text, "cancelled") {
				t.Fatal(text)
			}
			after := snapshotTree(t, filepath.Dir(p.Path))
			if len(before) != len(after) {
				t.Fatal("cancel changed files")
			}
			for k, v := range before {
				if after[k] != v {
					t.Fatalf("cancel changed %s", k)
				}
			}
		})
	}
	isolateProfiles(t)
	repo := gitFixture(t)
	_, _, e := runCmd(t, []string{"connect", "team", repo}, []string{masterPassword}, nil)
	if e != nil {
		t.Fatal(e)
	}
	t.Setenv("PATH", t.TempDir())
	if text := discardOK(t, "team", nil, nil); !strings.Contains(text, "nothing to discard") {
		t.Fatal(text)
	}
}
func TestDiscardRestoresLatestRemote(t *testing.T) {
	repo, p := pendingGit(t)
	password := []byte(masterPassword)
	peer, e := profiles.OpenGit(repo, filepath.Join(t.TempDir(), "peer"))
	if e != nil {
		t.Fatal(e)
	}
	remote, e := vault.Load(peer.Path(), password)
	if e != nil {
		t.Fatal(e)
	}
	_ = remote.Set("p", "s", "b", vault.Field{Value: "LATEST-REMOTE-SECRET", Secret: true})
	if e = peer.Publish(password, remote, fastOptions()); e != nil {
		t.Fatal(e)
	}
	beforeRemote := gitTest(t, repo, "show-ref")
	wantHead := gitTest(t, repo, "rev-parse", "refs/heads/main")
	untouched := filepath.Join(p.RepoDir(), "keep.txt")
	if e = os.WriteFile(untouched, []byte("keep"), 0600); e != nil {
		t.Fatal(e)
	}
	text := discardOK(t, "team", []string{masterPassword}, []string{"yes"})
	if !strings.Contains(text, "restored remote vault") {
		t.Fatal(text)
	}
	if got := gitTest(t, p.RepoDir(), "rev-parse", "HEAD"); got != wantHead {
		t.Fatal("wrong commit restored")
	}
	if got := gitTest(t, repo, "show-ref"); got != beforeRemote {
		t.Fatal("discard changed remote refs")
	}
	result, e := vault.Load(filepath.Join(p.RepoDir(), "vault.age"), password)
	if e != nil {
		t.Fatal(e)
	}
	a, _ := result.GetField("p", "s", "a")
	b, _ := result.GetField("p", "s", "b")
	if a.Value != "old" || b.Value != "LATEST-REMOTE-SECRET" {
		t.Fatal("discard did not use latest remote")
	}
	for _, path := range append(recoveryPaths(p), filepath.Join(p.RepoDir(), "vault.age.bak")) {
		if _, e = os.Lstat(path); !os.IsNotExist(e) {
			t.Fatalf("left file %s: %v", path, e)
		}
	}
	if _, e = os.Stat(untouched); e != nil {
		t.Fatal("unrelated untracked file removed")
	}
	c, _ := profiles.Load()
	if c.Default != "team" || c.Vaults["team"] != p {
		t.Fatal("discard changed profile/default")
	}
	if dirty := gitTest(t, p.RepoDir(), "status", "--porcelain", "--untracked-files=no"); dirty != "" {
		t.Fatalf("dirty checkout %s", dirty)
	}
	// The profile is usable again after discard.
	_, _, e = runCmd(t, []string{"--vault", "team", "ls"}, []string{masterPassword}, nil)
	if e != nil {
		t.Fatal(e)
	}
}
func TestDiscardFailurePreservesLocalData(t *testing.T) {
	for _, mode := range []string{"offline", "wrong-password", "corrupt", "missing", "symlink", "missing-branch"} {
		t.Run(mode, func(t *testing.T) {
			repo, p := pendingGit(t)
			before := pendingContents(t, p)
			head := gitTest(t, p.RepoDir(), "rev-parse", "HEAD")
			local, e := os.ReadFile(filepath.Join(p.RepoDir(), "vault.age"))
			if e != nil {
				t.Fatal(e)
			}
			passwords := []string{masterPassword}
			switch mode {
			case "offline":
				if e = os.Rename(repo, repo+".unavailable"); e != nil {
					t.Fatal(e)
				}
				passwords = nil
			case "wrong-password":
				passwords = []string{"WRONG-MASTER-PASSWORD"}
			case "missing-branch":
				h := gitTest(t, repo, "rev-parse", "refs/heads/main")
				gitTest(t, repo, "update-ref", "refs/tags/keep", h)
				gitTest(t, repo, "update-ref", "-d", "refs/heads/main")
				passwords = nil
			default:
				peer, e := profiles.OpenGit(repo, filepath.Join(t.TempDir(), "peer"))
				if e != nil {
					t.Fatal(e)
				}
				if mode == "corrupt" {
					if e = os.WriteFile(peer.Path(), []byte("CORRUPT-SECRET-DO-NOT-PRINT"), 0600); e != nil {
						t.Fatal(e)
					}
				} else {
					if e = os.Remove(peer.Path()); e != nil {
						t.Fatal(e)
					}
					if mode == "symlink" {
						if e = os.Symlink("../../outside", peer.Path()); e != nil {
							t.Fatal(e)
						}
					}
				}
				gitTest(t, peer.Dir, "add", "-A")
				gitTest(t, peer.Dir, "commit", "-m", "Change remote fixture")
				gitTest(t, peer.Dir, "push", "origin", "main")
				if mode == "missing" || mode == "symlink" {
					passwords = nil
				}
			}
			out, errout, e := runCmd(t, []string{"vault", "discard", "team"}, passwords, []string{"y"})
			if e == nil {
				t.Fatal("unsafe discard succeeded")
			}
			if mode == "wrong-password" && !errors.Is(e, vault.ErrWrongPassword) {
				t.Fatal(e)
			}
			assertNoLeak(t, out+errout+e.Error(), "LOCAL-DISCARD-SECRET", "CORRUPT-SECRET-DO-NOT-PRINT", "WRONG-MASTER-PASSWORD", masterPassword)
			assertPendingUnchanged(t, before)
			if got := gitTest(t, p.RepoDir(), "rev-parse", "HEAD"); got != head {
				t.Fatal("failed validation reset checkout")
			}
			after, e := os.ReadFile(filepath.Join(p.RepoDir(), "vault.age"))
			if e != nil || !bytes.Equal(after, local) {
				t.Fatal("failed validation changed local vault")
			}
		})
	}
}
func TestDiscardInvalidOrBusyProfiles(t *testing.T) {
	isolateProfiles(t)
	_, _, e := runCmd(t, []string{"vault", "discard", "unknown"}, nil, nil)
	if e == nil {
		t.Fatal("unknown accepted")
	}
	_, _, e = runCmd(t, []string{"init", "personal"}, []string{masterPassword, masterPassword}, nil)
	if e != nil {
		t.Fatal(e)
	}
	_, _, e = runCmd(t, []string{"vault", "discard", "personal"}, nil, nil)
	if e == nil || !strings.Contains(e.Error(), "Git vault") {
		t.Fatal(e)
	}
	_, p := pendingGit(t)
	release, e := profiles.Lock(p.Path + ".lock")
	if e != nil {
		t.Fatal(e)
	}
	defer release()
	_, _, e = runCmd(t, []string{"vault", "discard", "team"}, nil, nil)
	if e == nil || !strings.Contains(e.Error(), "busy") {
		t.Fatal(e)
	}
}
func unfinishedSetup(t *testing.T) (string, profiles.Profile) {
	t.Helper()
	isolateProfiles(t)
	_ = gitFixture(t)
	repo := emptyRemote(t)
	_, _, e := runCmd(t, []string{"init", "personal"}, []string{masterPassword, masterPassword}, nil)
	if e != nil {
		t.Fatal(e)
	}
	if e = setDefaultProfile("personal"); e != nil {
		t.Fatal(e)
	}
	original := blockPush(t)
	if _, e = runWizard(t, setupShared, []string{masterPassword, masterPassword}, []string{"team", repo}); e == nil {
		t.Fatal("first push succeeded")
	}
	t.Setenv("PATH", original)
	c, _ := profiles.Load()
	return repo, c.Vaults["team"]
}
func TestDiscardUnfinishedSetup(t *testing.T) {
	for _, mode := range []string{"empty", "peer", "wrong-peer-password"} {
		t.Run(mode, func(t *testing.T) {
			repo, p := unfinishedSetup(t)
			passwords := []string{}
			if mode != "empty" {
				peer, e := profiles.OpenGit(repo, filepath.Join(t.TempDir(), "peer"))
				if e != nil {
					t.Fatal(e)
				}
				v := vault.New()
				_ = v.Add("peer", "s", "f", vault.Field{Value: "REMOTE-SECRET", Secret: true})
				if e = peer.Publish([]byte("new-remote-master-password"), v, fastOptions()); e != nil {
					t.Fatal(e)
				}
				passwords = []string{"new-remote-master-password"}
				if mode == "wrong-peer-password" {
					passwords = []string{masterPassword}
				}
			}
			beforeRemote := gitTest(t, repo, "for-each-ref", "--format=%(objectname) %(refname)")
			_, _, e := runCmd(t, []string{"vault", "discard", "team"}, passwords, []string{"y"})
			if mode == "wrong-peer-password" {
				if !errors.Is(e, vault.ErrWrongPassword) {
					t.Fatal(e)
				}
				c, _ := profiles.Load()
				if !c.Vaults["team"].Initializing {
					t.Fatal("setup state lost")
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			c, _ := profiles.Load()
			if c.Default != "personal" {
				t.Fatal("default changed")
			}
			if mode == "empty" {
				if _, ok := c.Vaults["team"]; ok {
					t.Fatal("unfinished profile retained")
				}
				if _, e = os.Stat(filepath.Dir(p.Path)); !os.IsNotExist(e) {
					t.Fatal("unfinished checkout retained")
				}
			} else {
				if c.Vaults["team"].Initializing || c.Vaults["team"].Type != "git" {
					t.Fatal("existing remote profile not retained")
				}
				if _, e = vault.Load(filepath.Join(p.RepoDir(), "vault.age"), []byte("new-remote-master-password")); e != nil {
					t.Fatal(e)
				}
			}
			if after := gitTest(t, repo, "for-each-ref", "--format=%(objectname) %(refname)"); after != beforeRemote {
				t.Fatal("remote modified")
			}
		})
	}
}
func TestCancelSetupRollsBackAndReportsCleanupFailure(t *testing.T) {
	for _, mode := range []string{"save", "cleanup"} {
		t.Run(mode, func(t *testing.T) {
			_, p := unfinishedSetup(t)
			before := pendingContents(t, p)
			failure := errors.New("injected filesystem failure")
			save := profiles.Save
			cleanup := os.RemoveAll
			if mode == "save" {
				save = func(profiles.Config) error { return failure }
			} else {
				cleanup = func(string) error { return failure }
			}
			e := cancelSetup("team", p, save, cleanup)
			operationErr := e
			if !errors.Is(e, failure) {
				t.Fatal(e)
			}
			c, _ := profiles.Load()
			if c.Default != "personal" {
				t.Fatal("default changed")
			}
			if mode == "save" {
				if c.Vaults["team"] != p {
					t.Fatal("profile lost on failed config save")
				}
				assertPendingUnchanged(t, before)
			} else {
				if _, ok := c.Vaults["team"]; ok {
					t.Fatal("profile still registered")
				}
				dirs, e := filepath.Glob(filepath.Join(filepath.Dir(filepath.Dir(p.Path)), ".discard-team-*"))
				if e != nil || len(dirs) != 1 {
					t.Fatalf("detached data %v %v", dirs, e)
				}

				if !strings.Contains(operationErr.Error(), dirs[0]) {
					t.Fatal("cleanup error omitted the retained data location")
				}
				if _, e = os.Stat(filepath.Join(dirs[0], "profile", "vault.age.pending")); e != nil {
					t.Fatal("cleanup failure lost pending data")
				}
			}
		})
	}
}
func TestDiscardResumesInterruptedCleanup(t *testing.T) {
	_, p := pendingGit(t)
	base := p.Path + ".pending.base"
	if e := os.Remove(base); e != nil {
		t.Fatal(e)
	}
	if e := os.Mkdir(base, 0700); e != nil {
		t.Fatal(e)
	}
	blocker := filepath.Join(base, "blocker")
	if e := os.WriteFile(blocker, []byte("keep"), 0600); e != nil {
		t.Fatal(e)
	}
	_, _, e := runCmd(t, []string{"vault", "discard", "team"}, []string{masterPassword}, []string{"y"})
	if e == nil || !strings.Contains(e.Error(), "cleanup is incomplete") {
		t.Fatal(e)
	}
	if _, e = os.Stat(p.Path + ".pending"); e != nil {
		t.Fatal("primary change removed before cleanup completed")
	}
	if e = os.Remove(blocker); e != nil {
		t.Fatal(e)
	}
	if e = os.Remove(base); e != nil {
		t.Fatal(e)
	}
	discardOK(t, "team", []string{masterPassword}, []string{"y"})
	if remaining, e := hasRecovery(p); e != nil || remaining {
		t.Fatalf("remaining %v %v", remaining, e)
	}
}

func TestRemoteSnapshotPreservesBinaryAndPinnedCommit(t *testing.T) {
	repo, p := pendingGit(t)
	peer, e := profiles.OpenGit(repo, filepath.Join(t.TempDir(), "peer"))
	if e != nil {
		t.Fatal(e)
	}
	// Deliberately include leading/trailing whitespace and NULs: the transport
	// helper must not trim arbitrary ciphertext, even if validation will reject it.
	raw := []byte{' ', '\n', 0, 0xff, 'b', 'i', 'n', 'a', 'r', 'y', 0, '\r', '\n', ' '}
	if e = os.WriteFile(peer.Path(), raw, 0600); e != nil {
		t.Fatal(e)
	}
	gitTest(t, peer.Dir, "add", "vault.age")
	gitTest(t, peer.Dir, "commit", "-m", "Binary snapshot fixture")
	gitTest(t, peer.Dir, "push", "origin", "main")
	g, e := profiles.OpenGit(repo, p.RepoDir())
	if e != nil {
		t.Fatal(e)
	}
	headBefore := gitTest(t, g.Dir, "rev-parse", "HEAD")
	snapshot, e := g.FetchSnapshot()
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(snapshot.Ciphertext, raw) {
		t.Fatalf("binary changed: %x", snapshot.Ciphertext)
	}
	if gitTest(t, g.Dir, "rev-parse", "HEAD") != headBefore {
		t.Fatal("fetch reset local HEAD")
	}
	// Advancing the remote after fetching must not change which snapshot is restored.
	if e = os.WriteFile(peer.Path(), []byte("later remote content"), 0600); e != nil {
		t.Fatal(e)
	}
	gitTest(t, peer.Dir, "add", "vault.age")
	gitTest(t, peer.Dir, "commit", "-m", "Later snapshot")
	gitTest(t, peer.Dir, "push", "origin", "main")
	if e = g.ResetSnapshot(snapshot); e != nil {
		t.Fatal(e)
	}
	restored, e := os.ReadFile(g.Path())
	if e != nil || !bytes.Equal(restored, raw) {
		t.Fatal("did not restore pinned snapshot")
	}
}

func TestDiscardKeepsTrackedRemoteBackup(t *testing.T) {
	repo, p := pendingGit(t)
	peer, e := profiles.OpenGit(repo, filepath.Join(t.TempDir(), "peer"))
	if e != nil {
		t.Fatal(e)
	}
	backup := []byte("remote-tracked-backup")
	if e = os.WriteFile(filepath.Join(peer.Dir, "vault.age.bak"), backup, 0600); e != nil {
		t.Fatal(e)
	}
	gitTest(t, peer.Dir, "add", "vault.age.bak")
	gitTest(t, peer.Dir, "commit", "-m", "Tracked backup fixture")
	gitTest(t, peer.Dir, "push", "origin", "main")
	discardOK(t, "team", []string{masterPassword}, []string{"y"})
	restored, e := os.ReadFile(filepath.Join(p.RepoDir(), "vault.age.bak"))
	if e != nil || !bytes.Equal(restored, backup) {
		t.Fatal("tracked remote backup removed")
	}
	if dirty := gitTest(t, p.RepoDir(), "status", "--porcelain", "--untracked-files=no"); dirty != "" {
		t.Fatalf("tracked files changed: %s", dirty)
	}
}

func TestDiscardProgressStopsForPromptsAndValidationFailure(t *testing.T) {
	for _, password := range []string{masterPassword, "wrong-password"} {
		t.Run(password, func(t *testing.T) {
			_, _ = pendingGit(t)
			active := false
			phases := []string{}
			var output bytes.Buffer
			script := &scriptPrompter{passwords: []string{password}, lines: []string{"y"}}
			app := &App{Out: &output, Err: &output, Prompter: &idlePrompter{t, script, &active}, progressObserver: phaseRecorder(t, &active, &phases)}
			e := app.Execute([]string{"vault", "discard", "team"})
			if password == masterPassword && e != nil {
				t.Fatal(e)
			}
			if password != masterPassword && !errors.Is(e, vault.ErrWrongPassword) {
				t.Fatal(e)
			}
			if active {
				t.Fatal("discard left progress active")
			}
			if e = script.remaining(); e != nil {
				t.Fatal(e)
			}
			if !strings.Contains(strings.Join(phases, ","), "Validating remote vault") {
				t.Fatal("no validation phase")
			}
			assertNoLeak(t, output.String(), password, "LOCAL-DISCARD-SECRET")
		})
	}
}

func TestDiscardHandlesOnlyLeftoverBase(t *testing.T) {
	_, p := pendingGit(t)
	for _, path := range []string{p.Path + ".pending", p.Path + ".pending.bak", p.Path + ".pending.base.bak"} {
		if e := os.Remove(path); e != nil {
			t.Fatal(e)
		}
	}
	text := statusOK(t, "--vault", "team", "status")
	if !strings.Contains(text, "cred vault discard team") {
		t.Fatal("leftover recovery missing from status")
	}
	discardOK(t, "team", []string{masterPassword}, []string{"y"})
	if remaining, e := hasRecovery(p); remaining || e != nil {
		t.Fatalf("remaining %v %v", remaining, e)
	}
}
