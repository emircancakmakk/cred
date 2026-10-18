package cmd

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/emircancakmakk/cred/internal/crypto"
	"github.com/emircancakmakk/cred/internal/vault"
)

func (a *App) lsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List projects and services",
		Long:  "List every project and service. Field values are not printed.",
		Args:  cobra.NoArgs,
		RunE:  a.runLs,
	}
}

func (a *App) runLs(_ *cobra.Command, _ []string) error {
	loaded, _, password, err := a.unlock()
	if err != nil {
		return err
	}
	defer crypto.Wipe(password)
	_, err = io.WriteString(a.outWriter(), renderVault(loaded))
	return err
}

func renderVault(v *vault.Vault) string {
	if v == nil || len(v.Projects) == 0 {
		return "vault is empty\n"
	}
	projects := make([]string, 0, len(v.Projects))
	for name := range v.Projects {
		projects = append(projects, name)
	}
	sort.Strings(projects)

	var b strings.Builder
	for _, project := range projects {
		fmt.Fprintf(&b, "%s\n", project)
		services := v.Projects[project].Services
		names := make([]string, 0, len(services))
		for name := range services {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, service := range names {
			fmt.Fprintf(&b, "  %s\n", service)
		}
	}
	return b.String()
}
