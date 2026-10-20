package vault

import "fmt"

// availableName applies the same naming and case rules as Add, excluding the
// source during a rename. Existing siblings are never overwritten.
func availableName[T any](kind, name, source string, entries map[string]T) error {
	if err := validateName(kind, name); err != nil {
		return err
	}
	for existing := range entries {
		if existing == source {
			continue
		}
		if existing == name {
			return fmt.Errorf("%s already exists", kind)
		}
		if conflict, ok := caseConflict([]string{existing}, name); ok {
			return &CaseConflictError{Kind: kind, Existing: conflict, Given: name}
		}
	}
	return nil
}

func (v *Vault) CreateProject(name string) error {
	if err := v.ensure(); err != nil {
		return err
	}
	if err := availableName("project", name, "", v.Projects); err != nil {
		return err
	}
	v.Projects[name] = Project{Services: map[string]Service{}}
	return nil
}

func (v *Vault) CreateService(project, name string) error {
	if err := v.ensure(); err != nil {
		return err
	}
	p, ok := v.Projects[project]
	if !ok {
		return ErrProjectNotFound
	}
	if err := availableName("service", name, "", p.Services); err != nil {
		return err
	}
	if p.Services == nil {
		p.Services = map[string]Service{}
	}
	p.Services[name] = Service{Fields: map[string]Field{}}
	v.Projects[project] = p
	return nil
}

func (v *Vault) RenameProject(old, name string) error {
	if err := v.ensure(); err != nil {
		return err
	}
	p, ok := v.Projects[old]
	if !ok {
		return ErrProjectNotFound
	}
	if err := availableName("project", name, old, v.Projects); err != nil {
		return err
	}
	delete(v.Projects, old)
	v.Projects[name] = p
	return nil
}

func (v *Vault) RenameService(project, old, name string) error {
	if err := v.ensure(); err != nil {
		return err
	}
	p, ok := v.Projects[project]
	if !ok {
		return ErrProjectNotFound
	}
	s, ok := p.Services[old]
	if !ok {
		return ErrServiceNotFound
	}
	if err := availableName("service", name, old, p.Services); err != nil {
		return err
	}
	delete(p.Services, old)
	p.Services[name] = s
	return nil
}

func (v *Vault) RenameField(project, service, old, name string) error {
	if err := v.ensure(); err != nil {
		return err
	}
	p, ok := v.Projects[project]
	if !ok {
		return ErrProjectNotFound
	}
	s, ok := p.Services[service]
	if !ok {
		return ErrServiceNotFound
	}
	f, ok := s.Fields[old]
	if !ok {
		return ErrFieldNotFound
	}
	if err := availableName("field", name, old, s.Fields); err != nil {
		return err
	}
	delete(s.Fields, old)
	s.Fields[name] = f
	return nil
}
