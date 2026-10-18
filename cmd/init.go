package cmd

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/emircancakmakk/cred/internal/crypto"
	"github.com/emircancakmakk/cred/internal/vault"
)

func (a *App) initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create an empty vault",
		Long: `Create an empty vault encrypted with a new master password.

The password is read twice, hidden, and must be at least 12 characters.
An existing vault file is left unchanged.`,
		Args: cobra.NoArgs,
		RunE: a.runInit,
	}
}

func (a *App) runInit(_ *cobra.Command, _ []string) error {
	path, err := vault.Path()
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("vault already exists: %s", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat vault: %w", err)
	}

	prompter, err := a.prompter()
	if err != nil {
		return err
	}
	first, err := prompter.ReadPassword(promptPassword)
	if err != nil {
		return err
	}
	defer crypto.Wipe(first)
	if err := vault.ValidateMasterPassword(first); err != nil {
		return err
	}
	second, err := prompter.ReadPassword(promptRepeat)
	if err != nil {
		return err
	}
	defer crypto.Wipe(second)
	if subtle.ConstantTimeCompare(first, second) != 1 {
		return errors.New("passwords do not match")
	}
	if err := a.save(path, first, vault.New()); err != nil {
		return err
	}
	fmt.Fprintf(a.errWriter(), "created %s\n", path)
	return nil
}
