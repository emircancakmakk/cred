package cmd

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/emircancakmakk/cred/internal/crypto"
	"github.com/emircancakmakk/cred/internal/profiles"
	"github.com/emircancakmakk/cred/internal/terminalui"
	"github.com/emircancakmakk/cred/internal/vault"
)

type setupRequest struct {
	Choice           setupChoice
	Fixed            bool
	Name, Repo, From string
}
type wizardProgress string
type wizardResult struct {
	profile     profiles.Profile
	err         error
	defaultOnly bool
}
type wizardAnswer struct {
	values []string
	choice string
	err    error
}
type wizardQuestion struct {
	form      *terminalui.FormSpec
	selection *terminalui.SelectionSpec
	reply     chan wizardAnswer
}

type wizardPrompter struct {
	events    chan tea.Msg
	passwords [][]byte
}

func (p *wizardPrompter) Form(spec terminalui.FormSpec) ([]string, error) {
	reply := make(chan wizardAnswer, 1)
	p.events <- wizardQuestion{form: &spec, reply: reply}
	a := <-reply
	return a.values, a.err
}
func (p *wizardPrompter) Select(spec terminalui.SelectionSpec) (string, error) {
	reply := make(chan wizardAnswer, 1)
	p.events <- wizardQuestion{selection: &spec, reply: reply}
	a := <-reply
	return a.choice, a.err
}
func (p *wizardPrompter) ReadPassword(prompt string) ([]byte, error) {
	if len(p.passwords) > 0 {
		answer := p.passwords[0]
		p.passwords = p.passwords[1:]
		return answer, nil
	}
	values, err := p.Form(passwordFormSpec(prompt, "Setup"))
	if err != nil {
		return nil, err
	}
	return []byte(values[0]), nil
}
func (p *wizardPrompter) ReadLine(prompt string) (string, error) {
	v, e := p.Form(terminalui.FormSpec{Context: "Setup", Title: "Continue setup", Description: prompt, Fields: []terminalui.Field{{Label: "Answer"}}})
	if e != nil {
		return "", e
	}
	return v[0], nil
}

type setupWizard struct {
	app                   *App
	request               setupRequest
	name, repo            string
	step, width, height   int
	form                  *terminalui.Form
	selection             *terminalui.Selection
	password              []byte
	events                chan tea.Msg
	question              *wizardQuestion
	busy, cancelled, done bool
	status                string
	profile               profiles.Profile
	err                   error
}

