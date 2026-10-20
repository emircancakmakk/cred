package cmd

import (
	"fmt"
	"reflect"

	"github.com/emircancakmakk/cred/internal/tui"
	"github.com/emircancakmakk/cred/internal/vault"
)

// Work on a clone so a validation or persistence failure cannot alter the
// browser's last successful snapshot. The password stays inside the cmd layer.
func (a *App) tuiCommit(loaded *vault.Vault, path string, password []byte) tui.Commit {
	return func(change tui.Change) ([]tui.Project, error) {
		if a.gitSession != nil {
			pending, err := hasRecovery(a.profile)
			if err != nil {
				return nil, err
			}
			if pending {
				return nil, fmt.Errorf("unshared change retained; run cred vault retry %s or cred vault discard %s before saving again", a.activeName(), a.activeName())
			}
		}
		candidate := vault.Clone(loaded)
		if err := applyTUIChange(candidate, change); err != nil {
			return nil, err
		}
		if reflect.DeepEqual(candidate, loaded) {
			return projectsOf(loaded), nil
		}
		if err := a.save(path, password, candidate); err != nil {
			return nil, err
		}
		if a.gitSession != nil {
			// A successful concurrent merge can include team edits or reject a local
			// choice. Display the actual published vault and use it as the next base.
			current, err := vault.Load(path, password)
			if err != nil {
				return nil, fmt.Errorf("saved, but could not refresh vault: %w", err)
			}
			candidate = current
			a.base = vault.Clone(current)
		}
		loaded = candidate
		return projectsOf(loaded), nil
	}
}

func applyTUIChange(v *vault.Vault, c tui.Change) error {
	switch c.Kind {
	case tui.ProjectKind:
		switch c.Action {
		case tui.Create:
			return v.CreateProject(c.NewName)
		case tui.Edit:
			return v.RenameProject(c.Name, c.NewName)
		case tui.Delete:
			return v.DeleteProject(c.Name)
		}
	case tui.ServiceKind:
		switch c.Action {
		case tui.Create:
			return v.CreateService(c.Project, c.NewName)
		case tui.Edit:
			return v.RenameService(c.Project, c.Name, c.NewName)
		case tui.Delete:
			return v.DeleteService(c.Project, c.Name)
		}
	case tui.FieldKind:
		f := vault.Field{Value: c.Value, Secret: c.Secret}
		switch c.Action {
		case tui.Create:
			return v.Add(c.Project, c.Service, c.NewName, f)
		case tui.Edit:
			if err := v.RenameField(c.Project, c.Service, c.Name, c.NewName); err != nil {
				return err
			}
			return v.Set(c.Project, c.Service, c.NewName, f)
		case tui.Delete:
			return v.DeleteField(c.Project, c.Service, c.Name)
		}
	}
	return fmt.Errorf("unsupported vault change")
}
