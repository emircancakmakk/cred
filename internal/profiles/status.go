package profiles

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type GitStatus struct {
	Checkout         string
	Branch           string
	Commit           string
	Dirty            bool
	Unpublished      bool
	UnpublishedKnown bool
	LastFetch        time.Time
}

// InspectGit uses read-only Git commands and does not initialize a checkout.
func InspectGit(dir string) (GitStatus, error) {
	s := GitStatus{Checkout: "missing"}
	if _, e := os.Stat(dir); errors.Is(e, os.ErrNotExist) {
		return s, nil
	} else if e != nil {
		return s, e
	}
	root, e := git(dir, "rev-parse", "--show-toplevel")
	if e != nil {
		var ge *GitError
		if errors.As(e, &ge) && ge.Kind == GitUnavailable {
			return s, e
		}
		s.Checkout = "invalid"
		return s, nil
	}
	actual, e := filepath.EvalSymlinks(dir)
	if e != nil {
		return s, e
	}
	if root != actual {
		s.Checkout = "invalid"
		return s, nil
	}
	s.Checkout = "present"
	s.Branch, e = git(dir, "symbolic-ref", "--short", "HEAD")
	if e != nil {
		s.Checkout = "detached HEAD"
		s.Branch = ""
	}
	s.Commit, _ = git(dir, "rev-parse", "--verify", "HEAD")
	dirty, e := git(dir, "status", "--porcelain", "--untracked-files=no")
	if e != nil {
		return s, e
	}
	s.Dirty = dirty != ""
	if s.Commit != "" && s.Branch != "" {
		ref := "refs/remotes/origin/" + s.Branch
		if _, e = git(dir, "rev-parse", "--verify", ref); e == nil {
			count, e := git(dir, "rev-list", "--count", ref+"..HEAD")
			if e != nil {
				return s, e
			}
			s.Unpublished = count != "0"
			s.UnpublishedKnown = true
		}
	}
	if info, e := os.Stat(filepath.Join(dir, ".git", "FETCH_HEAD")); e == nil {
		s.LastFetch = info.ModTime()
	} else if !errors.Is(e, os.ErrNotExist) {
		return s, e
	}
	return s, nil
}

// RemoteHead checks reachability and the selected branch without fetching refs.
func RemoteHead(repo, branch string) (string, error) {
	if repo == "" || strings.HasPrefix(repo, "-") || strings.Contains(repo, "::") {
		return "", errors.New("invalid Git repository URL")
	}
	ref := "HEAD"
	if branch != "" {
		ref = "refs/heads/" + branch
	}
	out, e := git("", "ls-remote", "--", repo, ref)
	if e != nil {
		return "", e
	}
	for _, line := range strings.Split(out, "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == ref {
			return parts[0], nil
		}
	}
	return "", errors.New("remote branch is missing or the repository is empty; check the repository's default branch and vault setup")
}
