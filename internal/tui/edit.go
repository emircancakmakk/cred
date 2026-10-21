package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/emircancakmakk/cred/internal/password"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type Kind string
type Action string

const (
	ProjectKind Kind   = "project"
	ServiceKind Kind   = "service"
	FieldKind   Kind   = "field"
	Create      Action = "create"
	Edit        Action = "edit"
	Delete      Action = "delete"
)

// Change identifies an exact target and the replacement form values.
type Change struct {
	Action                          Action
	Kind                            Kind
	Project, Service, Name, NewName string
	Value                           string
	Secret                          bool
}

// Commit returns the persisted snapshot, including any merged remote changes.
type Commit func(Change) ([]Project, error)

type editor struct {
	change       Change
	name, value  textinput.Model
	initialValue string
	focus        int
	secret       bool
	err          string
	nameErr      string
	generateErr  string
	generate     func() (string, error)
}

type deletion struct {
	change  Change
	summary string
	confirm bool
	err     string
}

type saved struct {
	projects []Project
	change   Change
	err      error
}

// Saving can invoke existing Git conflict prompts. Exec releases Bubble Tea's
// terminal/input ownership for that operation and restores the browser afterward.
type saveCommand struct {
	commit   Commit
	change   Change
	projects []Project
}

func (c *saveCommand) Run() error {
	var err error
	c.projects, err = c.commit(c.change)
	return err
}
func (*saveCommand) SetStdin(io.Reader)  {}
func (*saveCommand) SetStdout(io.Writer) {}
func (*saveCommand) SetStderr(io.Writer) {}

func (m model) target() (Change, bool) {
	c := Change{}
	switch m.mode {
	case screenProjects:
		c.Kind = ProjectKind
		item, ok := m.projectList.SelectedItem().(projectItem)
		if ok {
			c.Name = item.name
		}
	case screenServices:
		c.Kind = ServiceKind
		c.Project = m.projects[m.projectIx].Name
		item, ok := m.serviceList.SelectedItem().(serviceItem)
		if ok {
			c.Name = item.name
		}
	case screenFields:
		c.Kind = FieldKind
		c.Project = m.projects[m.projectIx].Name
		service, ok := m.currentService()
		if !ok {
			return c, false
		}
		c.Service = service.Name
		item, ok := m.fieldList.SelectedItem().(fieldItem)
		if ok {
			c.Name, c.Value, c.Secret = item.name, item.value, item.secret
		}
	}
	return c, c.Name != ""
}

func (m model) startEditor(edit bool) (model, tea.Cmd) {
	if m.commit == nil {
		m.status = "editing unavailable"
		return m, nil
	}
	c, selected := m.target()
	if edit && !selected {
		return m, nil
	}
	c.Action = Create
	name := textinput.New()
	name.Prompt = ""
	// Names and values are not silently truncated at the widget's default limit.
	name.CharLimit = 0
	value := textinput.New()
	value.Prompt = ""
	value.CharLimit = 0
	e := &editor{change: c, name: name, value: value, secret: true, generate: password.Generate}
	if edit {
		e.change.Action = Edit
		e.name.SetValue(c.Name)
		e.value.SetValue(c.Value)
		e.secret = c.Secret
	}
	e.initialValue = e.value.Value()
	e.value.EchoMode = textinput.EchoPassword
	e.value.EchoCharacter = '•'
	if !e.secret {
		e.value.EchoMode = textinput.EchoNormal
	}
	m.clearReveal()
	m.status = ""
	m.dialogScroll = 0
	m.editor = e
	return m, m.editor.name.Focus()
}

func (m model) startDelete() model {
	if m.commit == nil {
		m.status = "editing unavailable"
		return m
	}
	c, ok := m.target()
	if !ok {
		return m
	}
	c.Action = Delete
	summary := ""
	switch c.Kind {
	case ProjectKind:
		item := m.projectList.SelectedItem().(projectItem)
		project := m.projects[item.index]
		fields := 0
		for _, s := range project.Services {
			fields += len(s.Fields)
		}
		summary = fmt.Sprintf("Also deletes %d services and %d fields.", len(project.Services), fields)
	case ServiceKind:
		item := m.serviceList.SelectedItem().(serviceItem)
		summary = fmt.Sprintf("Also deletes %d fields.", item.fields)
	}
	m.clearReveal()
	m.status = ""
	m.dialogScroll = 0
	m.deleting = &deletion{change: c, summary: summary}
	return m
}

