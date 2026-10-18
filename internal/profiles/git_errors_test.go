package profiles

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestGitErrorsAreTypedAndSanitized(t *testing.T) {
	tests := []struct {
		name, op, stderr string
		err              error
		want             GitErrorKind
	}{
		{"missing Git", "fetch", "", exec.ErrNotFound, GitUnavailable},
		{"timeout", "fetch", "", context.DeadlineExceeded, GitTimeout},
		{"DNS", "clone", "Could not resolve host: private.example", errors.New("exit"), GitNetwork},
		{"SSH network", "fetch", "ssh: connect to host private.example port 22: Connection refused", errors.New("exit"), GitNetwork},
		{"SSH identity", "fetch", "Permission denied (publickey). fatal: Could not read from remote repository.", errors.New("exit"), GitAccess},
		{"HTTPS auth", "fetch", "Authentication failed for https://token@private.example", errors.New("exit"), GitAccess},
		{"identity", "commit", "Author identity unknown. Please tell me who you are.", errors.New("exit"), GitIdentity},
		{"stale remote", "push", "! [rejected] HEAD -> main (fetch first)", errors.New("exit"), GitConcurrent},
		{"non-fast-forward", "push", "! [rejected] HEAD -> main (non-fast-forward)", errors.New("exit"), GitConcurrent},
		{"policy", "push", "! [remote rejected] HEAD -> main (pre-receive hook declined)", errors.New("exit"), GitPolicy},
		{"ambiguous access", "fetch", "fatal: Could not read from remote repository.", errors.New("exit"), GitUnknown},
		{"unknown", "commit", "unexpected output", errors.New("exit"), GitUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diagnostic := tt.stderr + "\nTOKEN-DO-NOT-PRINT VAULT-SECRET https://user:password@internal.example/repo"
			e := classifyGit(tt.op, tt.err, diagnostic)
			if e.Kind != tt.want {
				t.Fatalf("kind %s want %s", e.Kind, tt.want)
			}
			for _, secret := range []string{"TOKEN-DO-NOT-PRINT", "VAULT-SECRET", "internal.example", "private.example", "user:password"} {
				if strings.Contains(e.Error(), secret) {
					t.Fatalf("diagnostic leaked %s", secret)
				}
			}
			if !strings.Contains(e.Error(), "git "+tt.op+" failed:") {
				t.Fatal(e)
			}
			if IsConcurrent(fmt.Errorf("wrapped: %w", e)) != (tt.want == GitConcurrent) {
				t.Fatal("wrapped concurrency classification")
			}
		})
	}
}
func TestGitMissingExecutable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := git("", "version")
	var ge *GitError
	if !errors.As(err, &ge) || ge.Kind != GitUnavailable {
		t.Fatalf("%v", err)
	}
}
func TestReadLockDoesNotCreate(t *testing.T) {
	path := t.TempDir() + "/missing/vault.lock"
	release, busy, e := ReadLock(path)
	if e != nil || busy {
		t.Fatalf("%v %v", busy, e)
	}
	release()
	if _, e = os.Stat(path); !os.IsNotExist(e) {
		t.Fatal("read lock created a file")
	}
	existing := t.TempDir() + "/vault.lock"
	writer, e := Lock(existing)
	if e != nil {
		t.Fatal(e)
	}
	release, busy, e = ReadLock(existing)
	if e != nil || !busy {
		writer()
		t.Fatalf("writer not detected: %v %v", busy, e)
	}
	release()
	writer()
	release, busy, e = ReadLock(existing)
	if e != nil || busy {
		t.Fatalf("shared read lock: %v %v", busy, e)
	}
	release()
}
