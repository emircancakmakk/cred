package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/emircancakmakk/cred/internal/profiles"
	"github.com/spf13/cobra"
)

func (a *App) statusCmd() *cobra.Command {
	command := &cobra.Command{Use: "status", Short: "Show vault and Git status without unlocking", Long: "Show the selected vault, paths and recovery state without a password or network access.\nUse --remote to check Git access and the remote branch without changing the checkout.", Args: cobra.NoArgs, RunE: a.runStatus}
	command.Flags().Bool("remote", false, "check Git access and compare the remote branch without fetching")
	return command
}
func fileState(path string) (string, error) {
	info, e := os.Lstat(path)
	if errors.Is(e, os.ErrNotExist) {
		return "missing", nil
	}
	if e != nil {
		return "", e
	}
	if !info.Mode().IsRegular() {
		return "not a regular file", nil
	}
	return "present", nil
}
func (a *App) runStatus(cmd *cobra.Command, _ []string) error {
	p, name, e := profiles.Select(a.vaultName)
	if e != nil {
		return e
	}
	remote, _ := cmd.Flags().GetBool("remote")
	if remote && p.Type != "git" {
		return errors.New("--remote requires a Git vault; select one with --vault <profile>")
	}
	source := "default profile"
	if a.vaultName != "" {
		source = "--vault"
	} else if os.Getenv("CRED_VAULT") != "" {
		source = "CRED_VAULT"
	}
	out := a.outWriter()
	path := p.Path
	if p.Type == "git" {
		path = filepath.Join(p.RepoDir(), "vault.age")
	}
	fmt.Fprintf(out, "Profile: %s\nType: %s\nSelected by: %s\nVault: %s\n", name, p.Type, source, path)
	release, busy, e := profiles.ReadLock(p.Path + ".lock")
	if e != nil {
		return e
	}
	defer release()
	if busy {
		fmt.Fprintln(out, "State: busy (another cred operation is running)")
		if remote {
			return errors.New("remote status was not checked because the vault is busy; try again after the other operation finishes")
		}
		return nil
	}
	state, e := fileState(path)
	if e != nil {
		return e
	}
	fmt.Fprintf(out, "Vault file: %s\n", state)
	if p.Type != "git" {
		if state == "missing" {
			config, e := profiles.Load()
			if e != nil && source != "CRED_VAULT" {
				return e
			}
			if configured, ok := config.Vaults[name]; ok && source != "CRED_VAULT" && configured.Path == p.Path {
				fmt.Fprintln(out, "State: vault file missing; restore it from a backup or fix the profile path")
			} else {
				fmt.Fprintln(out, "State: not initialized; create a vault with cred init")
			}
		}
		return nil
	}
	fmt.Fprintf(out, "Checkout: %s\n", p.RepoDir())
	if p.Initializing {
		fmt.Fprintln(out, "Setup: incomplete")
		fmt.Fprintf(out, "Next step: cred vault retry %s or cred vault discard %s\n", name, name)
	}
	pending, e := fileState(p.Path + ".pending")
	if e != nil {
		return e
	}
	base, e := fileState(p.Path + ".pending.base")
	if e != nil {
		return e
	}
	fmt.Fprintf(out, "Pending change: %s\nRecovery base: %s\n", pending, base)
	remaining, e := hasRecovery(p)
	if e != nil {
		return e
	}
	if remaining && !p.Initializing {
		fmt.Fprintf(out, "Next step: cred vault retry %s or cred vault discard %s\n", name, name)
	}
	snapshot, e := profiles.InspectGit(p.RepoDir())
	if e != nil {
		return e
	}
	fmt.Fprintf(out, "Checkout state: %s\n", snapshot.Checkout)
	branch := snapshot.Branch
	if branch == "" {
		branch = "unknown"
	}
	commit := snapshot.Commit
	if commit == "" {
		commit = "none"
	}
	lastFetch := "unknown"
	if !snapshot.LastFetch.IsZero() {
		lastFetch = snapshot.LastFetch.Format(time.RFC3339)
	}
	fmt.Fprintf(out, "Branch: %s\nLocal commit: %s\nLast fetch: %s\n", branch, commit, lastFetch)
	if snapshot.Checkout == "present" {
		worktree := "clean"
		if snapshot.Dirty {
			worktree = "uncommitted changes"
		}
		unpublished := "unknown"
		if snapshot.UnpublishedKnown {
			unpublished = fmt.Sprintf("%t", snapshot.Unpublished)
		}
		fmt.Fprintf(out, "Working copy: %s\nUnpublished commits (last fetched refs): %s\n", worktree, unpublished)
	}
	if !remote {
		fmt.Fprintln(out, "Remote: not checked (use --remote)")
		return nil
	}
	stop := a.stage("Checking remote")
	head, e := profiles.RemoteHead(p.Repo, snapshot.Branch)
	stop()
	if e != nil {
		return e
	}
	fmt.Fprintln(out, "Remote: reachable")
	switch {
	case snapshot.Commit == "":
		fmt.Fprintln(out, "Sync: no local commit to compare")
	case snapshot.Commit == head:
		fmt.Fprintln(out, "Sync: local and remote commits match")
	default:
		fmt.Fprintln(out, "Sync: local and remote commits differ; the next vault operation will check for updates")
	}
	return nil
}
