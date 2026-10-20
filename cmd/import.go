package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/emircancakmakk/cred/internal/crypto"
	"github.com/emircancakmakk/cred/internal/vault"
)

func (a *App) importCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import <file>",
		Short: "Add fields from a JSON file",
		Long: `Add fields from a JSON file.

The file is vault plaintext version 1: projects, services, and fields.
Every field has a value and a secret flag. An omitted secret flag is an
error. Unknown keys are rejected. An empty value is allowed.

A field that already exists is skipped, including a name that differs only
by case. Its value is left unchanged. Other fields in the file are saved.
The vault is written once, and only when at least one field is new. Names
must be exact; a prefix is a different name.

The import file stays on disk.`,
		Example: `  cred import data.json`,
		Args:    cobra.ExactArgs(1),
		RunE:    a.runImport,
	}
}

func (a *App) runImport(_ *cobra.Command, args []string) error {
	data, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("read import file: %w", err)
	}
	defer crypto.Wipe(data)

	incoming, err := vault.ParseImport(data)
	if err != nil {
		return fmt.Errorf("import file: %w", err)
	}
	if incoming.FieldCount() == 0 {
		fmt.Fprintln(a.humanErr(), "no fields added")
		return nil
	}

	loaded, path, password, err := a.unlock()
	if err != nil {
		return err
	}
	defer crypto.Wipe(password)

	result, err := loaded.Merge(incoming)
	if err != nil {
		return err
	}
	if result.Added > 0 {
		if err := a.save(path, password, loaded); err != nil {
			return err
		}
	}
	reportImport(a.humanErr(), result)
	return nil
}

func reportImport(w io.Writer, result vault.MergeResult) {
	switch result.Added {
	case 0:
		fmt.Fprintln(w, "no fields added")
	case 1:
		fmt.Fprintln(w, "added 1 field")
	default:
		fmt.Fprintf(w, "added %d fields\n", result.Added)
	}
	switch len(result.Skipped) {
	case 0:
		return
	case 1:
		fmt.Fprintln(w, "skipped 1 field")
	default:
		fmt.Fprintf(w, "skipped %d fields\n", len(result.Skipped))
	}
	for _, path := range result.Skipped {
		fmt.Fprintln(w, path)
	}
}
