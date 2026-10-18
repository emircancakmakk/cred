// Package profiles stores machine-local vault connections, never passwords.
package profiles

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/emircancakmakk/cred/internal/vault"
	"golang.org/x/sys/unix"
)

type Profile struct {
	Type         string `json:"type"`
	Path         string `json:"path"`
	Repo         string `json:"repo,omitempty"`
	Initializing bool   `json:"initializing,omitempty"`
}
type Config struct {
	Default string             `json:"default_vault"`
	Vaults  map[string]Profile `json:"vaults"`
}

func Dir() (string, error) { d, e := os.UserConfigDir(); return filepath.Join(d, "cred"), e }
func Load() (Config, error) {
	c := Config{Vaults: map[string]Profile{}}
	d, e := Dir()
	if e != nil {
		return c, e
	}
	b, e := os.ReadFile(filepath.Join(d, "profiles.json"))
	if errors.Is(e, os.ErrNotExist) {
		return c, nil
	}
	if e != nil {
		return c, e
	}
	e = json.Unmarshal(b, &c)
	if e != nil {
		return c, errors.New("profile configuration is invalid JSON; fix profiles.json in the cred config directory")
	}
	if c.Vaults == nil {
		c.Vaults = map[string]Profile{}
	}
	return c, e
}
func Save(c Config) error {
	d, e := Dir()
	if e != nil {
		return e
	}
	if e = os.MkdirAll(d, 0700); e != nil {
		return e
	}
	b, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(d, ".profiles-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(f.Name(), filepath.Join(d, "profiles.json"))
}
func ValidateName(name string) error {
	if !regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`).MatchString(name) {
		return errors.New("profile name must contain only letters, numbers, hyphens and underscores")
	}
	return nil
}
func NamedPath(name string) (string, error) {
	if e := ValidateName(name); e != nil {
		return "", e
	}
	d, e := Dir()
	return filepath.Join(d, "vaults", name, "vault.age"), e
}
func Select(name string) (Profile, string, error) {
	if name == "" && os.Getenv("CRED_VAULT") != "" {
		p, e := vault.Path()
		return Profile{Type: "local", Path: p}, "local", e
	}
	c, e := Load()
	if e != nil {
		return Profile{}, "", e
	}
	if name == "" {
		name = c.Default
	}
	if name == "" {
		name = "local"
	}
	p, ok := c.Vaults[name]
	if !ok && name == "local" {
		path, e := NamedPath(name)
		return Profile{Type: "local", Path: path}, name, e
	}
	if !ok {
		return p, name, fmt.Errorf("unknown vault profile %q; run cred vault list to see available profiles", name)
	}
	if p.Type != "local" && p.Type != "git" {
		return p, name, errors.New("invalid vault profile type in profiles.json; expected local or git")
	}
	if p.Path == "" {
		return p, name, errors.New("vault profile path is missing in profiles.json; fix the profile configuration")
	}
	if p.Type == "git" && p.Repo == "" {
		return p, name, errors.New("Git repository is missing in profiles.json; fix the profile configuration")
	}
	return p, name, nil
}

// Lock is held for the entire read/modify/write operation, including prompts.
func Lock(path string) (func(), error) {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return nil, e
	}
	f, e := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); e != nil {
		f.Close()
		return nil, errors.New("vault is busy in another cred process")
	}
	return func() { _ = unix.Flock(int(f.Fd()), unix.LOCK_UN); _ = f.Close() }, nil
}

// RepoDir is the managed persistent checkout, separate from recovery files.
func (p Profile) RepoDir() string { return filepath.Join(filepath.Dir(p.Path), "repo") }

// ReadLock never creates files. An absent lock indicates no managed operation
// has started; callers treat this as a best-effort status snapshot.
func ReadLock(path string) (release func(), busy bool, err error) {
	f, e := os.Open(path)
	if errors.Is(e, os.ErrNotExist) {
		return func() {}, false, nil
	}
	if e != nil {
		return nil, false, e
	}
	if e = unix.Flock(int(f.Fd()), unix.LOCK_SH|unix.LOCK_NB); e != nil {
		f.Close()
		if errors.Is(e, unix.EWOULDBLOCK) || errors.Is(e, unix.EAGAIN) {
			return func() {}, true, nil
		}
		return nil, false, e
	}
	return func() { _ = unix.Flock(int(f.Fd()), unix.LOCK_UN); _ = f.Close() }, false, nil
}
