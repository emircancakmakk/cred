package vault

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

var ErrInvalidImport = errors.New("import file is invalid")

type MissingSecretError struct {
	Project string
	Service string
	Field   string
}

func (e *MissingSecretError) Error() string {
	return fmt.Sprintf("missing secret flag for %s/%s/%s", e.Project, e.Service, e.Field)
}

type MergeResult struct {
	Added   int
	Skipped []string
}

type importDocument struct {
	Version  int                      `json:"version"`
	Projects map[string]importProject `json:"projects"`
}

type importProject struct {
	Services map[string]importService `json:"services"`
}

type importService struct {
	Fields map[string]importField `json:"fields"`
}

type importField struct {
	Value  string `json:"value"`
	Secret *bool  `json:"secret"`
}

func ParseImport(data []byte) (*Vault, error) {
	var header struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return nil, ErrInvalidImport
	}
	if header.Version != legacyVersion {
		return nil, ErrUnsupportedVersion
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var doc importDocument
	if err := dec.Decode(&doc); err != nil {
		return nil, ErrInvalidImport
	}
	if dec.More() {
		return nil, ErrInvalidImport
	}

	loaded := New()
	for _, project := range sortedKeys(doc.Projects) {
		services := doc.Projects[project].Services
		for _, service := range sortedKeys(services) {
			fields := services[service].Fields
			for _, name := range sortedKeys(fields) {
				if err := validateName("project", project); err != nil {
					return nil, err
				}
				if err := validateName("service", service); err != nil {
					return nil, err
				}
				if err := validateName("field", name); err != nil {
					return nil, err
				}
				item := fields[name]
				if item.Secret == nil {
					return nil, &MissingSecretError{Project: project, Service: service, Field: name}
				}
				field := Field{Value: item.Value, Secret: *item.Secret}
				if err := loaded.Add(project, service, name, field); err != nil {
					return nil, err
				}
			}
		}
	}
	return loaded, nil
}

func (v *Vault) FieldCount() int {
	n := 0
	_ = eachField(v, func(_, _, _ string, _ Field) error {
		n++
		return nil
	})
	return n
}

func (v *Vault) Merge(src *Vault) (MergeResult, error) {
	if v == nil || src == nil {
		return MergeResult{}, errNilVault
	}
	next := Clone(v)
	var skipped []string
	added := 0
	err := eachField(src, func(project, service, name string, field Field) error {
		if v.alreadyPresent(project, service, name) {
			skipped = append(skipped, fieldPath(project, service, name))
			return nil
		}
		if err := next.Add(project, service, name, field); err != nil {
			return err
		}
		added++
		return nil
	})
	if err != nil {
		return MergeResult{}, err
	}
	sort.Strings(skipped)
	v.Projects = next.Projects
	return MergeResult{Added: added, Skipped: skipped}, nil
}

func (v *Vault) alreadyPresent(project, service, name string) bool {
	if _, ok := caseConflict(mapKeys(v.Projects), project); ok {
		return true
	}
	p, ok := v.Projects[project]
	if !ok {
		return false
	}
	if _, ok := caseConflict(mapKeys(p.Services), service); ok {
		return true
	}
	s, ok := p.Services[service]
	if !ok {
		return false
	}
	if _, ok := caseConflict(mapKeys(s.Fields), name); ok {
		return true
	}
	_, exists := s.Fields[name]
	return exists
}

func eachField(v *Vault, fn func(project, service, name string, field Field) error) error {
	if v == nil {
		return nil
	}
	for _, project := range sortedKeys(v.Projects) {
		services := v.Projects[project].Services
		for _, service := range sortedKeys(services) {
			fields := services[service].Fields
			for _, name := range sortedKeys(fields) {
				if err := fn(project, service, name, fields[name]); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := mapKeys(m)
	sort.Strings(keys)
	return keys
}

func fieldPath(project, service, name string) string {
	return project + "/" + service + "/" + name
}
