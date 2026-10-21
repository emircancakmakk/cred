package tui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/emircancakmakk/cred/internal/terminalui"
	"github.com/emircancakmakk/cred/internal/vault"
)

type emailItem struct{ SavedEmail }

func (e emailItem) Title() string       { return e.Address }
func (e emailItem) Meta() string        { return "" }
func (e emailItem) FilterValue() string { return e.Address }

type emailManager struct {
	list        list.Model
	rows        *rowDelegate
	bulk        *emailBulk
	single      *terminalui.Form
	editingID   string
	deleting    *SavedEmail
	confirm     bool
	err, status string
	scroll      int
}
type emailBulk struct {
	input  textarea.Model
	review bool
	focus  int
	batch  vault.EmailBatch
}
type emailPicker struct {
	list          list.Model
	rows          *rowDelegate
	previousFocus int
}

func emailList(emails []SavedEmail) (list.Model, *rowDelegate) {
	emails = append([]SavedEmail(nil), emails...)
	sort.Slice(emails, func(i, j int) bool { return emails[i].Address < emails[j].Address })
	items := make([]list.Item, len(emails))
	for i, email := range emails {
		items[i] = emailItem{email}
	}
	rows := &rowDelegate{width: 60}
	return newBrowserList("email", "emails", rows, items), rows
}
func newEmailManager(emails []SavedEmail) *emailManager {
	l, rows := emailList(emails)
	return &emailManager{list: l, rows: rows}
}
func (m model) updateOverlay(msg tea.Msg) (model, tea.Cmd) {
	if m.manager != nil {
		return m.updateEmailManager(msg)
	}
	return m.updateDialog(msg)
}
func emailAddresses(emails []SavedEmail) []string {
	addresses := make([]string, 0, len(emails))
	for _, email := range emails {
		addresses = append(addresses, email.Address)
	}
	return addresses
}
func (m model) updateEmailManager(msg tea.Msg) (model, tea.Cmd) {
	g := m.manager
	if m.saving {
		return m, nil
	}
	if g.single != nil {
		g.single.Width, g.single.Height = m.width, m.height
		_, cmd := g.single.Update(msg)
		if g.single.Cancelled {
			g.single = nil
			g.err = ""
			return m, nil
		}
		if g.single.Accepted {
			// Completion commands from the standalone form must not quit this browser.
			return m.saveChange(Change{Kind: EmailKind, Action: Edit, Name: g.editingID, Value: g.single.Values()[0]})
		}
		return m, cmd
	}
	if g.bulk != nil {
		return m.updateEmailBulk(msg)
	}
	if g.deleting != nil {
		if k, ok := msg.(tea.KeyMsg); ok {
			switch k.String() {
			case "esc", "n":
				g.deleting = nil
				g.err = ""
			case "pgup", "pgdown":
				step := max(1, m.height/3)
				if k.String() == "pgup" {
					step = -step
				}
				g.scroll += step
			case "tab", "shift+tab", "left", "right":
				g.scroll = 0
				g.confirm = !g.confirm
			case "enter":
				if !g.confirm {
					g.deleting = nil
					g.err = ""
					return m, nil
				}
				return m.saveChange(Change{Kind: EmailKind, Action: Delete, Name: g.deleting.ID})
			}
		}
		return m, nil
	}
	if g.list.SettingFilter() {
		var cmd tea.Cmd
		g.list, cmd = g.list.Update(msg)
		return m, cmd
	}
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc", "left":
			m.manager = nil
			return m, nil
		case "q":
			return m, tea.Quit
		case "n":
			input := textarea.New()
			input.CharLimit = 0
			input.MaxHeight = 0
			input.ShowLineNumbers = false
			input.Prompt = ""
			input.SetHeight(5)
			g.bulk = &emailBulk{input: input}
			g.err, g.status, g.scroll = "", "", 0
			return m, g.bulk.input.Focus()
		case "e":
			if selected, ok := g.list.SelectedItem().(emailItem); ok {
				g.editingID = selected.ID
				g.err, g.status, g.scroll = "", "", 0
				g.single = terminalui.NewForm(terminalui.FormSpec{Context: strings.TrimSpace(m.vaultLabel + "  Emails"), Title: "Edit saved email", Description: "Existing credential fields keep their copied values.", Submit: "Save", Fields: []terminalui.Field{{Label: "Address", Value: selected.Address, Required: true}}, Validate: func(values []string) error {
					address, err := vault.NormalizeEmail(values[0])
					if err != nil {
						return err
					}
					for _, email := range m.emails {
						if email.ID != selected.ID && strings.EqualFold(email.Address, address) {
							return vault.ErrEmailExists
						}
					}
					return nil
				}})
				return m, g.single.Init()
			}
			return m, nil
		case "d":
			if selected, ok := g.list.SelectedItem().(emailItem); ok {
				email := selected.SavedEmail
				g.deleting = &email
				g.confirm = false
				g.err, g.status, g.scroll = "", "", 0
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	g.list, cmd = g.list.Update(msg)
	return m, cmd
}

func (m model) updateEmailBulk(msg tea.Msg) (model, tea.Cmd) {
	g, b := m.manager, m.manager.bulk
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc":
			g.bulk = nil
			g.err = ""
			g.scroll = 0
			return m, nil
		case "pgup", "pgdown":
			if b.review || b.focus != 0 {
				step := max(1, m.height/3)
				if k.String() == "pgup" {
					step = -step
				}
				g.scroll += step
				_ = m.emailManagerView()
				return m, nil
			}
		case "tab", "shift+tab":
			delta := 1
			if k.String() == "shift+tab" {
				delta = -1
			}
			b.focus = (b.focus + delta + 3) % 3
			g.scroll = 0
			b.input.Blur()
			if !b.review && b.focus == 0 {
				return m, b.input.Focus()
			}
			return m, nil
		case "enter":
			if !b.review {
				switch b.focus {
				case 1:
					b.batch = vault.PreviewEmails(b.input.Value(), emailAddresses(m.emails))
					b.review, b.focus = true, 0
					b.input.Blur()
					g.scroll, g.err = 0, ""
					return m, nil
				case 2:
					g.bulk = nil
					g.err = ""
					return m, nil
				}
			} else {
				switch b.focus {
				case 0:
					if len(b.batch.Invalid) > 0 {
						g.err = "Fix invalid entries before saving. Choose Back to edit."
						return m, nil
					}
					if len(b.batch.Added) == 0 {
						g.err = "No new addresses to save. Choose Back to edit or Cancel."
						return m, nil
					}
					return m.saveChange(Change{Kind: EmailKind, Action: Create, Value: b.input.Value()})
				case 1:
					b.review, b.focus = false, 0
					g.err, g.scroll = "", 0
					return m, b.input.Focus()
				case 2:
					g.bulk = nil
					g.err, g.scroll = "", 0
					return m, nil
				}
			}
		}
	}
	if !b.review && b.focus == 0 {
		previous := b.input.Value()
		var cmd tea.Cmd
		b.input, cmd = b.input.Update(msg)
		// The widget silently truncates pasted input at 10,000 lines. Reject
		// that boundary and restore the previous draft instead of losing data.
		if b.input.LineCount() >= 10000 {
			b.input.SetValue(previous)
			g.err = "Batch input supports up to 9,999 lines. Split the paste into smaller batches; your previous draft is retained."
		} else if b.input.Value() != previous {
			g.err = ""
		}
		return m, cmd
	}
	return m, nil
}

