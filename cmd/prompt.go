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

const (
	promptPassword = "Master password: "
	promptRepeat   = "Repeat master password: "
	promptService  = "Service: "
	promptField    = "Field name (empty to finish): "
	promptValue    = "Value: "
)

type Prompter interface {
	ReadPassword(prompt string) ([]byte, error)
	ReadLine(prompt string) (string, error)
}

type terminalPrompter struct {
	in  *os.File
	out io.Writer
}

func newTerminalPrompter(in *os.File, out io.Writer) *terminalPrompter {
	return &terminalPrompter{in: in, out: out}
}

func (p *terminalPrompter) ReadPassword(prompt string) ([]byte, error) {
	if !term.IsTerminal(int(p.in.Fd())) {
		return nil, errors.New("interactive terminal required")
	}

	return readPasswordScreen(prompt, p.in, p.out)
}

func (p *terminalPrompter) ReadLine(prompt string) (string, error) {
	if !term.IsTerminal(int(p.in.Fd())) {
		return "", errors.New("interactive terminal required")
	}
	values, err := p.Form(terminalui.FormSpec{Context: "Input", Title: prompt, Fields: []terminalui.Field{{Label: prompt}}})
	if err != nil {
		return "", err
	}
	return values[0], nil
}

func readLine(in *os.File) (string, error) {
	var buf []byte
	one := make([]byte, 1)
	for {
		n, err := in.Read(one)
		if n == 1 {
			if one[0] == '\n' {
				return string(buf), nil
			}
			if one[0] != '\r' {
				buf = append(buf, one[0])
			}
			continue
		}
		if errors.Is(err, io.EOF) {
			if len(buf) == 0 {
				return "", io.EOF
			}
			return string(buf), nil
		}
		if err != nil {
			return "", fmt.Errorf("read input: %w", err)
		}
		return "", errors.New("read input: short read")
	}
}

func parseConfirm(answer string) bool {
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}

func parseSecret(answer string, current *bool) (bool, error) {
	if strings.TrimSpace(answer) == "" {
		if current != nil {
			return *current, nil
		}
		return false, nil
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		return false, errors.New("answer y or n")
	}
}

func secretPrompt(current *bool) string {
	if current != nil && *current {
		return "Secret? [Y/n]: "
	}
	return "Secret? [y/N]: "
}
