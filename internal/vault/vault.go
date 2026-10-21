package vault

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	legacyVersion  = 1
	currentVersion = 2
)

const minPasswordRunes = 12

var (
	ErrEmptyPassword = errors.New("master password is empty")

	ErrPasswordTooShort = errors.New("master password must be at least 12 characters")

	ErrFieldExists = errors.New("field already exists")

	ErrFieldNotFound = errors.New("field not found")

	ErrServiceNotFound = errors.New("service not found")

	ErrProjectNotFound = errors.New("project not found")

	ErrUnsupportedVersion = errors.New("unsupported vault version")

	ErrCorruptVault = errors.New("vault file is corrupt or not a cred vault")

	ErrWrongPassword = errors.New("wrong master password")

	ErrVaultNotFound = errors.New("vault file not found")

	errNilVault = errors.New("nil vault")
)

type NameError struct {
	Kind   string
	Reason string
}

func (e *NameError) Error() string {
	return fmt.Sprintf("invalid %s name: %s", e.Kind, e.Reason)
}

type CaseConflictError struct {
	Kind     string
	Existing string
	Given    string
}

func (e *CaseConflictError) Error() string {
	return fmt.Sprintf("%s name %q differs only by case from existing %q", e.Kind, e.Given, e.Existing)
}

type AmbiguousError struct {
	Kind    string
	Query   string
	Matches []string
}

func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("ambiguous %s name %q: %s", e.Kind, e.Query, strings.Join(e.Matches, ", "))
}

type Field struct {
	Value  string `json:"value"`
	Secret bool   `json:"secret"`
}

type Service struct {
	Fields map[string]Field `json:"fields"`
}

type Project struct {
	Services map[string]Service `json:"services"`
}

type Vault struct {
	Version     int                `json:"version"`
	Projects    map[string]Project `json:"projects"`
	SavedEmails map[string]string  `json:"saved_emails,omitempty"`
}

// New keeps unused collections compatible with v1 readers. Collection writes promote to v2.
func New() *Vault {
	return &Vault{
		Version:  legacyVersion,
		Projects: map[string]Project{},
	}
}

func ValidateMasterPassword(password []byte) error {
	if len(password) == 0 {
		return ErrEmptyPassword
	}
	if utf8.RuneCount(password) < minPasswordRunes {
		return ErrPasswordTooShort
	}
	return nil
}

func (v *Vault) Add(project, service, name string, field Field) error {
	if err := v.ensure(); err != nil {
		return err
	}
	if err := validateName("project", project); err != nil {
		return err
	}
	if err := validateName("service", service); err != nil {
		return err
	}
	if err := validateName("field", name); err != nil {
		return err
	}
	if existing, ok := caseConflict(mapKeys(v.Projects), project); ok {
		return &CaseConflictError{Kind: "project", Existing: existing, Given: project}
	}

	p, ok := v.Projects[project]
	if !ok {
		p = Project{Services: map[string]Service{}}
	} else if p.Services == nil {
		p.Services = map[string]Service{}
	}
	if existing, ok := caseConflict(mapKeys(p.Services), service); ok {
		return &CaseConflictError{Kind: "service", Existing: existing, Given: service}
	}
	s, ok := p.Services[service]
	if !ok {
		s = Service{Fields: map[string]Field{}}
	} else if s.Fields == nil {
		s.Fields = map[string]Field{}
	}
	if existing, ok := caseConflict(mapKeys(s.Fields), name); ok {
		return &CaseConflictError{Kind: "field", Existing: existing, Given: name}
	}
	if _, exists := s.Fields[name]; exists {
		return ErrFieldExists
	}

	s.Fields[name] = field
	p.Services[service] = s
	v.Projects[project] = p
	return nil
}

