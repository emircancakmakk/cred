package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/emircancakmakk/cred/internal/profiles"
	"github.com/emircancakmakk/cred/internal/tui"
	"github.com/emircancakmakk/cred/internal/vault"
)

func TestTUIEmailCommitPersistsWithoutChangingFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.age")
	password := []byte(masterPassword)
	original := vault.New()
	_ = original.Add("p", "s", "email", vault.Field{Value: "personal@example.com"})
	if err := vault.Save(path, password, original, fastOptions()); err != nil {
		t.Fatal(err)
	}
	app := &App{WorkFactor: 1, Err: &bytes.Buffer{}}
	commit := app.tuiCommit(original, path, password)
	before, _ := os.ReadFile(path)
	if _, err := commit(tui.Change{Kind: tui.EmailKind, Action: tui.Create, Value: "new@example.com;invalid"}); err == nil {
		t.Fatal("invalid batch saved")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("validation changed persisted vault")
	}
	old, err := vault.Load(path, password)
	if err != nil || old.Version != 1 {
		t.Fatal("failed save upgraded vault")
	}
	snapshot, err := commit(tui.Change{Kind: tui.EmailKind, Action: tui.Create, Value: "personal@example.com;work@example.com;PERSONAL@example.com"})
	if err != nil || len(snapshot.Emails) != 2 {
		t.Fatalf("batch save: %v", err)
	}
	// Reopen the persisted vault with a new callback, as on an application restart.
	reopened, err := vault.Load(path, password)
	if err != nil {
		t.Fatal(err)
	}
	commit = app.tuiCommit(reopened, path, password)
	id := snapshot.Emails[0].ID
	snapshot, err = commit(tui.Change{Kind: tui.EmailKind, Action: tui.Edit, Name: id, Value: "changed@example.com"})
	if err != nil || snapshot.Emails[0].Address != "changed@example.com" {
		t.Fatalf("edit: %v", err)
	}
	snapshot, err = commit(tui.Change{Kind: tui.EmailKind, Action: tui.Delete, Name: id})
	if err != nil || len(snapshot.Emails) != 1 {
		t.Fatalf("delete: %v", err)
	}
	disk, err := vault.Load(path, password)
	if err != nil || disk.Version != 2 || !reflect.DeepEqual(snapshot, snapshotOf(disk)) {
		t.Fatal("snapshot differs from persisted vault")
	}
	field, _ := disk.GetField("p", "s", "email")
	if field.Value != "personal@example.com" || len(original.SavedEmails) != 0 || original.Version != 1 {
		t.Fatal("collection changed fields or original snapshot")
	}
}

func TestTUIEmailGitMergeAndRecovery(t *testing.T) {
	for _, mode := range []string{"different", "cancel-retry", "cancel-discard"} {
		t.Run(mode, func(t *testing.T) {
			isolateProfiles(t)
			repo := gitFixture(t)
			if _, _, err := runCmd(t, []string{"connect", "team", repo}, []string{masterPassword}, nil); err != nil {
				t.Fatal(err)
			}
			config, err := profiles.Load()
			if err != nil {
				t.Fatal(err)
			}
			profile := config.Vaults["team"]
			one, err := profiles.OpenGit(repo, filepath.Join(t.TempDir(), "peer"))
			if err != nil {
				t.Fatal(err)
			}
			password := []byte(masterPassword)
			seed, err := vault.Load(one.Path(), password)
			if err != nil {
				t.Fatal(err)
			}
			_, err = seed.AddSavedEmails("one@example.com;two@example.com")
			if err != nil {
				t.Fatal(err)
			}
			if err = one.Publish(password, seed, fastOptions()); err != nil {
				t.Fatal(err)
			}
			two, err := profiles.OpenGit(repo, profile.RepoDir())
			if err != nil {
				t.Fatal(err)
			}
			if err = two.Update(false); err != nil {
				t.Fatal(err)
			}
			base, err := vault.Load(two.Path(), password)
			if err != nil {
				t.Fatal(err)
			}
			emails := snapshotOf(base).Emails
			remote := vault.Clone(base)
			remoteID := emails[0].ID
			if mode == "different" {
				remoteID = emails[1].ID
			}
			if err = remote.EditSavedEmail(remoteID, "team@example.com"); err != nil {
				t.Fatal(err)
			}
			_ = remote.Set("p", "s", "b", vault.Field{Value: "remote-field", Secret: true})
			if err = one.Publish(password, remote, fastOptions()); err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			app := &App{gitSession: two, base: vault.Clone(base), profile: profile, vaultName: "team", WorkFactor: 1, Prompter: &scriptPrompter{lines: []string{"3"}}, Err: &output}
			commit := app.tuiCommit(base, two.Path(), password)
			change := tui.Change{Kind: tui.EmailKind, Action: tui.Edit, Name: emails[0].ID, Value: "mine@example.com"}
			snapshot, err := commit(change)
			if mode == "different" {
				if err != nil || len(snapshot.Emails) != 2 || snapshot.Emails[0].Address != "mine@example.com" || snapshot.Emails[1].Address != "team@example.com" {
					t.Fatalf("merged snapshot: %+v %v", snapshot.Emails, err)
				}
				if app.base.Projects["p"].Services["s"].Fields["b"].Value != "remote-field" || app.base.SavedEmails[emails[0].ID] != "mine@example.com" {
					t.Fatal("merge base not refreshed")
				}
				change.Value = "second@example.com"
				if _, err = commit(change); err != nil {
					t.Fatalf("subsequent save: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "merge cancelled") {
				t.Fatalf("cancel did not fail: %v", err)
			}
			pending := profile.Path + ".pending"
			retained, err := vault.Load(pending, password)
			if err != nil || retained.SavedEmails[emails[0].ID] != "mine@example.com" || retained.Version != 2 {
				t.Fatal("v2 pending change lost")
			}
			before, err := os.ReadFile(pending)
			if err != nil {
				t.Fatal(err)
			}
			change.Value = "overwrite@example.com"
			if _, err = commit(change); err == nil {
				t.Fatal("pending recovery was overwritten")
			}
			after, _ := os.ReadFile(pending)
			if !bytes.Equal(before, after) {
				t.Fatal("pending snapshot changed")
			}
			args, lines := []string{"vault", "retry", "team"}, []string{"2"}
			if mode == "cancel-discard" {
				args, lines = []string{"vault", "discard", "team"}, []string{"y"}
			}
			if _, _, err = runCmd(t, args, []string{masterPassword}, lines); err != nil {
				t.Fatalf("recovery: %v", err)
			}
			got, err := vault.Load(two.Path(), password)
			if err != nil {
				t.Fatal(err)
			}
			want := "mine@example.com"
			if mode == "cancel-discard" {
				want = "team@example.com"
			}
			if got.SavedEmails[emails[0].ID] != want {
				t.Fatalf("recovered wrong address: %s", got.SavedEmails[emails[0].ID])
			}
			if _, err = os.Stat(pending); !os.IsNotExist(err) {
				t.Fatal("recovery not cleared")
			}
			if strings.Contains(output.String(), "mine@example.com") || strings.Contains(output.String(), "team@example.com") {
				t.Fatal("conflict output leaked address values")
			}
		})
	}
}