func newSetupWizard(a *App, r setupRequest) *setupWizard {
	m := &setupWizard{app: a, request: r, name: r.Name, repo: r.Repo, width: 80, height: 24, events: make(chan tea.Msg, 32)}
	if r.Fixed {
		m.details()
	} else {
		m.choose()
	}
	return m
}
func (m *setupWizard) Init() tea.Cmd { return textBlink() }
func textBlink() tea.Cmd             { return textinput.Blink }
func (m *setupWizard) size() {
	if m.form != nil {
		m.form.Width, m.form.Height = m.width, m.height
	}
	if m.selection != nil {
		m.selection.Width, m.selection.Height = m.width, m.height
	}
}
func (m *setupWizard) choose() {
	m.step = 0
	m.form = nil
	options := []terminalui.Option{}
	for i, label := range setupLabels {
		options = append(options, terminalui.Option{Label: label, Value: fmt.Sprint(i)})
	}
	m.selection = terminalui.NewSelection(terminalui.SelectionSpec{Context: "Setup", Title: "Choose a vault", Options: options, Selected: int(m.request.Choice)})
	m.size()
}
func (m *setupWizard) details() {
	m.step = 1
	m.selection = nil
	fields := []terminalui.Field{}
	if m.request.Name == "" {
		name := m.name
		if name == "" && m.request.Choice == setupLocal {
			name = "local"
		}
		fields = append(fields, terminalui.Field{Label: "Profile name", Value: name, Required: true})
	}
	if m.request.Choice != setupLocal && m.request.Repo == "" {
		fields = append(fields, terminalui.Field{Label: "Git repository URL", Value: m.repo, Required: true})
	}
	if len(fields) == 0 {
		m.credentials()
		return
	}
	m.form = terminalui.NewForm(terminalui.FormSpec{Context: "Setup", Title: "Vault details", Fields: fields, Back: !m.request.Fixed, Validate: func(values []string) error {
		index := 0
		if m.request.Name == "" {
			if err := profiles.ValidateName(strings.TrimSpace(values[0])); err != nil {
				return err
			}
			index++
		}
		if m.request.Choice != setupLocal && m.request.Repo == "" && strings.TrimSpace(values[index]) == "" {
			return errors.New("Git repository URL is required.")
		}
		return nil
	}})
	m.size()
}
func (m *setupWizard) credentials() {
	m.step = 2
	m.selection = nil
	crypto.Wipe(m.password)
	m.password = nil
	fields := []terminalui.Field{{Label: "Master password", Sensitive: true, Required: true}}
	title := "Create a password"
	if m.request.Choice == setupConnect {
		title = "Open vault"
	} else {
		fields = append(fields, terminalui.Field{Label: "Repeat master password", Sensitive: true, Required: true})
	}
	m.form = terminalui.NewForm(terminalui.FormSpec{Context: "Setup", Title: title, Description: "Passwords start hidden. Ctrl+R shows or hides the focused field.", Fields: fields, Back: !m.request.Fixed || m.request.Name == "" || m.request.Repo == "" && m.request.Choice != setupLocal, Validate: func(v []string) error {
		if m.request.Choice == setupConnect {
			return nil
		}
		first, second := []byte(v[0]), []byte(v[1])
		defer crypto.Wipe(first)
		defer crypto.Wipe(second)
		if err := vault.ValidateMasterPassword(first); err != nil {
			return err
		}
		if subtle.ConstantTimeCompare(first, second) != 1 {
			return errors.New("Passwords do not match.")
		}
		return nil
	}})
	m.size()
}
func (m *setupWizard) review() {
	m.step = 3
	m.form = terminalui.NewForm(terminalui.FormSpec{Context: "Setup", Title: "Review vault", Description: m.summary() + "\n\nThe password is ready and hidden.", Submit: m.action(), Back: true})
	m.size()
}
func (m *setupWizard) action() string {
	if m.request.Choice == setupConnect {
		return "Connect"
	}
	return "Create"
}
func (m *setupWizard) summary() string {
	body := "Profile: " + m.name + "\n" + setupLabels[m.request.Choice]
	if m.repo != "" && m.request.Choice != setupLocal {
		body += "\nRepository: " + m.repo
	}
	if m.request.From != "" {
		body += "\nSource: " + m.request.From
	}
	return body
}
func (m *setupWizard) wait() tea.Msg { return <-m.events }
func (m *setupWizard) start() tea.Cmd {
	m.step = 4
	m.busy = true
	m.form = nil
	m.selection = nil
	m.status = "Preparing vault"
	password := m.password
	m.password = nil
	// The worker owns the collected password and the existing service lifecycle.
	return func() tea.Msg {
		go func() {
			defer crypto.Wipe(password)
			worker := *m.app
			worker.Out, worker.Err = io.Discard, io.Discard
			prompts := &wizardPrompter{events: m.events, passwords: [][]byte{append([]byte(nil), password...)}}
			if m.request.Choice != setupConnect {
				prompts.passwords = append(prompts.passwords, append([]byte(nil), password...))
			}
			defer func() {
				for _, p := range prompts.passwords {
					crypto.Wipe(p)
				}
			}()
			worker.Prompter = prompts
			worker.progressObserver = func(label string) func() { m.events <- wizardProgress(label); return func() {} }
			var p profiles.Profile
			var err error
			switch m.request.Choice {
			case setupLocal:
				p, err = worker.createLocal(m.name)
			case setupConnect:
				p, err = worker.connectProfile(m.name, m.repo, m.request.From)
			case setupShared:
				p, err = worker.createShared(m.name, m.repo)
			}
			m.events <- wizardResult{profile: p, err: err}
		}()
		return m.wait()
	}
}
func (m *setupWizard) result() {
	m.step = 5
	path := m.profile.Path
	if m.profile.Type == "git" {
		path = m.profile.RepoDir() + "/vault.age"
	}
	m.form = nil
	m.selection = terminalui.NewSelection(terminalui.SelectionSpec{Context: "Setup", Title: "Vault ready", Description: m.summary() + "\nVault: " + path + "\nTry: cred --vault " + m.name, Options: []terminalui.Option{{Label: "Finish", Value: "finish"}, {Label: "Use this vault by default", Value: "default"}}})
	m.size()
}
func (m *setupWizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = max(1, v.Width), max(1, v.Height)
		m.size()
		return m, nil
	case wizardProgress:
		m.status = string(v)
		return m, m.wait
	case wizardQuestion:
		if m.cancelled {
			v.reply <- wizardAnswer{err: terminalui.ErrCancelled}
			return m, m.wait
		}
		m.question = &v
		if v.form != nil {
			m.form = terminalui.NewForm(*v.form)
			m.selection = nil
		} else {
			m.selection = terminalui.NewSelection(*v.selection)
			m.form = nil
		}
		m.size()
		return m, textBlink()
	case wizardResult:
		m.busy = false
		m.question = nil
		m.err = v.err
		m.profile = v.profile
		if m.cancelled {
			if v.err == nil {
				m.cancelled = false
			}
			m.done = true
			return m, tea.Quit
		}
		if v.defaultOnly {
			m.result()
			if v.err != nil {
				m.selection.Spec.Description += "\n\nVault is ready; default selection failed: " + v.err.Error()
			} else {
				m.selection.Spec.Description += "\n\nDefault vault: " + m.name
			}
			if v.err == nil && os.Getenv("CRED_VAULT") != "" {
				m.selection.Spec.Description += "\nCRED_VAULT is set; unset it to use the default without --vault."
			}
			m.selection.Spec.Options = m.selection.Spec.Options[:1]
			return m, nil
		}
		if v.err != nil {
			m.step = 6
			m.form = terminalui.NewForm(terminalui.FormSpec{Context: "Setup", Title: "Setup stopped", Description: v.err.Error(), Submit: "Close"})
			m.selection = nil
			m.size()
			return m, nil
		}
		m.result()
		return m, nil
	case tea.KeyMsg:
		if v.String() == "ctrl+c" && m.busy {
			m.cancelled = true
			if m.question != nil {
				if m.form != nil {
					m.form.ClearSecrets()
				}
				m.question.reply <- wizardAnswer{err: terminalui.ErrCancelled}
				m.question = nil
				m.form = nil
				m.selection = nil
				m.status = "Stopping after the current operation"
				return m, m.wait
			}
			m.status = "Stopping after the current operation"
			return m, nil
		}
	}
	if m.busy && m.question == nil {
		return m, nil
	}
	if m.selection != nil {
		_, cmd := m.selection.Update(msg)
		if m.selection.Cancelled {
			if m.question != nil {
				m.question.reply <- wizardAnswer{err: terminalui.ErrCancelled}
				m.question = nil
				m.selection = nil
				return m, m.wait
			}
			m.cancelled = m.step != 5
			m.done = true
			return m, tea.Quit
		}
		if !m.selection.Accepted {
			return m, cmd
		}
		choice := m.selection.Spec.Options[m.selection.Index].Value
		if m.question != nil {
			m.question.reply <- wizardAnswer{choice: choice}
			m.question = nil
			m.selection = nil
			return m, m.wait
		}
		if m.step == 0 {
			m.request.Choice = setupChoice(m.selection.Index)
			m.details()
			return m, textBlink()
		}
		if choice == "default" {
			m.busy = true
			m.selection = nil
			m.status = "Selecting default vault"
			name := m.name
			return m, func() tea.Msg {
				err := setDefaultProfile(name)
				return wizardResult{profile: m.profile, err: err, defaultOnly: true}
			}
		}
		m.done = true
		return m, tea.Quit
	}
	if m.form == nil {
		return m, nil
	}
	_, cmd := m.form.Update(msg)
	if m.question != nil {
		if m.form.Accepted || m.form.Cancelled {
			answer := wizardAnswer{values: m.form.Values()}
			if m.form.Cancelled {
				answer.err = terminalui.ErrCancelled
			}
			m.form.ClearSecrets()
			m.question.reply <- answer
			m.question = nil
			m.form = nil
			return m, m.wait
		}
		return m, cmd
	}
	if m.form.Cancelled {
		crypto.Wipe(m.password)
		m.password = nil
		m.cancelled = m.step != 6
		m.done = true
		return m, tea.Quit
	}
	if m.form.WentBack {
		switch m.step {
		case 1:
			values := m.form.Values()
			index := 0
			if m.request.Name == "" {
				m.name = values[0]
				index++
			}
			if m.request.Choice != setupLocal && m.request.Repo == "" {
				m.repo = values[index]
			}
			m.choose()
		case 2:
			m.details()
		case 3:
			m.credentials()
		}
		return m, textBlink()
	}
	if !m.form.Accepted {
		return m, cmd
	}
	switch m.step {
	case 1:
		v := m.form.Values()
		index := 0
		if m.request.Name == "" {
			m.name = strings.TrimSpace(v[0])
			index++
		}
		if m.request.Choice != setupLocal && m.request.Repo == "" {
			m.repo = strings.TrimSpace(v[index])
		}
		m.credentials()
	case 2:
		m.password = []byte(m.form.Values()[0])
		m.form.ClearSecrets()
		m.review()
	case 3:
		return m, m.start()
	case 6:
		m.done = true
		return m, tea.Quit
	}
	return m, textBlink()
}
func (m *setupWizard) View() string {
	if m.done {
		return ""
	}
	if m.form != nil {
		return m.form.View()
	}
	if m.selection != nil {
		return m.selection.View()
	}
	view, _ := (terminalui.Frame{Header: terminalui.Header("Setup"), Title: m.action() + " vault", Body: terminalui.Message("… "+m.status, terminalui.Accent), Help: []string{"ctrl+c stop after current operation"}, Width: m.width, Height: m.height}).Render()
	return view
}
func (a *App) runSetupWizard(request setupRequest) error {
	in, ok := terminalInput(a.In)
	if !ok {
		return errors.New("interactive terminal required")
	}
	m := newSetupWizard(a, request)
	defer func() {
		crypto.Wipe(m.password)
		if m.form != nil {
			m.form.ClearSecrets()
		}
	}()
	_, err := tea.NewProgram(m, tea.WithInput(in), tea.WithOutput(a.errWriter()), tea.WithAltScreen()).Run()
	if err != nil {
		return err
	}
	if m.cancelled {
		fmt.Fprintln(a.humanErr(), "cancelled")
		return nil
	}
	if m.err != nil {
		return m.err
	}
	fmt.Fprintf(a.humanErr(), "Ready: %s\nTry: cred --vault %s\n", m.name, m.name)
	return nil
}
