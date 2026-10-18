package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/emircancakmakk/cred/internal/crypto"
	"github.com/emircancakmakk/cred/internal/vault"
)

func (a *App) addCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <project> [service]",
		Short: "Add fields to a service",
		Long: `Add one or more fields to a project and service.

The service name is prompted when it is omitted. Field values are read
hidden and are never command arguments. An empty field name finishes the
list. Names must be exact; a prefix is a different name.

The file is written once, after every field in this run is accepted.
An existing field is not overwritten.`,
		Example: `  cred add my-project vercel
  cred add my-project`,
		Args: cobra.RangeArgs(1, 2),
		RunE: a.runAdd,
	}
}

func (a *App) runAdd(_ *cobra.Command, args []string) error {
	loaded, path, password, err := a.unlock()
	if err != nil {
		return err
	}
	defer crypto.Wipe(password)

	prompter, err := a.prompter()
	if err != nil {
		return err
	}
	project := args[0]
	service := ""
	if len(args) == 2 {
		service = args[1]
	} else {
		service, err = prompter.ReadLine(promptService)
		if err != nil {
			return err
		}
	}

	added := 0
	for {
		name, err := prompter.ReadLine(promptField)
		if err != nil {
			return err
		}
		if name == "" {
			break
		}
		field, err := readField(prompter, nil)
		if err != nil {
			return err
		}
		if err := loaded.Add(project, service, name, field); err != nil {
			if errors.Is(err, vault.ErrFieldExists) {
				return fmt.Errorf("%s/%s/%s already exists. To change it: cred set %s %s %s", project, service, name, project, service, name)
			}
			return fmt.Errorf("add %s/%s/%s: %w", project, service, name, err)
		}
		added++
	}
	if added == 0 {
		fmt.Fprintln(a.errWriter(), "no fields added")
		return nil
	}
	if err := a.save(path, password, loaded); err != nil {
		return err
	}
	if added == 1 {
		fmt.Fprintln(a.errWriter(), "added 1 field")
		return nil
	}
	fmt.Fprintf(a.errWriter(), "added %d fields\n", added)
	return nil
}

func readField(prompter Prompter, current *bool) (vault.Field, error) {
	value, err := prompter.ReadPassword(promptValue)
	if err != nil {
		return vault.Field{}, err
	}
	defer crypto.Wipe(value)
	answer, err := prompter.ReadLine(secretPrompt(current))
	if err != nil {
		return vault.Field{}, err
	}
	secret, err := parseSecret(answer, current)
	if err != nil {
		return vault.Field{}, err
	}
	return vault.Field{Value: string(value), Secret: secret}, nil
}

func fieldLabel(project, service, field string) string {
	parts := []string{project}
	if service != "" {
		parts = append(parts, service)
	}
	if field != "" {
		parts = append(parts, field)
	}
	return strings.Join(parts, "/")
}
