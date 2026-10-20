package cmd

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/emircancakmakk/cred/internal/terminalui"

	"github.com/emircancakmakk/cred/internal/crypto"
	"github.com/emircancakmakk/cred/internal/vault"
)

var errPasswordCancelled = errors.New("password entry cancelled")

const passwordAttempts = 3

func passwordFormSpec(prompt, context string) terminalui.FormSpec {
	// The legacy Prompter text ends with a field label. The shared form already
	// renders that label, so only retain notices preceding it.
	notice := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(prompt), strings.TrimSpace(promptPassword)))
	return terminalui.FormSpec{Context: context, Title: "Open vault", Description: notice, Submit: "Open", Fields: []terminalui.Field{{Label: "Master password", Sensitive: true, Required: true}}}
}

func readPasswordScreen(prompt string, in io.Reader, out io.Writer) ([]byte, error) {
	values, err := terminalui.RunForm(passwordFormSpec(prompt, "Vault"), in, out)
	if errors.Is(err, terminalui.ErrCancelled) {
		return nil, errPasswordCancelled
	}
	if err != nil {
		return nil, fmt.Errorf("read password: %w", err)
	}
	return []byte(values[0]), nil
}

// Prepare/lock/sync happen outside this loop. Only decryption is retried.
func (a *App) readUnlockedVault(p Prompter, path string) (*vault.Vault, []byte, error) {
	prompt := promptPassword
	for attempt := 1; ; {
		password, err := p.ReadPassword(prompt)
		if err != nil {
			crypto.Wipe(password)
			return nil, nil, err
		}
		if len(password) == 0 {
			crypto.Wipe(password)
			prompt = "Password is required.\n\n" + strings.TrimPrefix(prompt, "Password is required.\n\n")
			continue
		}
		loaded, err := a.loadVault(path, password)
		if err == nil {
			return loaded, password, nil
		}
		crypto.Wipe(password)
		if !errors.Is(err, vault.ErrWrongPassword) {
			return nil, nil, err
		}
		if attempt == passwordAttempts {
			return nil, nil, fmt.Errorf("unlock failed after %d attempts: %w", passwordAttempts, err)
		}
		remaining := fmt.Sprintf("%d attempts remaining", passwordAttempts-attempt)
		if passwordAttempts-attempt == 1 {
			remaining = "1 attempt remaining"
		}
		prompt = fmt.Sprintf("Wrong master password. %s.\n\n%s", remaining, promptPassword)
		attempt++
	}
}