func (m model) openEmailPicker() (model, tea.Cmd) {
	e := m.editor
	l, rows := emailList(m.emails)
	e.emailPicker = &emailPicker{list: l, rows: rows, previousFocus: e.focus}
	e.name.Blur()
	e.value.Blur()
	return m, nil
}
func (m model) updateEmailPicker(msg tea.Msg) (model, tea.Cmd) {
	e, p := m.editor, m.editor.emailPicker
	if p.list.SettingFilter() {
		var cmd tea.Cmd
		p.list, cmd = p.list.Update(msg)
		return m, cmd
	}
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc", "left":
			e.emailPicker = nil
			return m, e.setFocus(p.previousFocus)
		case "enter":
			if selected, ok := p.list.SelectedItem().(emailItem); ok {
				e.value.SetValue(selected.Address)
				e.valueReplaced = true
				e.emailPicker = nil
				return m, e.setFocus(1)
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return m, cmd
}

// Rebuild from the actual persisted snapshot, preserving browser context by name.
func (m model) finishEmailSave(result saved) model {
	g := m.manager
	if result.err != nil {
		g.err = result.err.Error()
		if g.single != nil {
			g.single.Accepted = false
			g.single.Error = g.err
		}
		return m
	}
	project, service, field := "", "", ""
	if m.mode != screenProjects && m.projectIx >= 0 && m.projectIx < len(m.projects) {
		project = m.projects[m.projectIx].Name
	}
	if m.mode == screenFields {
		if s, ok := m.currentService(); ok {
			service = s.Name
		}
	}
	if selected := m.active().SelectedItem(); selected != nil {
		if row, ok := selected.(interface{ Title() string }); ok {
			field = row.Title()
		}
	}
	next, _ := newModel(result.snapshot.Projects, "", m.copier)
	next.commit, next.vaultLabel, next.emails, next.lastCopied = m.commit, m.vaultLabel, result.snapshot.Emails, m.lastCopied
	next = next.resize(m.width, m.height)
	for i, p := range next.projects {
		if p.Name == project {
			next = next.openProject(i)
			for j, s := range p.Services {
				if s.Name == service {
					next = next.openService(j)
					break
				}
			}
			break
		}
	}
	for i, item := range next.active().Items() {
		if row, ok := item.(interface{ Title() string }); ok && row.Title() == field {
			switch next.mode {
			case screenProjects:
				next.projectList.Select(i)
			case screenServices:
				next.serviceList.Select(i)
			case screenFields:
				next.fieldList.Select(i)
			}
			break
		}
	}
	next.manager = newEmailManager(next.emails)
	index := g.list.Index()
	if result.change.Action == Edit {
		for i, item := range next.manager.list.Items() {
			if item.(emailItem).ID == result.change.Name {
				index = i
				break
			}
		}
	}
	next.manager.list.Select(max(0, min(index, len(next.emails)-1)))
	next.manager.status = "Saved email addresses updated."
	return next
}
