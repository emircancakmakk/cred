package cmd

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/emircancakmakk/cred/internal/crypto"
	"github.com/emircancakmakk/cred/internal/profiles"
	"github.com/emircancakmakk/cred/internal/vault"
	"github.com/spf13/cobra"
)

func (a *App) prepare() (string, error) {
	if a.sessionPath != "" {
		return a.sessionPath, nil
	}
	p, name, e := profiles.Select(a.vaultName)
	if e != nil {
		return "", e
	}
	release, e := profiles.Lock(p.Path + ".lock")
	if e != nil {
		return "", e
	}
	a.release = release
	a.profile = p
	if p.Type == "git" {
		if p.Initializing {
			return "", fmt.Errorf("shared vault setup is incomplete; run cred vault retry %s or cred vault discard %s", name, name)
		}
		remaining, e := hasRecovery(p)
		if e != nil {
			return "", e
		}
		if remaining {
			return "", fmt.Errorf("unshared change or recovery files exist for %s; run cred vault retry %s or cred vault discard %s", name, name, name)
		}

		a.gitSession, e = profiles.OpenGit(p.Repo, p.RepoDir(), a.stage)
		if e != nil {
			return "", e
		}
		if e = a.gitSession.Update(false); e != nil {
			return "", e
		}
		a.sessionPath = a.gitSession.Path()
	} else {
		a.sessionPath = p.Path
	}
	if a.vaultName != "" || p.Type == "git" {
		fmt.Fprintf(a.humanErr(), "vault: %s · %s\n", name, p.Type)
	}
	return a.sessionPath, nil
}
func (a *App) resolveConflict(path string) (bool, error) {
	p, e := a.prompter()
	if e != nil {
		return false, e
	}
	for {
		answer, e := askConflict(p, path)
		if e != nil {
			return false, e
		}
		switch strings.TrimSpace(answer) {
		case "1":
			return false, nil
		case "2":
			return true, nil
		case "", "3":
			return false, errors.New("merge cancelled; change has not been shared")
		}
	}
}
func (a *App) saveGit(password []byte, v *vault.Vault) error {
	pending := a.profile.Path + ".pending"
	// Never replace a retained base/change pair during recovery.
	if _, e := os.Stat(pending); errors.Is(e, os.ErrNotExist) {
		if a.base == nil {
			return errors.New("missing merge base")
		}
		if e := a.retainChange(pending, password, v); e != nil {
			return e
		}
	} else if e != nil {
		return e
	}
	base := a.base
	desired := vault.Clone(v)
	for attempt := 0; attempt < 3; attempt++ {
		publishErr := a.gitSession.Publish(password, desired, vault.Options{WorkFactor: a.WorkFactor})
		if publishErr == nil {
			if e := a.completeInitialization(); e != nil {
				return fmt.Errorf("vault was shared but local setup is incomplete; run cred vault retry %s or cred vault discard %s: %w", a.activeName(), a.activeName(), e)
			}
			_ = os.Remove(pending)
			_ = os.Remove(pending + ".bak")
			_ = os.Remove(pending + ".base")
			_ = os.Remove(pending + ".base.bak")
			return nil
		}
		if !profiles.IsConcurrent(publishErr) {
			return fmt.Errorf("not shared; encrypted change retained at %s; run cred vault retry %s or cred vault discard %s: %w", pending, a.activeName(), a.activeName(), publishErr)
		}
		remote, e := a.gitSession.Refresh(password)
		if e != nil {
			return fmt.Errorf("not shared; encrypted change retained at %s; run cred vault retry %s or cred vault discard %s: %w", pending, a.activeName(), a.activeName(), e)
		}
		if base == nil {
			return fmt.Errorf("not shared; encrypted change retained at %s", pending)
		}
		merged, e := a.mergeVault(base, desired, remote)
		if e != nil {
			return fmt.Errorf("not shared; encrypted change retained at %s; run cred vault retry %s or cred vault discard %s: %w", pending, a.activeName(), a.activeName(), e)
		}
		base = remote
		desired = merged
	}
	return fmt.Errorf("not shared after 3 attempts; encrypted change retained at %s; run cred vault retry %s or cred vault discard %s", pending, a.activeName(), a.activeName())
}
func (a *App) connectCmd() *cobra.Command {
	command := &cobra.Command{Use: "connect <name> <repository>", Short: "Connect an existing Git vault", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		if a.Prompter == nil {
			from, _ := cmd.Flags().GetString("from")
			return a.runSetupWizard(setupRequest{Choice: setupConnect, Fixed: true, Name: args[0], Repo: args[1], From: from})
		}
		from, _ := cmd.Flags().GetString("from")
		if _, e := a.connectProfile(args[0], args[1], from); e != nil {
			return e
		}
		fmt.Fprintf(a.humanErr(), "connected %s; select with cred vault use %s\n", args[0], args[0])
		return nil
	}}
	command.Flags().String("from", "", "publish a local vault into an empty repository")
	return command
}
func (a *App) vaultCmd() *cobra.Command {
	root := &cobra.Command{Use: "vault", Short: "Manage vault profiles"}
	root.AddCommand(a.discardCmd(), &cobra.Command{Use: "retry <name>", Short: "Retry a retained encrypted Git change", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		p, _, e := profiles.Select(args[0])
		if e != nil {
			return e
		}
		if p.Type != "git" {
			return errors.New("retry requires a Git vault")
		}
		a.release, e = profiles.Lock(p.Path + ".lock")
		if e != nil {
			return e
		}
		prompt, e := a.prompter()
		if e != nil {
			return e
		}
		password, e := prompt.ReadPassword(promptPassword)
		if e != nil {
			return e
		}
		defer crypto.Wipe(password)
		desired, e := a.loadVault(p.Path+".pending", password)
		if e != nil {
			return e
		}
		base, e := a.loadVault(p.Path+".pending.base", password)
		if e != nil {
			return e
		}
		a.gitSession, e = profiles.OpenGit(p.Repo, p.RepoDir(), a.stage)
		if e != nil {
			return e
		}
		a.profile = p
		a.vaultName = args[0]
		if p.Initializing {
			hasRefs, err := a.gitSession.HasRemoteRefs()
			if err != nil {
				return err
			}
			if !hasRefs {
				a.base = base
				if e = a.saveGit(password, desired); e != nil {
					return e
				}
				fmt.Fprintln(a.humanErr(), "shared vault setup completed")
				return nil
			}
		}
		if e = a.gitSession.Update(true); e != nil {
			return e
		}
		remote, e := a.loadVault(a.gitSession.Path(), password)
		if e != nil {
			return e
		}
		merged, e := a.mergeVault(base, desired, remote)
		if e != nil {
			return e
		}
		a.base = remote
		if e = a.saveGit(password, merged); e != nil {
			return e
		}
		fmt.Fprintln(a.humanErr(), "change shared")
		return nil
	}}, &cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		c, e := profiles.Load()
		if e != nil {
			return e
		}
		names := []string{}
		for n := range c.Vaults {
			names = append(names, n)
		}
		sort.Strings(names)
		active := c.Default
		if active == "" {
			active = "local"
		}
		marker := " "
		for _, n := range names {
			marker = " "
			if n == active {
				marker = "*"
			}
			fmt.Fprintf(a.humanOut(), "%s %s (%s)\n", marker, n, c.Vaults[n].Type)
		}
		return nil
	}}, &cobra.Command{Use: "use <name>", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		return setDefaultProfile(args[0])
	}})
	return root
}

func (a *App) activeLabel() string {
	name := a.vaultName
	if name == "" {
		c, e := profiles.Load()
		if e == nil && os.Getenv("CRED_VAULT") == "" {
			name = c.Default
		}
	}
	if name == "" {
		name = "local"
	}
	return name + " · " + a.profile.Type
}

func (a *App) retainChange(pending string, password []byte, v *vault.Vault) error {
	stop := a.stage("Encrypting recovery snapshot")
	defer stop()
	if e := vault.Save(pending+".base", password, a.base, vault.Options{WorkFactor: a.WorkFactor}); e != nil {
		return e
	}
	return vault.Save(pending, password, v, vault.Options{WorkFactor: a.WorkFactor})
}
func (a *App) activeName() string {
	if a.vaultName != "" {
		return a.vaultName
	}
	c, e := profiles.Load()
	if e == nil {
		for name, p := range c.Vaults {
			if p == a.profile {
				return name
			}
		}
	}
	return "<profile>"
}
