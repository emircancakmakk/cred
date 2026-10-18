package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/emircancakmakk/cred/internal/crypto"
	"github.com/emircancakmakk/cred/internal/vault"
)

func (a *App) rmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <project> [service] [field]",
		Short: "Delete a project, service, or field",
		Long: `Delete a project, a service, or one field after confirmation.

Names must be exact. A prefix is not accepted. The prompt defaults to no:
an empty answer does not change the vault.`,
		Example: `  cred rm my-project
  cred rm my-project vercel
  cred rm my-project vercel password`,
		Args: cobra.RangeArgs(1, 3),
		RunE: a.runRm,
	}
}

func (a *App) runRm(_ *cobra.Command, args []string) error {
	project := args[0]
	service, field := "", ""
	if len(args) > 1 {
		service = args[1]
	}
	if len(args) > 2 {
		field = args[2]
	}

	loaded, path, password, err := a.unlock()
	if err != nil {
		return err
	}
	defer crypto.Wipe(password)
	if err := ensureTarget(loaded, project, service, field); err != nil {
		return err
	}

	prompter, err := a.prompter()
	if err != nil {
		return err
	}
	label := fieldLabel(project, service, field)
	answer, err := prompter.ReadLine(fmt.Sprintf("Delete %s? [y/N]: ", label))
	if err != nil {
		return err
	}
	if !parseConfirm(answer) {
		fmt.Fprintln(a.errWriter(), "cancelled")
		return nil
	}

	switch {
	case field != "":
		err = loaded.DeleteField(project, service, field)
	case service != "":
		err = loaded.DeleteService(project, service)
	default:
		err = loaded.DeleteProject(project)
	}
	if err != nil {
		return err
	}
	if err := a.save(path, password, loaded); err != nil {
		return err
	}
	fmt.Fprintf(a.errWriter(), "deleted %s\n", label)
	return nil
}

func ensureTarget(v *vault.Vault, project, service, field string) error {
	projectRecord, ok := v.Projects[project]
	if !ok {
		return fmt.Errorf("%s: %w", project, vault.ErrProjectNotFound)
	}
	if service == "" {
		return nil
	}
	serviceRecord, ok := projectRecord.Services[service]
	if !ok {
		return fmt.Errorf("%s: %w", fieldLabel(project, service, ""), vault.ErrServiceNotFound)
	}
	if field == "" {
		return nil
	}
	if _, ok := serviceRecord.Fields[field]; !ok {
		return fmt.Errorf("%s: %w", fieldLabel(project, service, field), vault.ErrFieldNotFound)
	}
	return nil
}
