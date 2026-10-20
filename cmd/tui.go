package cmd

import (
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/emircancakmakk/cred/internal/crypto"
	"github.com/emircancakmakk/cred/internal/tui"
	"github.com/emircancakmakk/cred/internal/vault"
)

const tuiCommand = "__tui"

func (a *App) tuiCmd() *cobra.Command {
	return &cobra.Command{
		Use:    tuiCommand + " <project>",
		Short:  "Choose a service and field",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE:   a.runTUI,
	}
}

func (a *App) runTUI(_ *cobra.Command, args []string) error {
	loaded, path, password, err := a.unlock()
	if err != nil {
		return err
	}
	defer crypto.Wipe(password)

	start := ""
	if len(args) == 1 {
		start, err = loaded.ResolveProject(args[0])
		if err != nil {
			return err
		}
	}
	if a.RunTUI != nil {
		return a.RunTUI(start)
	}
	file, ok := a.In.(*os.File)
	if !ok || file == nil || !term.IsTerminal(int(file.Fd())) {
		return errors.New("interactive terminal required")
	}
	copier, err := a.copier()
	if err != nil {
		return err
	}

	label, err := tui.RunEditable(projectsOf(loaded), start, copier, a.tuiCommit(loaded, path, password), file, a.outWriter(), a.activeLabel())
	if err != nil {
		return err
	}
	if label != "" {
		fmt.Fprintf(a.humanErr(), "copied %s\n", label)
	}
	return nil
}

func projectsOf(v *vault.Vault) []tui.Project {
	if v == nil || v.Projects == nil {
		return nil
	}
	names := make([]string, 0, len(v.Projects))
	for name := range v.Projects {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]tui.Project, 0, len(names))
	for _, name := range names {
		out = append(out, tui.Project{Name: name, Services: servicesOf(v, name)})
	}
	return out
}

func servicesOf(v *vault.Vault, project string) []tui.Service {
	projectEntry, ok := v.Projects[project]
	if !ok || projectEntry.Services == nil {
		return nil
	}
	names := make([]string, 0, len(projectEntry.Services))
	for name := range projectEntry.Services {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]tui.Service, 0, len(names))
	for _, name := range names {
		service := projectEntry.Services[name]
		fieldNames := make([]string, 0, len(service.Fields))
		for fieldName := range service.Fields {
			fieldNames = append(fieldNames, fieldName)
		}
		sort.Strings(fieldNames)
		fields := make([]tui.Field, 0, len(fieldNames))
		for _, fieldName := range fieldNames {
			field := service.Fields[fieldName]
			fields = append(fields, tui.Field{
				Name:   fieldName,
				Value:  field.Value,
				Secret: field.Secret,
			})
		}
		out = append(out, tui.Service{Name: name, Fields: fields})
	}
	return out
}
