package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emircancakmakk/cred/internal/vault"
)

func TestImportAddsFields(t *testing.T) {
	path := setVaultEnv(t)
	initVault(t)
	body := importBody(t, map[string]vault.Field{
		"filiz/vercel/email":    {Value: emailValue, Secret: false},
		"other/github/password": {Value: secretValue, Secret: true},
	})
	file := writeImport(t, body)

	stdout, stderr, err := runCmd(t, []string{"import", file}, []string{masterPassword}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "" {
		t.Fatalf("stdout %q", stdout)
	}
	if stderr != "added 2 fields\n" {
		t.Fatalf("stderr %q", stderr)
	}
	assertNoLeak(t, stdout+stderr, emailValue, secretValue, masterPassword)
	assertFileHides(t, path, emailValue, secretValue, masterPassword)
	if string(readFile(t, file)) != body {
		t.Fatal("import file changed")
	}

	email, err := loadVault(t, path, masterPassword).GetField("filiz", "vercel", "email")
	if err != nil {
		t.Fatal(err)
	}
	if email != (vault.Field{Value: emailValue, Secret: false}) {
		t.Fatalf("email = %+v", email)
	}
	password, err := loadVault(t, path, masterPassword).GetField("other", "github", "password")
	if err != nil {
		t.Fatal(err)
	}
	if password != (vault.Field{Value: secretValue, Secret: true}) {
		t.Fatalf("password = %+v", password)
	}

	listed, err := runOK(t, []string{"ls"}, []string{masterPassword}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if listed != "filiz\n  vercel\nother\n  github\n" {
		t.Fatalf("ls %q", listed)
	}
}

func TestImportSkipsExistingAndSavesNew(t *testing.T) {
	path := setVaultEnv(t)
	initVault(t)
	addPassword(t, path)
	if _, err := runOK(t, []string{"add", "filiz", "vercel"}, []string{masterPassword, emailValue}, []string{"email", "n", ""}); err != nil {
		t.Fatal(err)
	}
	const imported = "IMPORTED-TOKEN-VALUE"
	const overwriteSecret = "OVERWRITE-SECRET-VALUE"
	const overwriteEmail = "overwrite-email@example.com"
	body := importBody(t, map[string]vault.Field{
		"filiz/vercel/password":     {Value: overwriteSecret, Secret: false},
		"filiz/vercel/email":        {Value: overwriteEmail, Secret: true},
		"filiz/digital-ocean/token": {Value: imported, Secret: true},
	})
	file := writeImport(t, body)

	stdout, stderr, err := runCmd(t, []string{"import", file}, []string{masterPassword}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "" {
		t.Fatalf("stdout %q", stdout)
	}
	want := "added 1 field\nskipped 2 fields\nfiliz/vercel/email\nfiliz/vercel/password\n"
	if stderr != want {
		t.Fatalf("stderr %q", stderr)
	}
	assertNoLeak(t, stdout+stderr, secretValue, emailValue, imported, overwriteSecret, overwriteEmail, masterPassword)
	assertFileHides(t, path, secretValue, emailValue, imported, overwriteSecret, overwriteEmail, masterPassword)

	got, err := loadVault(t, path, masterPassword).GetField("filiz", "vercel", "password")
	if err != nil {
		t.Fatal(err)
	}
	if got != (vault.Field{Value: secretValue, Secret: true}) {
		t.Fatalf("password = %+v", got)
	}
	email, err := loadVault(t, path, masterPassword).GetField("filiz", "vercel", "email")
	if err != nil {
		t.Fatal(err)
	}
	if email != (vault.Field{Value: emailValue, Secret: false}) {
		t.Fatalf("email = %+v", email)
	}
	token, err := loadVault(t, path, masterPassword).GetField("filiz", "digital-ocean", "token")
	if err != nil {
		t.Fatal(err)
	}
	if token != (vault.Field{Value: imported, Secret: true}) {
		t.Fatalf("token = %+v", token)
	}
}

func TestImportAllSkippedLeavesFile(t *testing.T) {
	path := setVaultEnv(t)
	initVault(t)
	addPassword(t, path)
	before := readFile(t, path)
	body := importBody(t, map[string]vault.Field{
		"filiz/vercel/password": {Value: "OVERWRITE-SECRET-VALUE", Secret: false},
	})

	stdout, stderr, err := runCmd(t, []string{"import", writeImport(t, body)}, []string{masterPassword}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "" {
		t.Fatalf("stdout %q", stdout)
	}
	want := "no fields added\nskipped 1 field\nfiliz/vercel/password\n"
	if stderr != want {
		t.Fatalf("stderr %q", stderr)
	}
	if !bytes.Equal(before, readFile(t, path)) {
		t.Fatal("vault file changed")
	}
	assertNoLeak(t, stderr, secretValue, "OVERWRITE-SECRET-VALUE")
	got, err := loadVault(t, path, masterPassword).GetField("filiz", "vercel", "password")
	if err != nil {
		t.Fatal(err)
	}
	if got.Value != secretValue || !got.Secret {
		t.Fatalf("field = %+v", got)
	}
}

func TestImportEmptyDoesNotAskPassword(t *testing.T) {
	path := setVaultEnv(t)
	initVault(t)
	before := readFile(t, path)
	body := `{"version":1,"projects":{"empty":{"services":{}}}}`

	stdout, stderr, err := runCmd(t, []string{"import", writeImport(t, body)}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "" || stderr != "no fields added\n" {
		t.Fatalf("stdout %q stderr %q", stdout, stderr)
	}
	if !bytes.Equal(before, readFile(t, path)) {
		t.Fatal("vault file changed")
	}
}

func TestImportRejectsFileBeforePassword(t *testing.T) {
	const marker = "DISTINCTIVE-SECRET-VALUE"
	tests := []struct {
		name  string
		body  string
		check func(error) bool
	}{
		{
			name: "broken",
			body: `{"version":1,"projects":`,
			check: func(err error) bool {
				return errors.Is(err, vault.ErrInvalidImport) && !errors.Is(err, vault.ErrUnsupportedVersion)
			},
		},
		{
			name: "version",
			body: `{"version":2,"token":"` + marker + `"}`,
			check: func(err error) bool {
				return errors.Is(err, vault.ErrUnsupportedVersion) && !errors.Is(err, vault.ErrInvalidImport)
			},
		},
		{
			name: "unknown",
			body: `{"version":1,"projects":{},"token":"` + marker + `"}`,
			check: func(err error) bool {
				return errors.Is(err, vault.ErrInvalidImport) && !errors.Is(err, vault.ErrUnsupportedVersion)
			},
		},
		{
			name: "missing secret",
			body: `{"version":1,"projects":{"my-project":{"services":{"vercel":{"fields":{"password":{"value":"` + marker + `"}}}}}}}`,
			check: func(err error) bool {
				var missing *vault.MissingSecretError
				return errors.As(err, &missing) && *missing == (vault.MissingSecretError{Project: "my-project", Service: "vercel", Field: "password"})
			},
		},
		{
			name: "case conflict",
			body: `{"version":1,"projects":{"filiz":{"services":{"vercel":{"fields":{"Email":{"value":"first-secret-value","secret":true},"email":{"value":"` + marker + `","secret":false}}}}}}}`,
			check: func(err error) bool {
				var conflict *vault.CaseConflictError
				return errors.As(err, &conflict) && conflict.Kind == "field"
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := setVaultEnv(t)
			initVault(t)
			before := readFile(t, path)
			_, stderr, err := runCmd(t, []string{"import", writeImport(t, tt.body)}, nil, nil)
			if err == nil || !tt.check(err) {
				t.Fatalf("got %v", err)
			}
			if strings.Contains(err.Error(), "scripted password") {
				t.Fatalf("asked for a password: %v", err)
			}
			assertNoLeak(t, stderr+err.Error(), marker, "first-secret-value", masterPassword)
			if !bytes.Equal(before, readFile(t, path)) {
				t.Fatal("vault file changed")
			}
		})
	}
}

func TestImportMissingFile(t *testing.T) {
	path := setVaultEnv(t)
	_, _, err := runCmd(t, []string{"import", filepath.Join(t.TempDir(), "missing.json")}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "read import file") {
		t.Fatalf("got %v", err)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("vault created")
	}
}

func TestImportRequiresVault(t *testing.T) {
	path := setVaultEnv(t)
	body := importBody(t, map[string]vault.Field{
		"filiz/vercel/password": {Value: secretValue, Secret: true},
	})
	_, _, err := runCmd(t, []string{"import", writeImport(t, body)}, []string{masterPassword}, nil)
	if !errors.Is(err, vault.ErrVaultNotFound) {
		t.Fatalf("got %v", err)
	}
	assertNoLeak(t, err.Error(), secretValue, masterPassword)
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("vault created")
	}
}

func TestImportWrongPasswordLeavesVault(t *testing.T) {
	path := setVaultEnv(t)
	initVault(t)
	before := readFile(t, path)
	body := importBody(t, map[string]vault.Field{
		"filiz/vercel/password": {Value: secretValue, Secret: true},
	})
	wrong := "incorrect-horse-xx"
	stdout, stderr, err := runCmd(t, []string{"import", writeImport(t, body)}, []string{wrong}, nil)
	if !errors.Is(err, vault.ErrWrongPassword) {
		t.Fatalf("got %v", err)
	}
	assertNoLeak(t, stdout+stderr+err.Error(), secretValue, masterPassword, wrong)
	if !bytes.Equal(before, readFile(t, path)) {
		t.Fatal("vault file changed")
	}
}

func TestImportUsage(t *testing.T) {
	path := setVaultEnv(t)
	_, _, err := runCmd(t, []string{"import"}, nil, nil)
	if err == nil || strings.Contains(err.Error(), "password") {
		t.Fatalf("got %v", err)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("vault created")
	}
}

func importBody(t *testing.T, fields map[string]vault.Field) string {
	t.Helper()
	projects := map[string]map[string]map[string]vault.Field{}
	for path, field := range fields {
		parts := strings.Split(path, "/")
		if len(parts) != 3 {
			t.Fatalf("bad field path %s", path)
		}
		project, service, name := parts[0], parts[1], parts[2]
		if projects[project] == nil {
			projects[project] = map[string]map[string]vault.Field{}
		}
		if projects[project][service] == nil {
			projects[project][service] = map[string]vault.Field{}
		}
		projects[project][service][name] = field
	}
	var b strings.Builder
	b.WriteString(`{"version":1,"projects":{`)
	projectSep := ""
	for project, services := range projects {
		fmt.Fprintf(&b, `%s%q:{"services":{`, projectSep, project)
		projectSep = ","
		serviceSep := ""
		for service, names := range services {
			fmt.Fprintf(&b, `%s%q:{"fields":{`, serviceSep, service)
			serviceSep = ","
			fieldSep := ""
			for name, field := range names {
				fmt.Fprintf(&b, `%s%q:{"value":%q,"secret":%t}`, fieldSep, name, field.Value, field.Secret)
				fieldSep = ","
			}
			b.WriteString("}}")
		}
		b.WriteString("}}")
	}
	b.WriteString("}}")
	return b.String()
}

func writeImport(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
