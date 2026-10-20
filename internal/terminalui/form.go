package terminalui

import (
	"errors"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

var ErrCancelled = errors.New("entry cancelled")

type Field struct {
	Label, Value        string
	Sensitive, Required bool
}
type FormSpec struct {
	Context, Title, Description, Submit string
	Fields                              []Field
	Back                                bool
	Validate                            func([]string) error
}
type Form struct {
	Spec                          FormSpec
	Inputs                        []textinput.Model
	Focus, Width, Height, Scroll  int
	Invalid                       int
	Error                         string
	Accepted, Cancelled, WentBack bool
}

func NewForm(spec FormSpec) *Form {
	if spec.Submit == "" {
		spec.Submit = "Continue"
	}
	m := &Form{Spec: spec, Width: 80, Height: 24, Invalid: -1}
	for _, f := range spec.Fields {
		input := textinput.New()
		input.Prompt = ""
		input.CharLimit = 0
		input.SetValue(f.Value)
		if f.Sensitive {
			input.EchoMode = textinput.EchoPassword
			input.EchoCharacter = '•'
		}
		m.Inputs = append(m.Inputs, input)
	}
	m.setFocus(0)
	return m
}
func (m *Form) Init() tea.Cmd { return textinput.Blink }
func (m *Form) Values() []string {
	v := make([]string, len(m.Inputs))
	for i := range m.Inputs {
		v[i] = m.Inputs[i].Value()
	}
	return v
}
func (m *Form) ClearSecrets() {
	for i, f := range m.Spec.Fields {
		if f.Sensitive {
			m.Inputs[i].Reset()
			m.Inputs[i].EchoMode = textinput.EchoPassword
			m.Spec.Fields[i].Value = ""
		}
	}
}
func (m *Form) setFocus(n int) tea.Cmd {
	m.Focus = n
	m.Scroll = 0
	var cmd tea.Cmd
	for i := range m.Inputs {
		m.Inputs[i].Blur()
		if i == n {
			cmd = m.Inputs[i].Focus()
		}
	}
	return cmd
}
func (m *Form) done() bool { return m.Accepted || m.Cancelled || m.WentBack }
func (m *Form) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.done() {
		return m, nil
	}
	switch k := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = max(1, k.Width)
		m.Height = max(1, k.Height)
	case tea.KeyMsg:
		total := len(m.Inputs) + 2
		if m.Spec.Back {
			total++
		}
		switch k.String() {
		case "esc", "ctrl+c", "ctrl+d":
			m.Cancelled = true
			m.ClearSecrets()
			return m, tea.Quit
		case "pgup", "pgdown":
			delta := max(1, m.Height/3)
			if k.String() == "pgup" {
				delta = -delta
			}
			m.Scroll += delta
			_, m.Scroll = m.frame().Render()
			return m, nil
		case "tab", "shift+tab":
			delta := 1
			if k.String() == "shift+tab" {
				delta = -1
			}
			return m, m.setFocus((m.Focus + delta + total) % total)
		case "ctrl+r":
			if m.Focus < len(m.Inputs) && m.Spec.Fields[m.Focus].Sensitive {
				input := &m.Inputs[m.Focus]
				if input.EchoMode == textinput.EchoNormal {
					input.EchoMode = textinput.EchoPassword
				} else {
					input.EchoMode = textinput.EchoNormal
				}
			}
			return m, nil
		case "enter":
			if m.Focus < len(m.Inputs) {
				return m, m.setFocus(m.Focus + 1)
			}
			if m.Focus == len(m.Inputs) {
				m.Invalid = -1
				for i, f := range m.Spec.Fields {
					if f.Required && m.Inputs[i].Value() == "" {
						m.Error = f.Label + " is required."
						m.Invalid = i
						return m, m.setFocus(i)
					}
				}
				if m.Spec.Validate != nil {
					if err := m.Spec.Validate(m.Values()); err != nil {
						m.Error = err.Error()
						return m, nil
					}
				}
				m.Accepted = true
				return m, tea.Quit
			}
			if m.Spec.Back && m.Focus == len(m.Inputs)+1 {
				m.WentBack = true
			} else {
				m.Cancelled = true
			}
			m.ClearSecrets()
			return m, tea.Quit
		}
	}
	if m.Focus < len(m.Inputs) {
		var cmd tea.Cmd
		m.Inputs[m.Focus], cmd = m.Inputs[m.Focus].Update(msg)
		if m.Invalid == m.Focus && m.Inputs[m.Focus].Value() != "" {
			m.Error = ""
			m.Invalid = -1
		}
		return m, cmd
	}
	return m, nil
}
func (m *Form) frame() Frame {
	inner := max(1, min(64, m.Width)-4)
	var body []string
	anchor := 0
	add := func(s string) { body = append(body, strings.Split(ansi.Hardwrap(s, inner, false), "\n")...) }
	if m.Spec.Description != "" {
		add(m.Spec.Description)
		add("")
	}
	for i, f := range m.Spec.Fields {
		if i == m.Focus {
			anchor = len(body)
		}
		add(Input(f.Label, m.Inputs[i], i == m.Focus, m.Invalid == i, inner))
		if f.Sensitive {
			state := "Hidden"
			if m.Inputs[i].EchoMode == textinput.EchoNormal {
				state = "Visible"
			}
			add(Message("  "+state, Muted))
		}
		if m.Error != "" && m.Invalid == i {
			add(Message(m.Error, Danger))
		}
		add("")
	}
	labels := []string{m.Spec.Submit}
	if m.Spec.Back {
		labels = append(labels, "Back")
	}
	labels = append(labels, "Cancel")
	buttons, rows := Actions(labels, m.Focus-len(m.Inputs), inner)
	if m.Focus >= len(m.Inputs) {
		anchor = len(body) + rows[m.Focus-len(m.Inputs)]
	}
	add(buttons)
	if m.Error != "" && m.Invalid < 0 {
		add("")
		add(Message(m.Error, Danger))
	}
	help := []string{"tab/shift+tab move", "enter next", "esc cancel"}
	if m.Focus >= len(m.Inputs) {
		help[1] = "enter choose"
	}
	if m.Focus < len(m.Inputs) && m.Spec.Fields[m.Focus].Sensitive {
		help = append(help, "ctrl+r show/hide")
	}
	return Frame{Header: Header(m.Spec.Context), Title: m.Spec.Title, Body: strings.Join(body, "\n"), Help: help, Width: m.Width, Height: m.Height, MaxWidth: 64, Anchor: anchor, Offset: m.Scroll}
}
func (m *Form) View() string {
	if m.done() {
		return ""
	}
	view, _ := m.frame().Render()
	return view
}
func RunForm(spec FormSpec, in io.Reader, out io.Writer) ([]string, error) {
	m := NewForm(spec)
	defer m.ClearSecrets()
	_, err := tea.NewProgram(m, tea.WithInput(in), tea.WithOutput(out), tea.WithAltScreen()).Run()
	if err != nil {
		return nil, err
	}
	if !m.Accepted {
		return nil, ErrCancelled
	}
	return m.Values(), nil
}