func (v *Vault) Set(project, service, name string, field Field) error {
	if err := v.ensure(); err != nil {
		return err
	}
	if err := validateName("project", project); err != nil {
		return err
	}
	if err := validateName("service", service); err != nil {
		return err
	}
	if err := validateName("field", name); err != nil {
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
	if s.Fields == nil {
		return ErrFieldNotFound
	}
	if _, ok := s.Fields[name]; !ok {
		return ErrFieldNotFound
	}
	s.Fields[name] = field
	return nil
}

func (v *Vault) DeleteField(project, service, name string) error {
	if err := v.ensure(); err != nil {
		return err
	}
	if err := validateName("project", project); err != nil {
		return err
	}
	if err := validateName("service", service); err != nil {
		return err
	}
	if err := validateName("field", name); err != nil {
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
	if s.Fields == nil {
		return ErrFieldNotFound
	}
	if _, ok := s.Fields[name]; !ok {
		return ErrFieldNotFound
	}
	delete(s.Fields, name)
	return nil
}

func (v *Vault) DeleteService(project, service string) error {
	if err := v.ensure(); err != nil {
		return err
	}
	if err := validateName("project", project); err != nil {
		return err
	}
	if err := validateName("service", service); err != nil {
		return err
	}

	p, ok := v.Projects[project]
	if !ok {
		return ErrProjectNotFound
	}
	if _, ok := p.Services[service]; !ok {
		return ErrServiceNotFound
	}
	delete(p.Services, service)
	return nil
}

func (v *Vault) DeleteProject(project string) error {
	if err := v.ensure(); err != nil {
		return err
	}
	if err := validateName("project", project); err != nil {
		return err
	}
	if _, ok := v.Projects[project]; !ok {
		return ErrProjectNotFound
	}
	delete(v.Projects, project)
	return nil
}

func (v *Vault) ResolveProject(query string) (string, error) {
	if err := v.ensure(); err != nil {
		return "", err
	}
	return resolveName("project", query, mapKeys(v.Projects))
}

func (v *Vault) ResolveService(project, query string) (string, error) {
	if err := v.ensure(); err != nil {
		return "", err
	}
	if err := validateName("project", project); err != nil {
		return "", err
	}
	p, ok := v.Projects[project]
	if !ok {
		return "", ErrProjectNotFound
	}
	if p.Services == nil {
		return "", ErrServiceNotFound
	}
	return resolveName("service", query, mapKeys(p.Services))
}

func (v *Vault) ResolveField(project, service, query string) (string, error) {
	if err := v.ensure(); err != nil {
		return "", err
	}
	if err := validateName("project", project); err != nil {
		return "", err
	}
	if err := validateName("service", service); err != nil {
		return "", err
	}
	p, ok := v.Projects[project]
	if !ok {
		return "", ErrProjectNotFound
	}
	s, ok := p.Services[service]
	if !ok {
		return "", ErrServiceNotFound
	}
	if s.Fields == nil {
		return "", ErrFieldNotFound
	}
	return resolveName("field", query, mapKeys(s.Fields))
}

func (v *Vault) GetField(project, service, name string) (Field, error) {
	if err := v.ensure(); err != nil {
		return Field{}, err
	}
	if err := validateName("project", project); err != nil {
		return Field{}, err
	}
	if err := validateName("service", service); err != nil {
		return Field{}, err
	}
	if err := validateName("field", name); err != nil {
		return Field{}, err
	}
	p, ok := v.Projects[project]
	if !ok {
		return Field{}, ErrProjectNotFound
	}
	s, ok := p.Services[service]
	if !ok {
		return Field{}, ErrServiceNotFound
	}
	field, ok := s.Fields[name]
	if !ok {
		return Field{}, ErrFieldNotFound
	}
	return field, nil
}

func (v *Vault) ensure() error {
	if v == nil {
		return errNilVault
	}
	if v.Projects == nil {
		v.Projects = map[string]Project{}
	}
	return nil
}

func (v *Vault) normalize() {
	if v.Projects == nil {
		v.Projects = map[string]Project{}
		return
	}
	for projectName, project := range v.Projects {
		if project.Services == nil {
			project.Services = map[string]Service{}
			v.Projects[projectName] = project
			continue
		}
		for serviceName, service := range project.Services {
			if service.Fields == nil {
				service.Fields = map[string]Field{}
				project.Services[serviceName] = service
			}
		}
		v.Projects[projectName] = project
	}
}

func validateName(kind, name string) error {
	if name == "" {
		return &NameError{Kind: kind, Reason: "name is empty"}
	}
	if strings.TrimSpace(name) != name {
		return &NameError{Kind: kind, Reason: "leading or trailing space"}
	}
	if strings.ContainsAny(name, `/\`) {
		return &NameError{Kind: kind, Reason: "contains a path separator"}
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return &NameError{Kind: kind, Reason: "contains a control character"}
		}
	}
	return nil
}

func caseConflict(names []string, given string) (string, bool) {
	for _, name := range names {
		if name != given && strings.EqualFold(name, given) {
			return name, true
		}
	}
	return "", false
}

func resolveName(kind, query string, names []string) (string, error) {
	if err := validateName(kind, query); err != nil {
		return "", err
	}
	for _, name := range names {
		if name == query {
			return name, nil
		}
	}

	var folded []string
	for _, name := range names {
		if strings.EqualFold(name, query) {
			folded = append(folded, name)
		}
	}
	if len(folded) == 1 {
		return folded[0], nil
	}
	if len(folded) > 1 {
		sort.Strings(folded)
		return "", &AmbiguousError{Kind: kind, Query: query, Matches: folded}
	}

	var prefixes []string
	for _, name := range names {
		if hasPrefixFold(name, query) {
			prefixes = append(prefixes, name)
		}
	}
	sort.Strings(prefixes)
	switch len(prefixes) {
	case 1:
		return prefixes[0], nil
	case 0:
		return "", notFound(kind)
	default:
		return "", &AmbiguousError{Kind: kind, Query: query, Matches: prefixes}
	}
}

func hasPrefixFold(name, prefix string) bool {
	nameRunes := []rune(name)
	prefixRunes := []rune(prefix)
	if len(prefixRunes) > len(nameRunes) {
		return false
	}
	return strings.EqualFold(string(nameRunes[:len(prefixRunes)]), prefix)
}

func notFound(kind string) error {
	switch kind {
	case "project":
		return ErrProjectNotFound
	case "service":
		return ErrServiceNotFound
	case "field":
		return ErrFieldNotFound
	default:
		return fmt.Errorf("unknown name kind %q", kind)
	}
}

func mapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}
