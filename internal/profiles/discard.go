package profiles

import (
	"errors"
	"fmt"
	"strings"

	"github.com/emircancakmakk/cred/internal/vault"
)

type RemoteSnapshot struct {
	Empty      bool
	Commit     string
	Ciphertext []byte
}

// FetchSnapshot never resets the working copy. The returned ciphertext belongs
// to the pinned fetched commit and must be validated before ResetSnapshot.
func (g *Git) FetchSnapshot() (RemoteSnapshot, error) {
	var snapshot RemoteSnapshot
	refs, e := g.run("ls-remote", "origin")
	if e != nil {
		return snapshot, e
	}
	if refs == "" {
		snapshot.Empty = true
		return snapshot, nil
	}
	branchFound := false
	for _, line := range strings.Split(refs, "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == "refs/heads/"+g.Branch {
			branchFound = true
		}
	}
	if !branchFound {
		return snapshot, errors.New("remote branch is missing; pending changes were preserved")
	}
	ref := "refs/remotes/origin/" + g.Branch
	if _, e = g.run("fetch", "--quiet", "--no-tags", "origin", "+refs/heads/"+g.Branch+":"+ref); e != nil {
		return snapshot, e
	}
	snapshot.Commit, e = g.run("rev-parse", "--verify", ref+"^{commit}")
	if e != nil {
		return snapshot, e
	}
	entry, e := g.run("ls-tree", snapshot.Commit, "--", "vault.age")
	if e != nil {
		return snapshot, e
	}
	fields := strings.Fields(entry)
	if len(fields) == 0 {
		return snapshot, fmt.Errorf("remote snapshot: %w", vault.ErrVaultNotFound)
	}
	if len(fields) != 4 || (fields[0] != "100644" && fields[0] != "100755") || fields[1] != "blob" || fields[3] != "vault.age" {
		return snapshot, errors.New("remote vault.age must be a regular file; pending changes were preserved")
	}
	snapshot.Ciphertext, e = gitBytes(g.Dir, "show", snapshot.Commit+":vault.age")
	return snapshot, e
}
func (g *Git) ResetSnapshot(snapshot RemoteSnapshot) error {
	if snapshot.Empty || snapshot.Commit == "" {
		return errors.New("cannot restore an empty remote snapshot")
	}
	stop := observe(g.Observe, "Restoring remote vault")
	defer stop()
	if _, e := g.run("reset", "--hard", snapshot.Commit); e != nil {
		return e
	}
	g.Base = snapshot.Commit
	return g.checkPath()
}
func (g *Git) BackupTracked() (bool, error) {
	out, e := g.run("ls-files", "--", "vault.age.bak")
	return out != "", e
}
