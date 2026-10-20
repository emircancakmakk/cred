package cmd

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emircancakmakk/cred/internal/terminalui"
	"github.com/emircancakmakk/cred/internal/vault"
)

func TestPasswordProgramDoesNotRenderSubmittedPassword(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		var out bytes.Buffer
		input := "program-test-password\r\r"
		if cancel {
			input = "program-test-password\x03"
		}
		password, err := readPasswordScreen(promptPassword, strings.NewReader(input), &out)
		if cancel {
			if !errors.Is(err, errPasswordCancelled) || password != nil {
				t.Fatal("program did not cancel")
			}
		} else if err != nil || string(password) != "program-test-password" {
			t.Fatalf("program did not return password: %v", err)
		}
		assertNoLeak(t, out.String(), "program-test-password")
		if !strings.Contains(out.String(), "\x1b[?1049h") || !strings.Contains(out.String(), "\x1b[?1049l") {
			t.Fatal("alternate screen lifecycle missing")
		}
	}
}

type recordingPasswords struct {
	answers [][]byte
	prompts []string
	err     error
}

func (p *recordingPasswords) ReadLine(string) (string, error) { return "", io.EOF }
func (p *recordingPasswords) ReadPassword(prompt string) ([]byte, error) {
	p.prompts = append(p.prompts, prompt)
	index := len(p.prompts) - 1
	if index >= len(p.answers) {
		return nil, p.err
	}
	return p.answers[index], nil
}

func TestUnlockRetriesAndWipesFailedPasswords(t *testing.T) {
	path := setVaultEnv(t)
	initVault(t)
	wrong := []byte("wrong-password")
	p := &recordingPasswords{answers: [][]byte{nil, wrong, []byte(masterPassword)}}
	app := &App{Out: io.Discard, Err: io.Discard}
	loaded, password, err := app.readUnlockedVault(p, path)
	if err != nil || loaded == nil || string(password) != masterPassword || len(p.prompts) != 3 {
		t.Fatalf("wrong/correct sequence failed: %v", err)
	}
	if !bytes.Equal(wrong, make([]byte, len(wrong))) {
		t.Fatal("failed password not wiped")
	}
	if !strings.Contains(p.prompts[1], "Password is required") || !strings.Contains(p.prompts[2], "2 attempts remaining") {
		t.Fatal("retry message/count incorrect")
	}
}

func TestUnlockStopsAtThreeWrongPasswords(t *testing.T) {
	path := setVaultEnv(t)
	initVault(t)
	p := &recordingPasswords{answers: [][]byte{[]byte("bad-one"), []byte("bad-two"), []byte("bad-three"), []byte(masterPassword)}}
	app := &App{Out: io.Discard, Err: io.Discard}
	_, password, err := app.readUnlockedVault(p, path)
	if !errors.Is(err, vault.ErrWrongPassword) || password != nil || len(p.prompts) != 3 {
		t.Fatalf("wrong attempt limit: %v", err)
	}
	if !strings.Contains(p.prompts[2], "1 attempt remaining") {
		t.Fatal("last retry not announced")
	}
	for _, attempt := range p.answers[:3] {
		if !bytes.Equal(attempt, make([]byte, len(attempt))) {
			t.Fatal("failed attempt retained password")
		}
	}
}

func TestUnlockDoesNotRetryOtherErrors(t *testing.T) {
	for _, kind := range []string{"missing", "corrupt", "cancel", "input-error"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "vault.age")
			p := &recordingPasswords{answers: [][]byte{[]byte(masterPassword)}}
			want := vault.ErrVaultNotFound
			switch kind {
			case "corrupt":
				if err := os.WriteFile(path, []byte("invalid vault"), 0600); err != nil {
					t.Fatal(err)
				}
				want = vault.ErrCorruptVault
			case "cancel", "input-error":
				want = errPasswordCancelled
				if kind == "input-error" {
					want = io.ErrUnexpectedEOF
				}
				p.answers, p.err = nil, want
			}
			app := &App{Out: io.Discard, Err: io.Discard}
			_, password, err := app.readUnlockedVault(p, path)
			if !errors.Is(err, want) || password != nil || len(p.prompts) != 1 {
				t.Fatalf("non-password error retried: %v", err)
			}
		})
	}
}

func TestPasswordScreenHasOneFieldLabel(t *testing.T) {
	for _, context := range []string{"Vault", "Setup"} {
		for _, notice := range []string{"", "Wrong master password. 2 attempts remaining.", "Wrong master password. 1 attempt remaining.", "Password is required."} {
			t.Run(context+"/"+notice, func(t *testing.T) {
				prompt := promptPassword
				if notice != "" {
					prompt = notice + "\n\n" + promptPassword
				}
				spec := passwordFormSpec(prompt, context)
				view := terminalui.NewForm(spec).View()
				if strings.Count(view, "Master password") != 1 {
					t.Fatalf("password label repeated:\n%s", view)
				}
				if spec.Title != "Open vault" || spec.Description != notice {
					t.Fatalf("unexpected title/notice: %q / %q", spec.Title, spec.Description)
				}
				if spec.Context != context || !spec.Fields[0].Sensitive || !spec.Fields[0].Required {
					t.Fatal("password semantics changed")
				}
			})
		}
	}
}
