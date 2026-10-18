package profiles

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/emircancakmakk/cred/internal/vault"
)

// Observer starts a user-facing phase and returns its cleanup function.
type Observer func(string) func()
type Git struct {
	Dir, Branch, Base string
	Observe           Observer
}

func observe(observer Observer, phase string) func() {
	if observer != nil {
		return observer(phase)
	}
	return func() {}
}
func (g *Git) run(args ...string) (string, error) {
	phase := map[string]string{"fetch": "Updating vault", "push": "Sharing changes", "commit": "Committing encrypted vault", "ls-remote": "Checking repository"}[args[0]]
	if phase != "" {
		stop := observe(g.Observe, phase)
		defer stop()
	}
	return git(g.Dir, args...)
}

func git(dir string, args ...string) (string, error) {
	data, e := gitBytes(dir, args...)
	if e != nil {
		return "", e
	}
	return strings.TrimSpace(string(data)), nil
}

// gitBytes preserves ciphertext exactly; text callers explicitly trim their output.
func gitBytes(dir string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	prefix := []string{"-c", "core.hooksPath=/dev/null", "-c", "commit.gpgsign=false"}
	if dir != "" {
		prefix = append(prefix, "-C", dir)
	}
	cmd := exec.CommandContext(ctx, "git", append(prefix, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "LC_ALL=C")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	b, e := cmd.Output()
	if e != nil {
		if ctx.Err() != nil {
			e = ctx.Err()
		}
		return nil, classifyGit(args[0], e, stderr.String())
	}
	return b, nil
}

// OpenGit clones once into a dedicated managed directory, then reuses it.
func OpenGit(repo, dir string, observers ...Observer) (*Git, error) {
	var observer Observer
	if len(observers) > 0 {
		observer = observers[0]
	}
	if repo == "" || strings.HasPrefix(repo, "-") || strings.Contains(repo, "::") {
		return nil, errors.New("invalid Git repository URL")
	}
	if _, e := os.Stat(dir); errors.Is(e, os.ErrNotExist) {
		parent := filepath.Dir(dir)
		if e = os.MkdirAll(parent, 0700); e != nil {
			return nil, e
		}
		temp, e := os.MkdirTemp(parent, ".clone-*")
		if e != nil {
			return nil, e
		}
		defer os.RemoveAll(temp)
		stop := observe(observer, "Cloning vault")
		_, e = git("", "clone", "--quiet", "--no-local", "--", repo, temp)
		stop()
		if e != nil {
			return nil, e
		}
		if e = os.Rename(temp, dir); e != nil {
			return nil, e
		}
	} else if e != nil {
		return nil, e
	}
	root, e := git(dir, "rev-parse", "--show-toplevel")
	if e != nil {
		return nil, e
	}
	actual, e := filepath.EvalSymlinks(dir)
	if e != nil {
		return nil, e
	}
	if root != actual {
		return nil, errors.New("managed Git directory is not a repository root")
	}
	origin, e := git(dir, "remote", "get-url", "origin")
	if e != nil {
		return nil, e
	}
	if origin != repo {
		return nil, errors.New("managed Git origin differs from profile repository")
	}
	g := &Git{Dir: dir, Observe: observer}
	g.Branch, e = git(dir, "symbolic-ref", "--short", "HEAD")
	if e != nil {
		return nil, e
	}
	if e = g.checkPath(); e != nil {
		return nil, e
	}
	g.Base, _ = git(dir, "rev-parse", "HEAD")
	return g, nil
}

// Update fetches explicitly; Git never merges the encrypted file. Recovery may
// reset our own incomplete write only after its encrypted recovery pair is read.
func (g *Git) Update(recovery bool) error {
	if !recovery {
		dirty, e := g.run("status", "--porcelain", "--untracked-files=no")
		if e != nil {
			return e
		}
		if dirty != "" {
			return errors.New("managed Git checkout has uncommitted changes; use vault retry for a retained change")
		}
	}
	if _, e := g.run("fetch", "--quiet", "origin"); e != nil {
		return e
	}
	ref := "refs/remotes/origin/" + g.Branch
	if !recovery && g.Base != "" {
		if _, e := g.run("merge-base", "--is-ancestor", "HEAD", ref); e != nil {
			return errors.New("managed Git checkout has unpublished commits; recover pending changes with vault retry")
		}
	}
	if _, e := g.run("reset", "--hard", ref); e != nil {
		return e
	}
	var e error
	g.Base, e = g.run("rev-parse", "HEAD")
	if e != nil {
		return e
	}
	return g.checkPath()
}
func (g *Git) Path() string { return filepath.Join(g.Dir, "vault.age") }
func (g *Git) Refresh(password []byte) (*vault.Vault, error) {
	if e := g.Update(true); e != nil {
		return nil, e
	}
	stop := observe(g.Observe, "Unlocking updated vault")
	defer stop()
	return vault.Load(g.Path(), password)
}
func (g *Git) Publish(password []byte, v *vault.Vault, opt vault.Options) error {
	if e := g.checkPath(); e != nil {
		return e
	}
	stop := observe(g.Observe, "Encrypting vault")
	e := vault.Save(g.Path(), password, v, opt)
	stop()
	if e != nil {
		return e
	}
	if _, e := g.run("add", "--", "vault.age"); e != nil {
		return e
	}
	if _, e := g.run("commit", "--quiet", "-m", "Update encrypted vault"); e != nil {
		return e
	}
	_, e = g.run("push", "--quiet", "origin", "HEAD:refs/heads/"+g.Branch)
	return e
}

func (g *Git) checkPath() error {
	for _, path := range []string{g.Path(), g.Path() + ".bak"} {
		info, e := os.Lstat(path)
		if errors.Is(e, os.ErrNotExist) {
			continue
		}
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() {
			return errors.New("Git vault and backup must be regular files")
		}
		if e = os.Chmod(path, 0600); e != nil {
			return e
		}
	}
	return nil
}

// HasRemoteRefs distinguishes a truly empty repository from one whose HEAD
// points to a missing branch but still contains other branches or tags.
func (g *Git) HasRemoteRefs() (bool, error) {
	out, e := g.run("ls-remote", "origin")
	return out != "", e
}
