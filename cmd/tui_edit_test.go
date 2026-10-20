package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/emircancakmakk/cred/internal/profiles"
	"github.com/emircancakmakk/cred/internal/tui"
	"github.com/emircancakmakk/cred/internal/vault"
)

func TestTUICommitPersistsCRUDWithoutLosingDescendants(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.age")
	password := []byte(masterPassword)
	loaded := vault.New()
	var output bytes.Buffer
	app := &App{WorkFactor: 1, Err: &output}
	commit := app.tuiCommit(loaded, path, password)
	changes := []tui.Change{
		{Action: tui.Create, Kind: tui.ProjectKind, NewName: "p"},
		{Action: tui.Create, Kind: tui.ServiceKind, Project: "p", NewName: "s"},
		{Action: tui.Create, Kind: tui.FieldKind, Project: "p", Service: "s", NewName: "token", Value: secretValue, Secret: true},
		{Action: tui.Edit, Kind: tui.ProjectKind, Name: "p", NewName: "renamed-p"},
		{Action: tui.Edit, Kind: tui.ServiceKind, Project: "renamed-p", Name: "s", NewName: "renamed-s"},
		{Action: tui.Edit, Kind: tui.FieldKind, Project: "renamed-p", Service: "renamed-s", Name: "token", NewName: "key", Value: "changed", Secret: true},
		{Action: tui.Delete, Kind: tui.FieldKind, Project: "renamed-p", Service: "renamed-s", Name: "key"},
		{Action: tui.Delete, Kind: tui.ServiceKind, Project: "renamed-p", Name: "renamed-s"},
		{Action: tui.Delete, Kind: tui.ProjectKind, Name: "renamed-p"},
	}
	for _, change := range changes {
		snapshot, err := commit(change)
		if err != nil {
			t.Fatalf("%s %s: %v", change.Action, change.Kind, err)
		}
		disk, err := vault.Load(path, password)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(snapshot, projectsOf(disk)) {
			t.Fatal("browser differs from encrypted vault")
		}
		if change.Action == tui.Edit && change.Kind == tui.ServiceKind {
			f, err := disk.GetField("renamed-p", "renamed-s", "token")
			if err != nil || f.Value != secretValue || !f.Secret {
				t.Fatal("rename lost descendant")
			}
		}
	}
	if len(loaded.Projects) != 0 {
		t.Fatal("original snapshot was mutated")
	}
}

func TestTUICommitFailureKeepsLastSuccessfulState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blocked")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	app := &App{WorkFactor: 1, Err: &output}
	password := []byte(masterPassword)
	commit := app.tuiCommit(vault.New(), path, password)
	change := tui.Change{Action: tui.Create, Kind: tui.ProjectKind, NewName: "retry"}
	if _, err := commit(change); err == nil {
		t.Fatal("save to directory succeeded")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	snapshot, err := commit(change)
	if err != nil || len(snapshot) != 1 || snapshot[0].Name != "retry" {
		t.Fatalf("retry failed: %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = commit(change); err == nil {
		t.Fatal("duplicate overwrote existing project")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("validation error changed encrypted vault")
	}
}

func TestTUICommitRefreshesGitSnapshotAndMergeBase(t *testing.T) {
	repo := gitFixture(t)
	one, err := profiles.OpenGit(repo, filepath.Join(t.TempDir(), "one"))
	if err != nil {
		t.Fatal(err)
	}
	two, err := profiles.OpenGit(repo, filepath.Join(t.TempDir(), "two"))
	if err != nil {
		t.Fatal(err)
	}
	password := []byte(masterPassword)
	base, err := vault.Load(two.Path(), password)
	if err != nil {
		t.Fatal(err)
	}
	remote := vault.Clone(base)
	_ = remote.Set("p", "s", "b", vault.Field{Value: "team", Secret: true})
	if err = one.Publish(password, remote, fastOptions()); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	app := &App{gitSession: two, base: vault.Clone(base), profile: profiles.Profile{Type: "git", Path: filepath.Join(t.TempDir(), "vault.age")}, WorkFactor: 1, Err: &output}
	commit := app.tuiCommit(base, two.Path(), password)
	c := tui.Change{Action: tui.Edit, Kind: tui.FieldKind, Project: "p", Service: "s", Name: "a", NewName: "a", Value: "mine", Secret: true}
	snapshot, err := commit(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot) != 1 || snapshot[0].Services[0].Fields[1].Value != "team" {
		t.Fatal("merged remote field missing in browser")
	}
	if app.base.Projects["p"].Services["s"].Fields["b"].Value != "team" {
		t.Fatal("merge base not refreshed")
	}
	c.Value = "second"
	if _, err = commit(c); err != nil {
		t.Fatalf("second save failed: %v", err)
	}
	current, err := vault.Load(two.Path(), password)
	if err != nil {
		t.Fatal(err)
	}
	if current.Projects["p"].Services["s"].Fields["b"].Value != "team" || current.Projects["p"].Services["s"].Fields["a"].Value != "second" {
		t.Fatal("second save lost state")
	}
}

func TestTUICommitPreservesRetainedGitChange(t *testing.T) {
	repo := gitFixture(t)
	g, err := profiles.OpenGit(repo, filepath.Join(t.TempDir(), "checkout"))
	if err != nil {
		t.Fatal(err)
	}
	password := []byte(masterPassword)
	loaded, err := vault.Load(g.Path(), password)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	app := &App{gitSession: g, base: vault.Clone(loaded), profile: profiles.Profile{Type: "git", Path: filepath.Join(t.TempDir(), "vault.age")}, WorkFactor: 1, Err: &output}
	commit := app.tuiCommit(loaded, g.Path(), password)
	// An unreachable upstream causes the first save to retain encrypted recovery.
	gitTest(t, g.Dir, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing.git"))
	c := tui.Change{Action: tui.Edit, Kind: tui.FieldKind, Project: "p", Service: "s", Name: "a", NewName: "a", Value: "retained", Secret: true}
	if _, err = commit(c); err == nil {
		t.Fatal("unreachable upstream save succeeded")
	}
	pending := app.profile.Path + ".pending"
	before, err := os.ReadFile(pending)
	if err != nil {
		t.Fatal(err)
	}
	c.Value = "different"
	if _, err = commit(c); err == nil {
		t.Fatal("second save ignored retained recovery")
	}
	after, err := os.ReadFile(pending)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("second save overwrote recovery")
	}
	retained, err := vault.Load(pending, password)
	if err != nil || retained.Projects["p"].Services["s"].Fields["a"].Value != "retained" {
		t.Fatal("retained change lost")
	}
}