func (m model) updateDialog(msg tea.Msg) (model, tea.Cmd) {
	if m.saving {
		return m, nil
	}
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "pgup":
			m.dialogScroll -= max(1, m.height/3)
			_, m.dialogScroll = m.renderDialog()
			return m, nil
		case "pgdown":
			m.dialogScroll += max(1, m.height/3)
			_, m.dialogScroll = m.renderDialog()
			return m, nil
		}
	}
	if m.deleting != nil {
		if k, ok := msg.(tea.KeyMsg); ok {
			switch k.String() {
			case "esc", "n":
				m.deleting = nil
			case "tab", "shift+tab", "left", "right":
				m.deleting.confirm = !m.deleting.confirm
				m.dialogScroll = 0
			case "enter":
				if !m.deleting.confirm {
					m.deleting = nil
					return m, nil
				}
				return m.saveChange(m.deleting.change)
			}
		}
		return m, nil
	}
	e := m.editor
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc":
			m.editor = nil
			return m, nil
		case "enter":
			if e.change.Kind == FieldKind && e.focus == 3 {
				m.dialogScroll = 0
				return m, e.generatePassword()
			}
			if e.focus < e.saveFocus() {
				m.dialogScroll = 0
				return m, e.setFocus(e.focus + 1)
			}
			if e.focus == e.saveFocus()+1 {
				m.editor = nil
				return m, nil
			}
			c := e.change
			c.NewName, c.Value, c.Secret = e.name.Value(), e.value.Value(), e.secret
			// The single-line widget sanitizes imported multiline/control content.
			// A name/flag-only edit must preserve the original value byte for byte.
			if c.Action == Edit && e.value.Value() == e.initialValue {
				c.Value = e.change.Value
			}
			if strings.TrimSpace(c.NewName) == "" {
				e.nameErr = "Name is required."
				m.dialogScroll = 0
				return m, e.setFocus(0)
			}
			return m.saveChange(c)
		case "tab", "shift+tab":
			delta := 1
			if k.String() == "shift+tab" {
				delta = -1
			}
			m.dialogScroll = 0
			total := e.saveFocus() + 2
			return m, e.setFocus((e.focus + delta + total) % total)
		case "ctrl+g":
			if e.change.Kind == FieldKind {
				m.dialogScroll = 0
				return m, e.generatePassword()
			}
			return m, nil
		case "ctrl+r":
			if e.change.Kind == FieldKind && e.focus == 1 && e.secret {
				if e.value.EchoMode == textinput.EchoPassword {
					e.value.EchoMode = textinput.EchoNormal
				} else {
					e.value.EchoMode = textinput.EchoPassword
				}
			}
			return m, nil
		case " ":
			if e.change.Kind == FieldKind && e.focus == 2 {
				e.secret = !e.secret
				e.value.EchoMode = textinput.EchoPassword
				if !e.secret {
					e.value.EchoMode = textinput.EchoNormal
				}
				return m, nil
			}
		}
	}
	var cmd tea.Cmd
	switch e.focus {
	case 0:
		e.name, cmd = e.name.Update(msg)
		if strings.TrimSpace(e.name.Value()) != "" {
			e.nameErr = ""
		}
	case 1:
		if e.change.Kind == FieldKind {
			e.value, cmd = e.value.Update(msg)
		}
	}
	return m, cmd
}

func (m model) saveChange(c Change) (model, tea.Cmd) {
	m.saving = true
	m.dialogScroll = 0
	job := &saveCommand{commit: m.commit, change: c}
	return m, tea.Exec(job, func(err error) tea.Msg {
		return saved{projects: job.projects, change: c, err: err}
	})
}

func (m model) finishSave(result saved) model {
	m.saving = false
	m.dialogScroll = 0
	if result.err != nil {
		if m.editor != nil {
			m.editor.err = result.err.Error()
		}
		if m.deleting != nil {
			m.deleting.err = result.err.Error()
		}
		return m
	}
	c := result.change
	// Restore the same list by names: merged snapshots may change sorted indices.
	next, _ := newModel(result.projects, "", m.copier)
	next.commit, next.vaultLabel = m.commit, m.vaultLabel
	next.reveal.id = m.reveal.id
	next = next.resize(m.width, m.height)
	next.lastCopied = m.lastCopied
	if c.Kind != ProjectKind {
		for i, p := range next.projects {
			if p.Name != c.Project {
				continue
			}
			next = next.openProject(i)
			if c.Kind == FieldKind {
				for j, s := range p.Services {
					if s.Name == c.Service {
						next = next.openService(j)
						break
					}
				}
			}
			break
		}
	}
	index := m.active().Index()
	name := c.NewName
	if c.Action == Delete {
		name = ""
	}
	items := next.active().Items()
	for i, item := range items {
		if row, ok := item.(interface{ Title() string }); ok && row.Title() == name {
			index = i
			break
		}
	}
	if index >= len(items) {
		index = len(items) - 1
	}
	if index < 0 {
		index = 0
	}
	switch next.mode {
	case screenProjects:
		next.projectList.Select(index)
	case screenServices:
		next.serviceList.Select(index)
	case screenFields:
		next.fieldList.Select(index)
	}
	next.status = map[Action]string{Create: "created", Edit: "updated", Delete: "deleted"}[c.Action] + " "
	if c.Action == Delete {
		next.status += c.Name
	} else {
		next.status += c.NewName
	}
	return next
}

// The action buttons follow the available inputs in the keyboard focus order.
func (e *editor) saveFocus() int {
	if e.change.Kind == FieldKind {
		return 4
	}
	return 1
}

func (e *editor) generatePassword() tea.Cmd {
	value, err := e.generate()
	if err != nil {
		// Do not include backend errors, which may contain sensitive data.
		e.generateErr = "Could not generate password. Try again."
		return nil
	}
	e.value.SetValue(value)
	e.secret = true
	e.value.EchoMode = textinput.EchoPassword
	e.err = ""
	e.generateErr = ""
	return e.setFocus(1)
}

func (e *editor) setFocus(focus int) tea.Cmd {
	e.focus = focus
	e.name.Blur()
	e.value.Blur()
	switch focus {
	case 0:
		return e.name.Focus()
	case 1:
		if e.change.Kind == FieldKind {
			return e.value.Focus()
		}
	}
	return nil
}
