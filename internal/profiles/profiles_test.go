package profiles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSelectionAndPrivateConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("CRED_VAULT", "")
	personal, e := NamedPath("personal")
	if e != nil {
		t.Fatal(e)
	}
	c := Config{Default: "team", Vaults: map[string]Profile{"personal": {Type: "local", Path: personal}, "team": {Type: "git", Path: filepath.Join(t.TempDir(), "vault.age"), Repo: "git@example.invalid:team/vault.git"}}}
	if e = Save(c); e != nil {
		t.Fatal(e)
	}
	p, name, e := Select("")
	if e != nil || name != "team" || p.Type != "git" {
		t.Fatalf("default %v %s %v", p, name, e)
	}
	t.Setenv("CRED_VAULT", filepath.Join(t.TempDir(), "override.age"))
	p, _, e = Select("")
	if e != nil || p.Type != "local" || p.Path != os.Getenv("CRED_VAULT") {
		t.Fatalf("override %v %v", p, e)
	}
	p, _, e = Select("personal")
	if e != nil || p.Path != personal {
		t.Fatalf("explicit %v %v", p, e)
	}
	p, _, e = Select("local")
	if e != nil || p.Path == os.Getenv("CRED_VAULT") {
		t.Fatalf("default local %v %v", p, e)
	}
	if _, _, e = Select("missing"); e == nil {
		t.Fatal("missing accepted")
	}
	if _, e = NamedPath("../escape"); e == nil {
		t.Fatal("unsafe profile accepted")
	}
	d, _ := Dir()
	info, e := os.Stat(filepath.Join(d, "profiles.json"))
	if e != nil {
		t.Fatal(e)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("config permissions %o", info.Mode().Perm())
	}
	lock := filepath.Join(t.TempDir(), "vault.lock")
	release, e := Lock(lock)
	if e != nil {
		t.Fatal(e)
	}
	if second, e := Lock(lock); e == nil {
		second()
		t.Fatal("concurrent lock allowed")
	}
	release()
	release, e = Lock(lock)
	if e != nil {
		t.Fatal(e)
	}
	release()
}

func TestGitRejectsSymlink(t *testing.T) {
	g := &Git{Dir: t.TempDir()}
	target := filepath.Join(t.TempDir(), "target")
	if e := os.WriteFile(target, []byte("unchanged"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(target, g.Path()); e != nil {
		t.Fatal(e)
	}
	if e := g.checkPath(); e == nil {
		t.Fatal("symlink accepted")
	}
}
