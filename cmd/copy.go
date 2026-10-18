package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/emircancakmakk/cred/internal/crypto"
)

const copyCommand = "__copy"

var errCopyUsage = errors.New("usage: cred <project> <service> <field> [--show]")

var knownCommands = map[string]struct{}{
	"init":       {},
	"add":        {},
	"set":        {},
	"ls":         {},
	"rm":         {},
	copyCommand:  {},
	tuiCommand:   {},
	"help":       {},
	"completion": {},
}

func (a *App) copyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    copyCommand + " <project> <service> <field>",
		Short:  "Copy a field value to the clipboard",
		Hidden: true,
		Args:   cobra.ExactArgs(3),
		RunE:   a.runCopy,
	}
	cmd.Flags().Bool("show", false, "write the value to stdout instead of copying")
	return cmd
}

func (a *App) runCopy(cmd *cobra.Command, args []string) error {
	show, err := cmd.Flags().GetBool("show")
	if err != nil {
		return err
	}
	loaded, _, password, err := a.unlock()
	if err != nil {
		return err
	}
	defer crypto.Wipe(password)

	project, err := loaded.ResolveProject(args[0])
	if err != nil {
		return err
	}
	service, err := loaded.ResolveService(project, args[1])
	if err != nil {
		return err
	}
	name, err := loaded.ResolveField(project, service, args[2])
	if err != nil {
		return err
	}
	field, err := loaded.GetField(project, service, name)
	if err != nil {
		return err
	}

	label := fieldLabel(project, service, name)
	if show {
		_, err := fmt.Fprintf(a.outWriter(), "%s\n", field.Value)
		return err
	}
	copier, err := a.copier()
	if err != nil {
		return err
	}
	if err := copier.Copy(field.Value); err != nil {
		return fmt.Errorf("copy %s: %w", label, err)
	}
	fmt.Fprintf(a.errWriter(), "copied %s\n", label)
	return nil
}

func rewriteArgs(args []string) ([]string, error) {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		return args, nil
	}
	if _, ok := knownCommands[args[0]]; ok {
		return args, nil
	}
	positionals := 0
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			continue
		}
		positionals++
	}
	switch positionals {
	case 3:
		return append([]string{copyCommand}, args...), nil
	case 1:
		return append([]string{tuiCommand}, args...), nil
	default:
		return nil, errCopyUsage
	}
}
