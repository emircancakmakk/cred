package vault

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emircancakmakk/cred/internal/crypto"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.age")
	password := []byte("x")
	if err := ValidateMasterPassword(password); err == nil {
		t.Fatal("short password should fail validation")
	}

	v := New()
	if err := v.Add("filiz", "vercel", "email", Field{Value: "a@b.c", Secret: false}); err != nil {
		t.Fatal(err)
	}
	if err := v.Add("filiz", "vercel", "password", Field{Value: "round-trip-secret", Secret: true}); err != nil {
		t.Fatal(err)
	}
	if err := v.Add("filiz", "digital-ocean", "token", Field{Value: "", Secret: true}); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, password, v, fastOptions()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".bak"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("first save created a backup")
	}

	got, err := Load(path, password)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot(t, got) != snapshot(t, v) {
		t.Fatalf("loaded vault mismatch:\n%s", snapshot(t, got))
	}
}

func TestSaveLoadEmptyVault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.age")
	password := []byte("test-password")
	if err := Save(path, password, New(), fastOptions()); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path, password)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 1 || len(got.Projects) != 0 {
		t.Fatalf("got version %d projects %d", got.Version, len(got.Projects))
	}
}

func TestLoadWrongPasswordAndCorrupt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.age")
	password := []byte("super-secret-password-value")
	v := New()
	if err := v.Add("filiz", "vercel", "password", Field{Value: "plaintext-secret-value", Secret: true}); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, password, v, fastOptions()); err != nil {
		t.Fatal(err)
	}
	sealed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Load(path, []byte("other-password-value"))
	if !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("wrong password: %v", err)
	}
	if errors.Is(err, ErrCorruptVault) {
		t.Fatal("wrong password reported as corrupt")
	}
	assertNoSecret(t, err, "super-secret-password-value", "plaintext-secret-value")

	corruptPath := filepath.Join(dir, "broken.age")
	if err := os.WriteFile(corruptPath, []byte("not a vault"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = Load(corruptPath, password)
	if !errors.Is(err, ErrCorruptVault) {
		t.Fatalf("garbage: %v", err)
	}
	if errors.Is(err, ErrWrongPassword) {
		t.Fatal("garbage reported as wrong password")
	}

	truncated := filepath.Join(dir, "truncated.age")
	if err := os.WriteFile(truncated, sealed[:len(sealed)/2], 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = Load(truncated, password)
	if !errors.Is(err, ErrCorruptVault) {
		t.Fatalf("truncated: %v", err)
	}
	assertNoSecret(t, err, "super-secret-password-value", "plaintext-secret-value")
}

func TestLoadVersionBeforeUnknownFields(t *testing.T) {
	password := []byte("test-password")
	const marker = "DISTINCTIVE-SECRET-VALUE"
	tests := []struct {
		name    string
		body    string
		want    error
		notWant error
	}{
		{
			name:    "unsupported version with unknown field",
			body:    `{"version":2,"extra":"DISTINCTIVE-SECRET-VALUE","projects":{}}`,
			want:    ErrUnsupportedVersion,
			notWant: ErrCorruptVault,
		},
		{
			name:    "missing version with unknown field",
			body:    `{"projects":{},"note":"DISTINCTIVE-SECRET-VALUE"}`,
			want:    ErrUnsupportedVersion,
			notWant: ErrCorruptVault,
		},
		{
			name:    "version 1 unknown field",
			body:    `{"version":1,"projects":{},"extra":"DISTINCTIVE-SECRET-VALUE"}`,
			want:    ErrCorruptVault,
			notWant: ErrUnsupportedVersion,
		},
		{
			name: "version 1 nested unknown field",
			body: `{"version":1,"projects":{"filiz":{"services":{"vercel":{"fields":{"email":{"value":"a@b.c","secret":false,"extra":"DISTINCTIVE-SECRET-VALUE"}}}}}}}`,
			want: ErrCorruptVault,
		},
		{
			name: "version 1 document",
			body: `{"version":1,"projects":{"filiz":{"services":{"vercel":{"fields":{"email":{"value":"a@b.c","secret":false}}}}}}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "vault.age")
			writeEncrypted(t, path, []byte(tt.body), password)
			got, err := Load(path, password)
			if tt.want != nil {
				if !errors.Is(err, tt.want) {
					t.Fatalf("got %v, want %v", err, tt.want)
				}
				if tt.notWant != nil && errors.Is(err, tt.notWant) {
					t.Fatalf("also matched %v", tt.notWant)
				}
				assertNoSecret(t, err, marker, "filiz")
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			field, err := got.GetField("filiz", "vercel", "email")
			if err != nil {
				t.Fatal(err)
			}
			if field.Value != "a@b.c" || field.Secret {
				t.Fatalf("field = %+v", field)
			}
		})
	}
}

func TestBackupRestoresPrevious(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.age")
	password := []byte("test-password")
	v := New()
	if err := v.Add("filiz", "vercel", "password", Field{Value: "first-value", Secret: true}); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, password, v, fastOptions()); err != nil {
		t.Fatal(err)
	}
	if err := v.Set("filiz", "vercel", "password", Field{Value: "second-value", Secret: false}); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, password, v, fastOptions()); err != nil {
		t.Fatal(err)
	}

	current, err := Load(path, password)
	if err != nil {
		t.Fatal(err)
	}
	got, err := current.GetField("filiz", "vercel", "password")
	if err != nil {
		t.Fatal(err)
	}
	if got != (Field{Value: "second-value", Secret: false}) {
		t.Fatalf("current = %+v", got)
	}

	previous, err := Load(path+".bak", password)
	if err != nil {
		t.Fatal(err)
	}
	got, err = previous.GetField("filiz", "vercel", "password")
	if err != nil {
		t.Fatal(err)
	}
	if got != (Field{Value: "first-value", Secret: true}) {
		t.Fatalf("backup = %+v", got)
	}
}

func TestSaveReadOnlyDirectoryLeavesOriginal(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := t.TempDir()
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	path := filepath.Join(dir, "vault.age")
	password := []byte("test-password")
	v := New()
	if err := v.Add("filiz", "vercel", "password", Field{Value: "original-secret", Secret: true}); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, password, v, fastOptions()); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	if err := v.Set("filiz", "vercel", "password", Field{Value: "changed-secret", Secret: true}); err != nil {
		t.Fatal(err)
	}
	err := Save(path, password, v, fastOptions())
	if err == nil {
		t.Fatal("expected save to fail")
	}
	assertNoSecret(t, err, "original-secret", "changed-secret")
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path, password)
	if err != nil {
		t.Fatal(err)
	}
	field, err := got.GetField("filiz", "vercel", "password")
	if err != nil {
		t.Fatal(err)
	}
	if field.Value != "original-secret" {
		t.Fatalf("value = %q", field.Value)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp") {
			t.Fatalf("temporary file left behind: %s", entry.Name())
		}
	}
}

func TestFilePermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cred")
	path := filepath.Join(dir, "vault.age")
	password := []byte("test-password")
	v := New()
	if err := v.Add("filiz", "vercel", "password", Field{Value: "secret", Secret: true}); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, password, v, fastOptions()); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, password, v, fastOptions()); err != nil {
		t.Fatal(err)
	}
	assertMode(t, dir, 0o700)
	assertMode(t, path, 0o600)
	assertMode(t, path+".bak", 0o600)
}

func TestLoadMissing(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing.age"), []byte("test-password"))
	if !errors.Is(err, ErrVaultNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestSaveEmptyPasswordDoesNotCreateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.age")
	err := Save(path, nil, New(), fastOptions())
	if !errors.Is(err, ErrEmptyPassword) {
		t.Fatalf("got %v", err)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("empty password created a file")
	}
}

func TestPathFromEnv(t *testing.T) {
	want := filepath.Join(t.TempDir(), "custom.age")
	t.Setenv("CRED_VAULT", want)
	got, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %s", got)
	}
	if _, err := os.Stat(want); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("Path created a file")
	}
}

func TestDefaultPathDoesNotCreate(t *testing.T) {
	t.Setenv("CRED_VAULT", "")
	got, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	config, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(config, "cred", "vault.age")
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func fastOptions() Options {
	return Options{WorkFactor: crypto.MinScryptLogN}
}

func writeEncrypted(t *testing.T, path string, plain, password []byte) {
	t.Helper()
	sealed, err := crypto.Encrypt(plain, password, crypto.MinScryptLogN)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, sealed, 0o600); err != nil {
		t.Fatal(err)
	}
}

func snapshot(t *testing.T, v *Vault) string {
	t.Helper()
	buf, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(buf)
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != want {
		t.Fatalf("%s mode = %o, want %o", path, info.Mode().Perm(), want)
	}
}

func assertNoSecret(t *testing.T, err error, secrets ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	msg := err.Error()
	for _, secret := range secrets {
		if secret != "" && strings.Contains(msg, secret) {
			t.Fatal("error contains secret material")
		}
	}
}

func TestMarshalUsesIndent(t *testing.T) {
	v := New()
	buf, err := marshalVault(v)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf, []byte("\n")) {
		t.Fatal("expected indented JSON")
	}
}
