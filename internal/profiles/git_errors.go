package profiles

import (
	"context"
	"errors"
	"os/exec"
	"strings"
)

type GitErrorKind string

const (
	GitUnavailable GitErrorKind = "unavailable"
	GitTimeout     GitErrorKind = "timeout"
	GitNetwork     GitErrorKind = "network"
	GitAccess      GitErrorKind = "access"
	GitIdentity    GitErrorKind = "identity"
	GitConcurrent  GitErrorKind = "concurrent"
	GitPolicy      GitErrorKind = "policy"
	GitUnknown     GitErrorKind = "unknown"
)

// GitError intentionally contains neither subprocess output nor repository URLs.
type GitError struct {
	Operation string
	Kind      GitErrorKind
}

func (e *GitError) Error() string {
	hints := map[GitErrorKind]string{
		GitUnavailable: "Git is not installed or is not on PATH; install Git and try again",
		GitTimeout:     "the operation timed out; check connectivity and repository access, then try again",
		GitNetwork:     "could not reach the repository; check your network, VPN and DNS, then try again",
		GitAccess:      "authentication failed or the repository is unavailable to your account; check your SSH key or Git credentials and repository permissions",
		GitIdentity:    "Git author identity is missing; configure git config --global user.name and git config --global user.email",
		GitConcurrent:  "the remote branch changed; fetch the latest vault and merge before retrying",
		GitPolicy:      "the server rejected the update; check branch protection, server hooks and write permissions",
		GitUnknown:     "the cause could not be determined; check repository access, connectivity and Git configuration",
	}
	return "git " + e.Operation + " failed: " + hints[e.Kind]
}
func IsConcurrent(err error) bool {
	var e *GitError
	return errors.As(err, &e) && e.Kind == GitConcurrent
}
func classifyGit(operation string, err error, stderr string) *GitError {
	kind := GitUnknown
	message := strings.ToLower(stderr)
	contains := func(parts ...string) bool {
		for _, p := range parts {
			if strings.Contains(message, p) {
				return true
			}
		}
		return false
	}
	switch {
	case errors.Is(err, exec.ErrNotFound):
		kind = GitUnavailable
	case errors.Is(err, context.DeadlineExceeded):
		kind = GitTimeout
	case contains("please tell me who you are", "unable to auto-detect email address", "empty ident name", "author identity unknown", "committer identity unknown"):
		kind = GitIdentity
	case contains("could not resolve", "couldn't resolve", "failed to connect", "connection timed out", "connection refused", "network is unreachable"):
		kind = GitNetwork
	case contains("permission denied", "authentication failed", "repository not found", "could not read username", "access denied", "host key verification failed", "error: 403", "error: 401", "http 403", "http 401"):
		kind = GitAccess
	case operation == "push" && contains("[remote rejected]", "hook declined", "protected branch", "pre-receive hook", "remote: error: gh013"):
		kind = GitPolicy
	case operation == "push" && contains("[rejected]") && contains("non-fast-forward", "fetch first"):
		kind = GitConcurrent
	}
	return &GitError{Operation: operation, Kind: kind}
}
