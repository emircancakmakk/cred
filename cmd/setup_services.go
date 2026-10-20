package cmd

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/emircancakmakk/cred/internal/crypto"
	"github.com/emircancakmakk/cred/internal/profiles"
	"github.com/emircancakmakk/cred/internal/vault"
)

type setupSession struct {
	name                        string
	profile                     profiles.Profile
	config                      profiles.Config
	configUnlock, profileUnlock func()
}

func (s *setupSession) unlockConfig() {
	if s.configUnlock != nil {
		s.configUnlock()
		s.configUnlock = nil
	}
}
func (s *setupSession) close() {
	if s.profileUnlock != nil {
		s.profileUnlock()
	}
	s.unlockConfig()
}
func (s *setupSession) register() error {
	s.config.Vaults[s.name] = s.profile
	return profiles.Save(s.config)
}
func beginSetup(name, kind, repo string) (*setupSession, error) {
	if e := profiles.ValidateName(name); e != nil {
		return nil, e
	}
	d, e := profiles.Dir()
	if e != nil {
		return nil, e
	}
	unlock, e := profiles.Lock(filepath.Join(d, "profiles.lock"))
	if e != nil {
		return nil, e
	}
	s := &setupSession{name: name, configUnlock: unlock}
	ok := false
	defer func() {
		if !ok {
			s.close()
		}
	}()
	s.config, e = profiles.Load()
	if e != nil {
		return nil, e
	}
	if _, exists := s.config.Vaults[name]; exists {
		return nil, errors.New("profile already exists; use cred vault list to inspect it")
	}
	path, e := profiles.NamedPath(name)
	if e != nil {
		return nil, e
	}
	s.profile = profiles.Profile{Type: kind, Path: path, Repo: repo}
	s.profileUnlock, e = profiles.Lock(path + ".lock")
	if e != nil {
		return nil, e
	}
	for _, file := range []string{path, path + ".pending", path + ".pending.base"} {
		if _, e = os.Lstat(file); e == nil {
			return nil, fmt.Errorf("vault data already exists at %s; choose another profile name", file)
		} else if !errors.Is(e, os.ErrNotExist) {
			return nil, e
		}
	}
	ok = true
	return s, nil
}
func (a *App) newPassword() ([]byte, error) {
	p, e := a.prompter()
	if e != nil {
		return nil, e
	}
	first, e := p.ReadPassword(promptPassword)
	if e != nil {
		return nil, e
	}
	valid := false
	defer func() {
		if !valid {
			crypto.Wipe(first)
		}
	}()
	if e = vault.ValidateMasterPassword(first); e != nil {
		return nil, e
	}
	second, e := p.ReadPassword(promptRepeat)
	if e != nil {
		return nil, e
	}
	defer crypto.Wipe(second)
	if subtle.ConstantTimeCompare(first, second) != 1 {
		return nil, errors.New("passwords do not match")
	}
	valid = true
	return first, nil
}
func (a *App) createLocal(name string) (profiles.Profile, error) {
	s, e := beginSetup(name, "local", "")
	if e != nil {
		return profiles.Profile{}, e
	}
	defer s.close()
	password, e := a.newPassword()
	if e != nil {
		return s.profile, e
	}
	defer crypto.Wipe(password)
	stop := a.stage("Encrypting vault")
	e = vault.Save(s.profile.Path, password, vault.New(), vault.Options{WorkFactor: a.WorkFactor})
	stop()
	if e != nil {
		return s.profile, e
	}
	return s.profile, s.register()
}
func (a *App) connectProfile(name, repo, from string) (profiles.Profile, error) {
	s, e := beginSetup(name, "git", repo)
	if e != nil {
		return profiles.Profile{}, e
	}
	defer s.close()
	g, e := profiles.OpenGit(repo, s.profile.RepoDir(), a.stage)
	if e != nil {
		return s.profile, e
	}
	if from != "" {
		if e = requireEmpty(g); e != nil {
			return s.profile, e
		}
	} else if g.Base != "" {
		if e = g.Update(false); e != nil {
			return s.profile, e
		}
	}
	p, e := a.prompter()
	if e != nil {
		return s.profile, e
	}

	if from != "" {
		source, _, e := profiles.Select(from)
		if e != nil {
			return s.profile, e
		}
		if source.Type != "local" {
			return s.profile, errors.New("--from requires a local vault")
		}
		release, e := profiles.Lock(source.Path + ".lock")
		if e != nil {
			return s.profile, e
		}
		defer release()
		initial, password, e := a.readUnlockedVault(p, source.Path)
		if e != nil {
			return s.profile, e
		}
		defer crypto.Wipe(password)
		return s.profile, a.startShared(s, g, password, initial)
	}
	_, password, e := a.readUnlockedVault(p, g.Path())
	if e != nil {
		return s.profile, e
	}
	defer crypto.Wipe(password)
	return s.profile, s.register()
}
func requireEmpty(g *profiles.Git) error {
	refs, e := g.HasRemoteRefs()
	if e != nil {
		return e
	}
	if refs || g.Base != "" {
		return errors.New("a new shared vault requires an empty repository with no branches or tags; connect to an existing vault instead")
	}
	return nil
}
func (a *App) createShared(name, repo string) (profiles.Profile, error) {
	s, e := beginSetup(name, "git", repo)
	if e != nil {
		return profiles.Profile{}, e
	}
	defer s.close()
	g, e := profiles.OpenGit(repo, s.profile.RepoDir(), a.stage)
	if e != nil {
		return s.profile, e
	}
	if e = requireEmpty(g); e != nil {
		return s.profile, e
	}
	password, e := a.newPassword()
	if e != nil {
		return s.profile, e
	}
	defer crypto.Wipe(password)
	return s.profile, a.startShared(s, g, password, vault.New())
}
func (a *App) startShared(s *setupSession, g *profiles.Git, password []byte, initial *vault.Vault) error {
	s.profile.Initializing = true
	a.profile = s.profile
	a.vaultName = s.name
	a.gitSession = g
	a.base = vault.New()
	// Both encrypted snapshots are durable before the profile advertises recovery.
	if e := a.retainChange(s.profile.Path+".pending", password, initial); e != nil {
		return e
	}
	if e := s.register(); e != nil {
		return e
	}
	s.unlockConfig()
	return a.saveGit(password, initial)
}
func (a *App) completeInitialization() error {
	if !a.profile.Initializing {
		return nil
	}
	d, e := profiles.Dir()
	if e != nil {
		return e
	}
	release, e := profiles.Lock(filepath.Join(d, "profiles.lock"))
	if e != nil {
		return e
	}
	defer release()
	c, e := profiles.Load()
	if e != nil {
		return e
	}
	name := a.activeName()
	p, ok := c.Vaults[name]
	if !ok {
		return errors.New("initializing profile is missing from profiles.json")
	}
	p.Initializing = false
	c.Vaults[name] = p
	if e = profiles.Save(c); e != nil {
		return e
	}
	a.profile = p
	return nil
}
func setDefaultProfile(name string) error {
	d, e := profiles.Dir()
	if e != nil {
		return e
	}
	release, e := profiles.Lock(filepath.Join(d, "profiles.lock"))
	if e != nil {
		return e
	}
	defer release()
	c, e := profiles.Load()
	if e != nil {
		return e
	}
	p, ok := c.Vaults[name]
	if !ok {
		return errors.New("unknown vault profile")
	}
	if p.Initializing {
		return fmt.Errorf("shared vault setup is incomplete; run cred vault retry %s to complete it or cred vault discard %s to cancel it", name, name)
	}
	c.Default = name
	return profiles.Save(c)
}
