package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/emircancakmakk/cred/internal/profiles"
	"github.com/spf13/cobra"
)

func (a *App) initCmd() *cobra.Command {
	return &cobra.Command{Use: "init [name]", Short: "Set up a local or shared vault", Long: `Open the interactive setup menu to create a local vault, connect to an existing
team vault, or create a shared vault in an empty Git repository you provide.

cred init <name> creates a local vault directly. New passwords are read twice,
masked by default (Ctrl+R shows/hides), and must be at least 12 characters. Existing profiles are never replaced.
The interactive setup creates named profiles even when CRED_VAULT is set.`, Args: cobra.MaximumNArgs(1), RunE: a.runInit}
}
func (a *App) runInit(_ *cobra.Command, args []string) error {
	if a.Prompter == nil && a.setupMenu == nil {
		request := setupRequest{}
		if len(args) > 0 {
			request.Name = args[0]
			request.Fixed = true
		} else if a.vaultName != "" {
			request.Name = a.vaultName
			request.Fixed = true
		}
		return a.runSetupWizard(request)
	}
	if len(args) == 0 && a.vaultName == "" {
		return a.runSetup()
	}
	name := a.vaultName
	if len(args) == 1 {
		name = args[0]
	}
	p, e := a.createLocal(name)
	if e != nil {
		return e
	}
	fmt.Fprintf(a.humanErr(), "created %s\n", p.Path)
	return nil
}
func (a *App) runSetup() error {
	choice, e := a.chooseSetup()
	if errors.Is(e, errSetupCancelled) {
		fmt.Fprintln(a.humanErr(), "cancelled")
		return nil
	}
	if e != nil {
		return e
	}
	p, e := a.prompter()
	if e != nil {
		return e
	}
	prompt := "Profile name: "
	if choice == setupLocal {
		prompt = "Profile name [local]: "
	}
	name, e := p.ReadLine(prompt)
	if e != nil {
		return e
	}
	name = strings.TrimSpace(name)
	if name == "" && choice == setupLocal {
		name = "local"
	}
	if e = profiles.ValidateName(name); e != nil {
		return e
	}
	var profile profiles.Profile
	if choice == setupLocal {
		profile, e = a.createLocal(name)
	} else {
		repo, err := p.ReadLine("Git repository URL: ")
		if err != nil {
			return err
		}
		repo = strings.TrimSpace(repo)
		if repo == "" {
			return errors.New("a Git repository URL is required")
		}
		if choice == setupConnect {
			profile, e = a.connectProfile(name, repo, "")
		} else {
			profile, e = a.createShared(name, repo)
		}
	}
	if e != nil {
		return e
	}
	path := profile.Path
	if profile.Type == "git" {
		path = profile.RepoDir() + "/vault.age"
	}
	fmt.Fprintf(a.humanErr(), "Ready: %s (%s)\nVault: %s\nTry: cred --vault %s\n", name, profile.Type, path, name)
	answer, e := p.ReadLine("Use this vault by default? [y/N]: ")
	if e != nil {
		return fmt.Errorf("vault is ready; default selection was not changed: %w", e)
	}
	if parseConfirm(answer) {
		if e = setDefaultProfile(name); e != nil {
			return e
		}
		fmt.Fprintf(a.humanErr(), "default vault: %s\n", name)
		if os.Getenv("CRED_VAULT") != "" {
			fmt.Fprintln(a.humanErr(), "CRED_VAULT is set; unset CRED_VAULT to use this default without --vault.")
		}
	}
	return nil
}
