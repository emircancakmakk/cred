package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/emircancakmakk/cred/internal/crypto"
)

func (a *App) setCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <project> <service> <field>",
		Short: "Replace an existing field",
		Long: `Replace the value and secret flag of an existing field.

The new value is read hidden and is never a command argument. An empty
answer to the secret prompt keeps the current flag. A missing field is
an error; set does not create records. Names must be exact.`,
		Args: cobra.ExactArgs(3),
		RunE: a.runSet,
	}
}

func (a *App) runSet(_ *cobra.Command, args []string) error {
	project, service, name := args[0], args[1], args[2]
	loaded, path, password, err := a.unlock()
	if err != nil {
		return err
	}
	defer crypto.Wipe(password)

	existing, err := loaded.GetField(project, service, name)
	if err != nil {
		return fmt.Errorf("%s: %w", fieldLabel(project, service, name), err)
	}
	prompter, err := a.prompter()
	if err != nil {
		return err
	}

	field, err := readField(prompter, &existing.Secret)
	if err != nil {
		return err
	}
	if err := loaded.Set(project, service, name, field); err != nil {
		return fmt.Errorf("%s: %w", fieldLabel(project, service, name), err)
	}
	if err := a.save(path, password, loaded); err != nil {
		return err
	}
	fmt.Fprintf(a.humanErr(), "updated %s\n", fieldLabel(project, service, name))
	return nil
}
