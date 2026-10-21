package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/emircancakmakk/cred/internal/terminalui"
	"golang.org/x/term"
)

// Rich prompts are optional so existing embedders and scripted Prompters keep
// their contract. Call sites supply semantic requests rather than parsing text.
type interfacePrompter interface {
	Form(terminalui.FormSpec) ([]string, error)
	Select(terminalui.SelectionSpec) (string, error)
}

func (p *terminalPrompter) Form(spec terminalui.FormSpec) ([]string, error) {
	if !term.IsTerminal(int(p.in.Fd())) {
		return nil, errors.New("interactive terminal required")
	}
	return terminalui.RunForm(spec, p.in, p.out)
}
func (p *terminalPrompter) Select(spec terminalui.SelectionSpec) (string, error) {
	if !term.IsTerminal(int(p.in.Fd())) {
		return "", errors.New("interactive terminal required")
	}
	return terminalui.RunSelection(spec, p.in, p.out)
}
func askLine(p Prompter, prompt, label string, required bool) (string, error) {
	if rich, ok := p.(interfacePrompter); ok {
		v, e := rich.Form(terminalui.FormSpec{Context: "Credentials", Title: label, Fields: []terminalui.Field{{Label: label, Required: required}}})
		if e != nil {
			return "", e
		}
		return v[0], nil
	}
	return p.ReadLine(prompt)
}
func askFieldName(p Prompter) (string, error) {
	if rich, ok := p.(interfacePrompter); ok {
		name, err := rich.Select(terminalui.FieldNameSelection("Credentials", true))
		if err != nil || name != terminalui.CustomFieldChoice {
			return name, err
		}
		values, err := rich.Form(terminalui.FormSpec{
			Context: "Credentials", Title: "Custom field", Fields: []terminalui.Field{{Label: "Field name", Required: true}},
			Validate: func(values []string) error {
				if strings.TrimSpace(values[0]) == "" {
					return errors.New("Field name is required.")
				}
				return nil
			},
		})
		if err != nil {
			return "", err
		}
		return values[0], nil
	}
	return p.ReadLine(promptField)
}

func askValue(p Prompter) ([]byte, error) {
	if rich, ok := p.(interfacePrompter); ok {
		v, e := rich.Form(terminalui.FormSpec{Context: "Credentials", Title: "Field value", Fields: []terminalui.Field{{Label: "Value", Sensitive: true}}})
		if e != nil {
			return nil, e
		}
		return []byte(v[0]), nil
	}
	return p.ReadPassword(promptValue)
}
func askConfirm(p Prompter, prompt, title, description, action string, destructive bool) (bool, error) {
	if rich, ok := p.(interfacePrompter); ok {
		value, e := rich.Select(terminalui.SelectionSpec{Context: "Confirm", Title: title, Description: description, Buttons: true, Options: []terminalui.Option{{Label: "Cancel", Value: "n"}, {Label: action, Value: "y", Destructive: destructive}}})
		if errors.Is(e, terminalui.ErrCancelled) {
			return false, nil
		}
		return value == "y", e
	}
	value, e := p.ReadLine(prompt)
	return parseConfirm(value), e
}
func askSecret(p Prompter, current *bool) (bool, error) {
	if rich, ok := p.(interfacePrompter); ok {
		selected := 0
		if current != nil && *current {
			selected = 1
		}
		v, e := rich.Select(terminalui.SelectionSpec{Context: "Credentials", Title: "Field visibility", Description: "Secret fields are masked when browsing.", Selected: selected, Options: []terminalui.Option{{Label: "Visible field", Value: "n"}, {Label: "Secret field", Value: "y"}}})
		return v == "y", e
	}
	value, e := p.ReadLine(secretPrompt(current))
	if e != nil {
		return false, e
	}
	return parseSecret(value, current)
}
func askConflict(p Prompter, path string) (string, error) {
	if rich, ok := p.(interfacePrompter); ok {
		return rich.Select(terminalui.SelectionSpec{Context: "Git conflict", Title: "Choose a value", Description: path, Options: []terminalui.Option{{Label: "Cancel sharing", Value: "3"}, {Label: "Use team value", Value: "1"}, {Label: "Use my value", Value: "2"}}})
	}
	return p.ReadLine(fmt.Sprintf("Conflict at %s: [1] team [2] mine [3] cancel (default): ", path))
}

func (a *App) humanOut() io.Writer  { return terminalui.Output(a.outWriter()) }
func (a *App) humanErr() io.Writer  { return terminalui.Output(a.errWriter()) }
func (a *App) PrintError(err error) { fmt.Fprintf(a.humanErr(), "cred: %s\n", err) }
func terminalInput(in io.Reader) (*os.File, bool) {
	f, ok := in.(*os.File)
	return f, ok && f != nil && term.IsTerminal(int(f.Fd()))
}
