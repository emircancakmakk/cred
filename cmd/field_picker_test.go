package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/emircancakmakk/cred/internal/terminalui"
)

type fieldPickerPrompter struct {
	*scriptPrompter
	t       *testing.T
	choices []string
	names   []string
	cancel  bool
}

func (p *fieldPickerPrompter) Select(spec terminalui.SelectionSpec) (string, error) {
	if len(p.choices) == 0 {
		p.t.Fatal("unexpected selection")
	}
	choice := p.choices[0]
	p.choices = p.choices[1:]
	if choice == "cancel" {
		return "", terminalui.ErrCancelled
	}
	for _, option := range spec.Options {
		if option.Value == choice {
			return choice, nil
		}
	}
	p.t.Fatalf("choice %q missing from %v", choice, spec.Options)
	return "", nil
}

func (p *fieldPickerPrompter) Form(spec terminalui.FormSpec) ([]string, error) {
	if p.cancel && spec.Title == "Custom field" {
		return nil, terminalui.ErrCancelled
	}
	if spec.Title == "Custom field" {
		if spec.Validate == nil || spec.Validate([]string{"   "}) == nil {
			p.t.Fatal("blank custom name accepted")
		}
		name := p.names[0]
		p.names = p.names[1:]
		return []string{name}, nil
	}
	value, err := p.ReadPassword(spec.Title)
	return []string{string(value)}, err
}

func TestAddWithFieldPicker(t *testing.T) {
	for _, ending := range []string{"finish", "cancel", "duplicate", "custom cancel"} {
		t.Run(ending, func(t *testing.T) {
			path := setVaultEnv(t)
			initVault(t)
			p := &fieldPickerPrompter{t: t, scriptPrompter: &scriptPrompter{passwords: []string{masterPassword, "email-value", "password-value", "username-value", "url-value", "api_key-value", "token-value", "custom-value"}},
				choices: []string{"email", "n", "password", "y", "username", "n", "url", "n", "api_key", "y", "token", "y", "custom", "y"}, names: []string{"custom"}}
			switch ending {
			case "finish":
				p.choices = append(p.choices, "")
			case "cancel":
				p.choices = append(p.choices, "cancel")
			case "duplicate":
				p.choices = append(p.choices, "email", "n")
				p.passwords = append(p.passwords, "duplicate-value")
			case "custom cancel":
				p.choices = []string{"custom"}
				p.cancel = true
			}
			var output bytes.Buffer
			app := &App{Prompter: p, Out: &output, Err: &output, WorkFactor: 1}
			err := app.Execute([]string{"add", "p", "s"})
			loaded := loadVault(t, path, masterPassword)
			if ending != "finish" {
				if err == nil || len(loaded.Projects) != 0 {
					t.Fatal("failed/cancelled run saved fields")
				}
				if ending != "duplicate" && !errors.Is(err, terminalui.ErrCancelled) {
					t.Fatal(err)
				}
				if ending == "duplicate" && !strings.Contains(err.Error(), "already exists") {
					t.Fatal(err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"email", "password", "username", "url", "api_key", "token", "custom"} {
				field, err := loaded.GetField("p", "s", name)
				if err != nil || field.Value != name+"-value" || field.Secret != (name == "password" || name == "api_key" || name == "token" || name == "custom") {
					t.Fatalf("wrong saved field %s: %+v, %v", name, field, err)
				}
			}
			if len(p.choices) != 0 || len(p.names) != 0 {
				t.Fatal("unused picker input")
			}
			assertNoLeak(t, output.String(), "password-value", "api_key-value", "token-value", "custom-value")
		})
	}
}
