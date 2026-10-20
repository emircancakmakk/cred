package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/emircancakmakk/cred/internal/crypto"
	"github.com/emircancakmakk/cred/internal/profiles"
	"github.com/emircancakmakk/cred/internal/vault"
	"github.com/spf13/cobra"
)

func recoveryPaths(p profiles.Profile) []string {
	// Keep the primary change until last, so interrupted cleanup remains visible.
	return []string{p.Path + ".pending.base.bak", p.Path + ".pending.base", p.Path + ".pending.bak", p.Path + ".pending"}
}
func hasRecovery(p profiles.Profile) (bool, error) {
	for _, path := range recoveryPaths(p) {
		if _, e := os.Lstat(path); e == nil {
			return true, nil
		} else if !errors.Is(e, os.ErrNotExist) {
			return false, e
		}
	}
	return false, nil
}
func (a *App) discardCmd() *cobra.Command {
	return &cobra.Command{Use: "discard <name>", Short: "Discard pending Git changes and restore the remote vault", Long: `Discard a Git profile's pending changes after confirmation. The default is no.
The remote vault must be reachable and open with its master password before any
pending changes are removed. No changes are pushed.

If the first push never completed and the remote repository is still empty,
confirmation allows removing the unfinished local profile, checkout and recovery files.`, Args: cobra.ExactArgs(1), RunE: a.runDiscard}
}
func (a *App) runDiscard(_ *cobra.Command, args []string) error {
	name := args[0]
	p, _, e := profiles.Select(name)
	if e != nil {
		return e
	}
	if p.Type != "git" {
		return errors.New("discard requires a Git vault")
	}
	a.release, e = profiles.Lock(p.Path + ".lock")
	if e != nil {
		return e
	}
	// Another process may have removed/recreated the profile before our lock.
	current, _, e := profiles.Select(name)
	if e != nil {
		return e
	}
	if current != p {
		return errors.New("profile changed; run discard again")
	}
	pending, e := hasRecovery(p)
	if e != nil {
		return e
	}
	if !pending && !p.Initializing {
		fmt.Fprintln(a.humanErr(), "nothing to discard")
		return nil
	}
	prompt, e := a.prompter()
	if e != nil {
		return e
	}
	description := fmt.Sprintf("Discard pending changes for %s and restore the remote vault? Local pending changes will be removed.", name)
	question := fmt.Sprintf("Discard pending changes for %s and restore the remote vault? [y/N]: ", name)
	if p.Initializing {
		description = fmt.Sprintf("Discard unfinished setup for %s? If the remote is empty, the local profile, checkout and recovery files will be removed.", name)
		question = fmt.Sprintf("Discard unfinished setup for %s? If the remote is empty, the local profile, checkout and recovery files will be removed. [y/N]: ", name)
	}
	confirmed, e := askConfirm(prompt, question, "Discard pending changes", description, "Discard", true)
	if e != nil {
		return e
	}
	if !confirmed {
		fmt.Fprintln(a.humanErr(), "cancelled")
		return nil
	}
	g, e := profiles.OpenGit(p.Repo, p.RepoDir(), a.stage)
	if e != nil {
		return fmt.Errorf("discard stopped; local recovery data was preserved: %w", e)
	}
	snapshot, e := g.FetchSnapshot()
	if e != nil {
		return fmt.Errorf("discard stopped; local recovery data was preserved: %w", e)
	}
	defer crypto.Wipe(snapshot.Ciphertext)
	if snapshot.Empty {
		if !p.Initializing {
			return errors.New("remote repository is empty; pending changes were preserved")
		}
		stop := a.stage("Cancelling unfinished setup")
		e = cancelSetup(name, p, profiles.Save, os.RemoveAll)
		stop()
		if e != nil {
			return e
		}
		fmt.Fprintf(a.humanErr(), "cancelled unfinished setup for %s; remote repository unchanged\n", name)
		return nil
	}
	password, e := prompt.ReadPassword(promptPassword)
	defer crypto.Wipe(password)
	if e != nil {
		return e
	}
	stop := a.stage("Validating remote vault")
	_, e = vault.LoadEncrypted(snapshot.Ciphertext, password)
	stop()
	if e != nil {
		return fmt.Errorf("remote vault could not be opened; pending changes were preserved: %w", e)
	}
	if e = g.ResetSnapshot(snapshot); e != nil {
		return e
	}
	a.profile = p
	a.vaultName = name
	if e = a.completeInitialization(); e != nil {
		return fmt.Errorf("remote restored but local setup is incomplete; run cred vault discard %s again: %w", name, e)
	}
	stop = a.stage("Removing pending changes")
	e = cleanDiscard(p, g)
	stop()
	if e != nil {
		return fmt.Errorf("remote restored but cleanup is incomplete; run cred vault discard %s again: %w", name, e)
	}
	fmt.Fprintf(a.humanErr(), "discarded pending changes for %s; restored remote vault\n", name)
	return nil
}
func cleanDiscard(p profiles.Profile, g *profiles.Git) error {
	tracked, e := g.BackupTracked()
	if e != nil {
		return e
	}
	if !tracked {
		if e = removeIfPresent(filepath.Join(g.Dir, "vault.age.bak")); e != nil {
			return e
		}
	}
	for _, path := range recoveryPaths(p) {
		if e = removeIfPresent(path); e != nil {
			return e
		}
	}
	return nil
}
func removeIfPresent(path string) error {
	e := os.Remove(path)
	if errors.Is(e, os.ErrNotExist) {
		return nil
	}
	return e
}

// Rename before removing the config entry. A failed save rolls the directory
// back; once committed, cleanup only touches the detached directory.
func cancelSetup(name string, p profiles.Profile, save func(profiles.Config) error, cleanup func(string) error) error {
	expected, e := profiles.NamedPath(name)
	if e != nil {
		return e
	}
	if p.Path != expected {
		return errors.New("cannot cancel setup outside the managed profile directory")
	}
	root := filepath.Dir(expected)
	info, e := os.Lstat(root)
	if e != nil {
		return e
	}
	if !info.IsDir() {
		return errors.New("managed profile directory must not be a symbolic link")
	}
	d, e := profiles.Dir()
	if e != nil {
		return e
	}
	release, e := profiles.Lock(filepath.Join(d, "profiles.lock"))
	if e != nil {
		return e
	}
	defer release()
	c, e := profiles.Load()
	if e != nil {
		return e
	}
	if current, ok := c.Vaults[name]; !ok || current != p || !current.Initializing {
		return errors.New("profile changed; unfinished setup was preserved")
	}
	staging, e := os.MkdirTemp(filepath.Dir(root), ".discard-"+name+"-*")
	if e != nil {
		return e
	}
	detached := filepath.Join(staging, "profile")
	if e = os.Rename(root, detached); e != nil {
		_ = os.Remove(staging)
		return e
	}
	delete(c.Vaults, name)
	if e = save(c); e != nil {
		if rollback := os.Rename(detached, root); rollback != nil {
			return fmt.Errorf("profile config could not be saved (%v); directory rollback failed; data remains at %s: %w", e, detached, rollback)
		}
		_ = os.Remove(staging)
		return fmt.Errorf("profile config could not be saved; unfinished setup restored: %w", e)
	}
	if e = cleanup(staging); e != nil {
		return fmt.Errorf("profile removed, but local cleanup failed; remaining data is under %s: %w", staging, e)
	}
	return nil
}