type Option struct {
	Label, Value string
	Destructive  bool
}
type SelectionSpec struct {
	Context, Title, Description string
	Options                     []Option
	Selected                    int
	Buttons                     bool
}
type Selection struct {
	Spec                         SelectionSpec
	Index, Width, Height, Scroll int
	Accepted, Cancelled          bool
}

func NewSelection(spec SelectionSpec) *Selection {
	index := min(max(0, spec.Selected), max(0, len(spec.Options)-1))
	return &Selection{Spec: spec, Index: index, Width: 80, Height: 24}
}
func (m *Selection) Init() tea.Cmd { return nil }
func (m *Selection) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.Accepted || m.Cancelled {
		return m, nil
	}
	switch k := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = max(1, k.Width)
		m.Height = max(1, k.Height)
	case tea.KeyMsg:
		n := len(m.Spec.Options)
		if n == 0 {
			m.Cancelled = true
			return m, tea.Quit
		}
		switch k.String() {
		case "esc", "ctrl+c":
			m.Cancelled = true
			return m, tea.Quit
		case "up", "k", "left", "shift+tab":
			m.Index = (m.Index + n - 1) % n
			m.Scroll = 0
		case "down", "j", "right", "tab":
			m.Index = (m.Index + 1) % n
			m.Scroll = 0
		case "pgup", "pgdown":
			delta := max(1, m.Height/3)
			if k.String() == "pgup" {
				delta = -delta
			}
			m.Scroll += delta
			_, m.Scroll = m.frame().Render()
		case "enter":
			m.Accepted = true
			return m, tea.Quit
		}
	}
	return m, nil
}
func (m *Selection) frame() Frame {
	width := max(1, min(64, m.Width)-4)
	var body []string
	if m.Spec.Description != "" {
		body = append(body, strings.Split(Help([]string{m.Spec.Description}, width), "\n")...)
		body = append(body, "")
	}
	anchor := len(body)
	destructive := false
	for _, option := range m.Spec.Options {
		destructive = destructive || option.Destructive
	}
	if m.Spec.Buttons && len(m.Spec.Options) == 2 {
		buttons, row := Buttons(m.Spec.Options[0].Label, m.Spec.Options[1].Label, m.Index == 0, m.Index == 1, m.Spec.Options[1].Destructive, false, width)
		if m.Index == 1 {
			anchor += row
		}
		body = append(body, strings.Split(buttons, "\n")...)
	} else {
		for i, o := range m.Spec.Options {
			if i == m.Index {
				anchor = len(body)
			}
			body = append(body, strings.Split(Help([]string{Choice(o.Label, i == m.Index, o.Destructive, false)}, width), "\n")...)
		}
	}
	return Frame{Header: Header(m.Spec.Context), Title: m.Spec.Title, Body: strings.Join(body, "\n"), Help: []string{"↑/↓ or tab move", "enter choose", "esc cancel"}, Width: m.Width, Height: m.Height, MaxWidth: 64, Anchor: anchor, Offset: m.Scroll, Destructive: destructive}
}
func (m *Selection) View() string {
	if m.Accepted || m.Cancelled {
		return ""
	}
	view, _ := m.frame().Render()
	return view
}
func RunSelection(spec SelectionSpec, in io.Reader, out io.Writer) (string, error) {
	m := NewSelection(spec)
	_, err := tea.NewProgram(m, tea.WithInput(in), tea.WithOutput(out), tea.WithAltScreen()).Run()
	if err != nil {
		return "", err
	}
	if !m.Accepted {
		return "", ErrCancelled
	}
	return spec.Options[m.Index].Value, nil
}
