package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/emircancakmakk/cred/internal/crypto"
	"github.com/emircancakmakk/cred/internal/profiles"
	"github.com/emircancakmakk/cred/internal/vault"
)

const (
	masterPassword = "correct-horse"
	emailValue     = "visible-email@example.com"
	secretValue    = "SUPER-SECRET-VALUE"
)

func TestParseSecret(t *testing.T) {
	current := true
	tests := []struct {
		name    string
		answer  string
		current *bool
		want    bool
		wantErr bool
	}{
		{name: "empty add", answer: "", want: false},
		{name: "empty keeps current", answer: "", current: &current, want: true},
		{name: "yes", answer: "Y", want: true},
		{name: "no overrides current", answer: "no", current: &current, want: false},
		{name: "garbage", answer: "maybe", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSecret(tt.answer, tt.current)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseConfirm(t *testing.T) {
	tests := []struct {
		answer string
		want   bool
	}{
		{answer: "", want: false},
		{answer: "n", want: false},
		{answer: "no", want: false},
		{answer: "maybe", want: false},
		{answer: "y", want: true},
		{answer: "YES", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.answer, func(t *testing.T) {
			if got := parseConfirm(tt.answer); got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRenderVault(t *testing.T) {
	if got := renderVault(vault.New()); got != "vault is empty\n" {
		t.Fatalf("empty = %q", got)
	}
	v := vault.New()
	mustAdd(t, v, "foo", "github", "password", vault.Field{Value: secretValue, Secret: true})
	mustAdd(t, v, "filiz", "vercel", "email", vault.Field{Value: emailValue, Secret: false})
	mustAdd(t, v, "filiz", "vercel", "password", vault.Field{Value: secretValue, Secret: true})
	mustAdd(t, v, "filiz", "digital-ocean", "token", vault.Field{Value: secretValue, Secret: true})
	if err := v.DeleteService("foo", "github"); err != nil {
		t.Fatal(err)
	}

	got := renderVault(v)
	want := "filiz\n  digital-ocean\n  vercel\nfoo\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	assertNoLeak(t, got, secretValue, emailValue)
}

func TestInitAndRefuseOverwrite(t *testing.T) {
	path := setLocalInitEnv(t)
	stderr, err := runOK(t, []string{"init", "local"}, []string{masterPassword, masterPassword}, nil)
	if !strings.Contains(stderr, "created "+path) {
		t.Fatalf("stderr %q", stderr)
	}
	assertMode(t, filepath.Dir(path), 0o700)
	assertMode(t, path, 0o600)
	assertFileHides(t, path, masterPassword)

	loaded := loadVault(t, path, masterPassword)
	if loaded.Version != 1 || len(loaded.Projects) != 0 {
		t.Fatalf("vault = %+v", loaded)
	}

	_, _, err = runCmd(t, []string{"init", "local"}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "profile already exists") {
		t.Fatalf("second init: %v", err)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatal(statErr)
	}
}

func TestInitPasswordRules(t *testing.T) {
	tests := []struct {
		name      string
		passwords []string
		want      error
	}{
		{name: "empty", passwords: []string{""}, want: vault.ErrEmptyPassword},
		{name: "11 ascii", passwords: []string{"12345678901"}, want: vault.ErrPasswordTooShort},
		{name: "11 runes over 12 bytes", passwords: []string{strings.Repeat("ş", 11)}, want: vault.ErrPasswordTooShort},
		{name: "mismatch", passwords: []string{masterPassword, "correct-horse-other"}, want: errMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := setLocalInitEnv(t)
			_, _, err := runCmd(t, []string{"init", "local"}, tt.passwords, nil)
			if tt.want == errMismatch {
				if err == nil || err.Error() != "passwords do not match" {
					t.Fatalf("got %v", err)
				}
			} else if !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
			assertNoLeak(t, errString(err), tt.passwords...)
			if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatal("init created a vault")
			}
		})
	}
}

func TestInitAcceptsTwelveRunes(t *testing.T) {
	password := strings.Repeat("ş", 12)
	if utf8.RuneCountInString(password) != 12 || len(password) <= 12 {
		t.Fatal("fixture must be 12 runes and more than 12 bytes")
	}
	path := setLocalInitEnv(t)
	if _, err := runOK(t, []string{"init", "local"}, []string{password, password}, nil); err != nil {
		t.Fatal(err)
	}
	stdout, err := runOK(t, []string{"ls"}, []string{password}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "vault is empty\n" {
		t.Fatalf("stdout %q", stdout)
	}
	assertFileHides(t, path, password)
}

func TestOpenDoesNotEnforcePasswordLength(t *testing.T) {
	path := setVaultEnv(t)
	short := []byte("short")
	if err := vault.ValidateMasterPassword(short); err == nil {
		t.Fatal("fixture should fail validation")
	}
	if err := vault.Save(path, short, vault.New(), fastOptions()); err != nil {
		t.Fatal(err)
	}
	stdout, err := runOK(t, []string{"ls"}, []string{"short"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "vault is empty\n" {
		t.Fatalf("stdout %q", stdout)
	}
}

func TestAddSetAndList(t *testing.T) {
	path := setVaultEnv(t)
	initVault(t)

	stderr, err := runOK(t, []string{"add", "filiz"}, []string{masterPassword, emailValue, secretValue}, []string{
		"vercel", "email", "n", "password", "y", "",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "added 2 fields") {
		t.Fatalf("stderr %q", stderr)
	}
	assertFileHides(t, path, secretValue, emailValue, masterPassword)

	stdout, err := runOK(t, []string{"ls"}, []string{masterPassword}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "filiz\n  vercel\n" {
		t.Fatalf("stdout %q", stdout)
	}
	assertNoLeak(t, stdout, secretValue, emailValue)

	_, err = runOK(t, []string{"set", "filiz", "vercel", "password"}, []string{masterPassword, "ROTATED-SECRET-VALUE"}, []string{""})
	if err != nil {
		t.Fatal(err)
	}
	got, err := loadVault(t, path, masterPassword).GetField("filiz", "vercel", "password")
	if err != nil {
		t.Fatal(err)
	}
	if got != (vault.Field{Value: "ROTATED-SECRET-VALUE", Secret: true}) {
		t.Fatalf("after empty secret answer: %+v", got)
	}

	_, err = runOK(t, []string{"set", "filiz", "vercel", "password"}, []string{masterPassword, "VISIBLE-NOW"}, []string{"n"})
	if err != nil {
		t.Fatal(err)
	}
	got, err = loadVault(t, path, masterPassword).GetField("filiz", "vercel", "password")
	if err != nil {
		t.Fatal(err)
	}
	if got != (vault.Field{Value: "VISIBLE-NOW", Secret: false}) {
		t.Fatalf("after secret no: %+v", got)
	}

	stdout, err = runOK(t, []string{"ls"}, []string{masterPassword}, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertNoLeak(t, stdout, "ROTATED-SECRET-VALUE", "VISIBLE-NOW", emailValue)
}

func TestAddDuplicateField(t *testing.T) {
	path := setVaultEnv(t)
	initVault(t)
	addPassword(t, path)

	_, _, err := runCmd(t, []string{"add", "filiz", "vercel"}, []string{masterPassword, "other-secret"}, []string{"password", "y"})
	want := "filiz/vercel/password already exists. To change it: cred set filiz vercel password"
	if err == nil || err.Error() != want {
		t.Fatalf("got %v", err)
	}
	got, err := loadVault(t, path, masterPassword).GetField("filiz", "vercel", "password")
	if err != nil {
		t.Fatal(err)
	}
	if got.Value != secretValue {
		t.Fatalf("value changed to %q", got.Value)
	}
}

func TestAddCaseConflictDoesNotSave(t *testing.T) {
	path := setVaultEnv(t)
	initVault(t)
	addPassword(t, path)
	before := readFile(t, path)

	_, _, err := runCmd(t, []string{"add", "Filiz", "vercel"}, []string{masterPassword, "other-secret"}, []string{"token", "y"})
	var conflict *vault.CaseConflictError
	if !errors.As(err, &conflict) || conflict.Kind != "project" {
		t.Fatalf("got %v", err)
	}
	if !bytes.Equal(before, readFile(t, path)) {
		t.Fatal("vault file changed")
	}
}

func TestAddNoFields(t *testing.T) {
	path := setVaultEnv(t)
	initVault(t)
	before := readFile(t, path)
	stderr, err := runOK(t, []string{"add", "filiz", "vercel"}, []string{masterPassword}, []string{""})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "no fields added") {
		t.Fatalf("stderr %q", stderr)
	}
	if !bytes.Equal(before, readFile(t, path)) {
		t.Fatal("vault file changed")
	}
}

func TestSetMissingField(t *testing.T) {
	path := setVaultEnv(t)
	initVault(t)
	addPassword(t, path)
	_, _, err := runCmd(t, []string{"set", "filiz", "vercel", "missing"}, []string{masterPassword}, nil)
	if !errors.Is(err, vault.ErrFieldNotFound) {
		t.Fatalf("got %v", err)
	}
	if _, getErr := loadVault(t, path, masterPassword).GetField("filiz", "vercel", "password"); getErr != nil {
		t.Fatal(getErr)
	}
}

func TestRmConfirmation(t *testing.T) {
	path := setVaultEnv(t)
	initVault(t)
	addPassword(t, path)
	_, err := runOK(t, []string{"add", "filiz", "vercel"}, []string{masterPassword, emailValue}, []string{"email", "n", ""})
	if err != nil {
		t.Fatal(err)
	}

	before := readFile(t, path)
	for _, answer := range []string{"", "n", "maybe"} {
		stderr, err := runOK(t, []string{"rm", "filiz", "vercel", "password"}, []string{masterPassword}, []string{answer})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(stderr, "cancelled") {
			t.Fatalf("answer %q stderr %q", answer, stderr)
		}
		if !bytes.Equal(before, readFile(t, path)) {
			t.Fatalf("answer %q changed the vault", answer)
		}
	}

	stderr, err := runOK(t, []string{"rm", "filiz", "vercel", "password"}, []string{masterPassword}, []string{"y"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "deleted filiz/vercel/password") {
		t.Fatalf("stderr %q", stderr)
	}
	loaded := loadVault(t, path, masterPassword)
	if _, err := loaded.GetField("filiz", "vercel", "password"); !errors.Is(err, vault.ErrFieldNotFound) {
		t.Fatal("password still present")
	}
	if _, err := loaded.GetField("filiz", "vercel", "email"); err != nil {
		t.Fatal(err)
	}
	if _, ok := loaded.Projects["filiz"].Services["vercel"]; !ok {
		t.Fatal("service removed with its field")
	}

	if _, err := runOK(t, []string{"rm", "filiz", "vercel"}, []string{masterPassword}, []string{"yes"}); err != nil {
		t.Fatal(err)
	}
	loaded = loadVault(t, path, masterPassword)
	if _, ok := loaded.Projects["filiz"]; !ok {
		t.Fatal("project removed with its service")
	}
	if _, ok := loaded.Projects["filiz"].Services["vercel"]; ok {
		t.Fatal("service still present")
	}

	if _, err := runOK(t, []string{"rm", "filiz"}, []string{masterPassword}, []string{"y"}); err != nil {
		t.Fatal(err)
	}
	if projects := loadVault(t, path, masterPassword).Projects; len(projects) != 0 {
		t.Fatalf("projects = %+v", projects)
	}
}

func TestRmPrefixDoesNotMatch(t *testing.T) {
	path := setVaultEnv(t)
	initVault(t)
	addPassword(t, path)
	_, _, err := runCmd(t, []string{"rm", "fil"}, []string{masterPassword}, []string{"y"})
	if !errors.Is(err, vault.ErrProjectNotFound) {
		t.Fatalf("got %v", err)
	}
	if _, getErr := loadVault(t, path, masterPassword).GetField("filiz", "vercel", "password"); getErr != nil {
		t.Fatal(getErr)
	}
}

func TestWrongPasswordDoesNotLeak(t *testing.T) {
	setVaultEnv(t)
	initVault(t)
	wrong := "incorrect-horse-xx"
	stdout, stderr, err := runCmd(t, []string{"ls"}, []string{wrong, wrong, wrong}, nil)
	if !errors.Is(err, vault.ErrWrongPassword) {
		t.Fatalf("got %v", err)
	}
	assertNoLeak(t, stdout+stderr+err.Error(), masterPassword, wrong)
}

func TestMissingVaultHintsInit(t *testing.T) {
	setVaultEnv(t)
	_, _, err := runCmd(t, []string{"add", "filiz", "vercel"}, []string{masterPassword}, nil)
	if !errors.Is(err, vault.ErrVaultNotFound) {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(err.Error(), "run cred init") {
		t.Fatalf("got %v", err)
	}
}

func TestHelpAndUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	app := &App{Out: &stdout, Err: &stderr}
	if err := app.Execute([]string{"--help"}); err != nil {
		t.Fatal(err)
	}
	help := stdout.String() + stderr.String()
	for _, name := range []string{"init", "add", "import", "set", "ls", "rm", "--show"} {
		if !strings.Contains(help, name) {
			t.Fatalf("help missing %s:\n%s", name, help)
		}
	}
	if strings.Contains(help, "__copy") || strings.Contains(help, "__clipboard_clear") || strings.Contains(help, "__tui") {
		t.Fatalf("hidden command listed:\n%s", help)
	}
	if !strings.Contains(help, "cred my-project") || !strings.Contains(help, "5 seconds") {
		t.Fatalf("help missing selector:\n%s", help)
	}

	err := app.Execute(nil)
	if err == nil || err.Error() != "interactive terminal required" {
		t.Fatalf("got %v", err)
	}
	err = app.Execute([]string{"nope"})
	if err == nil || err.Error() != "interactive terminal required" {
		t.Fatalf("got %v", err)
	}
	err = app.Execute([]string{"nope", "extra"})
	if !errors.Is(err, errCopyUsage) {
		t.Fatalf("got %v", err)
	}
}

func TestCopyAndShow(t *testing.T) {
	saveCopyVault(t)
	fake := &fakeCopier{}
	stdout, stderr, err := runApp(t, []string{"fil", "ver", "pass"}, []string{masterPassword}, nil, fake)
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "" {
		t.Fatalf("stdout %q", stdout)
	}
	if stderr != "copied filiz/vercel/password\n" {
		t.Fatalf("stderr %q", stderr)
	}
	if fake.text != secretValue {
		t.Fatalf("copied %q", fake.text)
	}
	assertNoLeak(t, stdout+stderr, secretValue, "OTHER-TOKEN-SECRET", "EXACT-FI-SECRET")

	fake = &fakeCopier{}
	stdout, stderr, err = runApp(t, []string{"FILIZ", "VERCEL", "PASSWORD"}, []string{masterPassword}, nil, fake)
	if err != nil {
		t.Fatal(err)
	}
	if fake.text != secretValue {
		t.Fatalf("case copy = %q", fake.text)
	}

	fake = &fakeCopier{}
	stdout, stderr, err = runApp(t, []string{"fi", "vercel", "password"}, []string{masterPassword}, nil, fake)
	if err != nil {
		t.Fatal(err)
	}
	if fake.text != "EXACT-FI-SECRET" {
		t.Fatalf("exact match copied %q", fake.text)
	}

	fake = &fakeCopier{}
	stdout, stderr, err = runApp(t, []string{"--show", "filiz", "vercel", "password"}, []string{masterPassword}, nil, fake)
	if err != nil {
		t.Fatal(err)
	}
	if fake.called {
		t.Fatal("--show copied to the clipboard")
	}
	if stdout != secretValue+"\n" {
		t.Fatalf("stdout %q", stdout)
	}
	assertNoLeak(t, stderr, secretValue, "OTHER-TOKEN-SECRET")

	fake = &fakeCopier{}
	stdout, stderr, err = runApp(t, []string{"filiz", "vercel", "note", "--show"}, []string{masterPassword}, nil, fake)
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "line1\nline2\n" {
		t.Fatalf("stdout %q", stdout)
	}

	fake = &fakeCopier{}
	stdout, stderr, err = runApp(t, []string{"f", "vercel", "password"}, []string{masterPassword}, nil, fake)
	var ambiguous *vault.AmbiguousError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("got %v", err)
	}
	if strings.Join(ambiguous.Matches, ",") != "fi,filiz,foo" {
		t.Fatalf("matches = %v", ambiguous.Matches)
	}
	if fake.called {
		t.Fatal("ambiguous query copied a value")
	}
	assertNoLeak(t, stdout+stderr+err.Error(), secretValue, "OTHER-TOKEN-SECRET", "EXACT-FI-SECRET", "FOO-TOKEN-SECRET")

	fake = &fakeCopier{}
	_, _, err = runApp(t, []string{"filiz", "v", "password"}, []string{masterPassword}, nil, fake)
	if !errors.As(err, &ambiguous) || ambiguous.Kind != "service" {
		t.Fatalf("got %v", err)
	}
	if fake.called {
		t.Fatal("ambiguous service copied a value")
	}
}

func TestCopyWrongPasswordDoesNotCopy(t *testing.T) {
	saveCopyVault(t)
	fake := &fakeCopier{}
	wrong := "incorrect-horse-xx"
	stdout, stderr, err := runApp(t, []string{"filiz", "vercel", "password"}, []string{wrong, wrong, wrong}, nil, fake)
	if !errors.Is(err, vault.ErrWrongPassword) {
		t.Fatalf("got %v", err)
	}
	if fake.called {
		t.Fatal("wrong password copied a value")
	}
	assertNoLeak(t, stdout+stderr+err.Error(), masterPassword, wrong, secretValue)
}

func TestCopyFailureHidesValue(t *testing.T) {
	saveCopyVault(t)
	fake := &fakeCopier{err: errors.New("clipboard failed")}
	_, _, err := runApp(t, []string{"filiz", "vercel", "password"}, []string{masterPassword}, nil, fake)
	if err == nil || !strings.Contains(err.Error(), "copy filiz/vercel/password") {
		t.Fatalf("got %v", err)
	}
	assertNoLeak(t, err.Error(), secretValue)
}

func TestSelectorResolvesProject(t *testing.T) {
	saveCopyVault(t)
	rec := &tuiRec{}
	stdout, stderr, err := runApp(t, nil, []string{masterPassword}, nil, nil, rec.run)
	if err != nil {
		t.Fatal(err)
	}
	if !rec.called || rec.project != "" {
		t.Fatalf("project list start %q called %v", rec.project, rec.called)
	}
	assertNoLeak(t, stdout+stderr, secretValue, "OTHER-TOKEN-SECRET", "EXACT-FI-SECRET", emailValue)

	rec = &tuiRec{}
	stdout, stderr, err = runApp(t, []string{"fil"}, []string{masterPassword}, nil, nil, rec.run)
	if err != nil {
		t.Fatal(err)
	}
	if !rec.called || rec.project != "filiz" {
		t.Fatalf("selector project %q called %v", rec.project, rec.called)
	}
	assertNoLeak(t, stdout+stderr, secretValue, "OTHER-TOKEN-SECRET", "EXACT-FI-SECRET", emailValue)

	rec = &tuiRec{}
	stdout, stderr, err = runApp(t, []string{"fi"}, []string{masterPassword}, nil, nil, rec.run)
	if err != nil {
		t.Fatal(err)
	}
	if rec.project != "fi" {
		t.Fatalf("exact project %q", rec.project)
	}

	rec = &tuiRec{}
	stdout, stderr, err = runApp(t, []string{"FILIZ"}, []string{masterPassword}, nil, nil, rec.run)
	if err != nil {
		t.Fatal(err)
	}
	if rec.project != "filiz" {
		t.Fatalf("folded project %q", rec.project)
	}

	rec = &tuiRec{}
	stdout, stderr, err = runApp(t, []string{"f"}, []string{masterPassword}, nil, nil, rec.run)
	var ambiguous *vault.AmbiguousError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("got %v", err)
	}
	if strings.Join(ambiguous.Matches, ",") != "fi,filiz,foo" {
		t.Fatalf("matches %v", ambiguous.Matches)
	}
	if rec.called {
		t.Fatal("ambiguous name started the selector")
	}
	assertNoLeak(t, stdout+stderr+err.Error(), secretValue, "OTHER-TOKEN-SECRET", "EXACT-FI-SECRET", emailValue)

	rec = &tuiRec{}
	_, _, err = runApp(t, []string{"nope"}, []string{masterPassword}, nil, nil, rec.run)
	if !errors.Is(err, vault.ErrProjectNotFound) {
		t.Fatalf("got %v", err)
	}
	if rec.called {
		t.Fatal("missing project started the selector")
	}
}

func TestSelectorMissingVault(t *testing.T) {
	setVaultEnv(t)
	_, _, err := runCmd(t, []string{"filiz"}, []string{masterPassword}, nil)
	if !errors.Is(err, vault.ErrVaultNotFound) || !strings.Contains(err.Error(), "run cred init") {
		t.Fatalf("got %v", err)
	}
}

type tuiRec struct {
	called  bool
	project string
}

func (r *tuiRec) run(project string) error {
	r.called = true
	r.project = project
	return nil
}

func saveCopyVault(t *testing.T) {
	t.Helper()
	path := setVaultEnv(t)
	v := vault.New()
	mustAdd(t, v, "foo", "github", "token", vault.Field{Value: "FOO-TOKEN-SECRET", Secret: true})
	mustAdd(t, v, "filiz", "vercel", "email", vault.Field{Value: emailValue, Secret: false})
	mustAdd(t, v, "filiz", "vercel", "password", vault.Field{Value: secretValue, Secret: true})
	mustAdd(t, v, "filiz", "vercel", "note", vault.Field{Value: "line1\nline2", Secret: false})
	mustAdd(t, v, "filiz", "vault", "token", vault.Field{Value: "OTHER-TOKEN-SECRET", Secret: true})
	mustAdd(t, v, "fi", "vercel", "password", vault.Field{Value: "EXACT-FI-SECRET", Secret: true})
	if err := vault.Save(path, []byte(masterPassword), v, fastOptions()); err != nil {
		t.Fatal(err)
	}
}

type fakeCopier struct {
	text   string
	err    error
	called bool
}

func (f *fakeCopier) Copy(text string) error {
	f.called = true
	f.text = text
	return f.err
}

func TestAddRequiresProject(t *testing.T) {
	path := setVaultEnv(t)
	_, _, err := runCmd(t, []string{"add"}, nil, nil)
	if err == nil {
		t.Fatal("expected an error")
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("vault created without a project")
	}
}

type scriptPrompter struct {
	passwords []string
	lines     []string
	pi, li    int
}

func (s *scriptPrompter) ReadPassword(prompt string) ([]byte, error) {
	if s.pi >= len(s.passwords) {
		return nil, fmt.Errorf("no scripted password for %q", prompt)
	}
	password := s.passwords[s.pi]
	s.pi++
	return []byte(password), nil
}

func (s *scriptPrompter) ReadLine(prompt string) (string, error) {
	if s.li >= len(s.lines) {
		return "", fmt.Errorf("no scripted line for %q", prompt)
	}
	line := s.lines[s.li]
	s.li++
	return line, nil
}

func (s *scriptPrompter) remaining() error {
	if s.pi == len(s.passwords) && s.li == len(s.lines) {
		return nil
	}
	return fmt.Errorf("unused scripted input: %d passwords and %d lines", len(s.passwords)-s.pi, len(s.lines)-s.li)
}

func setVaultEnv(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cred", "vault.age")
	t.Setenv("CRED_VAULT", path)
	return path
}

func runCmd(t *testing.T, args, passwords, lines []string) (string, string, error) {
	t.Helper()
	return runApp(t, args, passwords, lines, nil)
}

func runApp(t *testing.T, args, passwords, lines []string, copier Copier, hooks ...func(string) error) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	script := &scriptPrompter{passwords: passwords, lines: lines}
	app := &App{
		Out:        &stdout,
		Err:        &stderr,
		Prompter:   script,
		Copier:     copier,
		WorkFactor: crypto.MinScryptLogN,
	}
	if len(hooks) > 0 {
		app.RunTUI = hooks[0]
	}
	err := app.Execute(args)
	if err == nil {
		if rem := script.remaining(); rem != nil {
			t.Fatalf("%s: %v", strings.Join(args, " "), rem)
		}
	}
	return stdout.String(), stderr.String(), err
}

func runOK(t *testing.T, args, passwords, lines []string) (string, error) {
	t.Helper()
	stdout, stderr, err := runCmd(t, args, passwords, lines)
	if err != nil {
		t.Fatalf("%s: %v\nstderr: %s", strings.Join(args, " "), err, stderr)
	}
	return stdout + stderr, nil
}

func initVault(t *testing.T) {
	t.Helper()
	path, err := vault.Path()
	if err != nil {
		t.Fatal(err)
	}
	if err = vault.Save(path, []byte(masterPassword), vault.New(), fastOptions()); err != nil {
		t.Fatal(err)
	}
}
func setLocalInitEnv(t *testing.T) string {
	t.Helper()
	isolateProfiles(t)
	path, e := profiles.NamedPath("local")
	if e != nil {
		t.Fatal(e)
	}
	t.Setenv("CRED_VAULT", path)
	return path
}

func addPassword(t *testing.T, path string) {
	t.Helper()
	if _, err := runOK(t, []string{"add", "filiz", "vercel"}, []string{masterPassword, secretValue}, []string{"password", "y", ""}); err != nil {
		t.Fatal(err)
	}
	got, err := loadVault(t, path, masterPassword).GetField("filiz", "vercel", "password")
	if err != nil {
		t.Fatal(err)
	}
	if got.Value != secretValue || !got.Secret {
		t.Fatalf("field = %+v", got)
	}
}

func loadVault(t *testing.T, path, password string) *vault.Vault {
	t.Helper()
	loaded, err := vault.Load(path, []byte(password))
	if err != nil {
		t.Fatal(err)
	}
	return loaded
}

func fastOptions() vault.Options {
	return vault.Options{WorkFactor: crypto.MinScryptLogN}
}

func mustAdd(t *testing.T, v *vault.Vault, project, service, name string, field vault.Field) {
	t.Helper()
	if err := v.Add(project, service, name, field); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
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

func assertFileHides(t *testing.T, path string, secrets ...string) {
	t.Helper()
	assertNoLeak(t, string(readFile(t, path)), secrets...)
}

func assertNoLeak(t *testing.T, text string, secrets ...string) {
	t.Helper()
	for _, secret := range secrets {
		if secret != "" && strings.Contains(text, secret) {
			t.Fatal("output contains secret material")
		}
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

var errMismatch = errors.New("passwords do not match")
