package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/emircancakmakk/cred/internal/terminalui"
)

// Size lists to the frame's usable body so pagination keeps the selected row visible.
func (m model) fitEmailList(l *list.Model, rows *rowDelegate, help []string, extra string) {
	inner := max(1, min(64, m.width)-4)
	rows.width = inner
	extraHeight := 0
	if extra != "" {
		extraHeight = lipgloss.Height(ansi.Hardwrap(extra, inner, false))
	}
	room := m.height - 8 - lipgloss.Height(terminalui.Help(help, inner)) - extraHeight
	l.SetSize(inner, max(1, min(len(l.VisibleItems()), room)))
}

func (m model) emailManagerView() string {
	g := m.manager
	if g.single != nil {
		g.single.Width, g.single.Height = m.width, m.height
		form := *g.single
		if m.saving {
			form.Spec.Title = "Saving…"
		}
		return form.View()
	}
	inner := max(1, min(64, m.width)-4)
	title := "Saved emails"
	var body []string
	add := func(s string) { body = append(body, strings.Split(ansi.Hardwrap(s, inner, false), "\n")...) }
	anchor := 0
	help := []string{"j/k move", "/ filter", "n add batch", "e edit", "d delete", "esc back", "q quit"}
	if b := g.bulk; b != nil {
		title = "Add email addresses"
		if !b.review {
			b.input.SetWidth(inner)
			b.input.SetHeight(max(1, min(5, m.height-14)))
			add("One address per line, or separate with commas or semicolons.")
			add("Example: personal@example.com; work@example.com")
			add("")
			add("Addresses")
			anchor = len(body)
			add(b.input.View())
			add("")
			if b.focus == 1 {
				anchor = len(body)
			}
			add(dialogChoice("[ Review ]", b.focus == 1, false, m.saving))
			if b.focus == 2 {
				anchor = len(body)
			}
			add(dialogChoice("[ Cancel ]", b.focus == 2, false, m.saving))
			help = []string{"enter newline", "tab/shift+tab move", "esc cancel"}
			if b.focus > 0 {
				help[0] = "enter choose"
			}
		} else {
			title = "Review email addresses"
			add(fmt.Sprintf("%d to add, %d duplicates skipped, %d invalid.", len(b.batch.Added), len(b.batch.Skipped), len(b.batch.Invalid)))
			for _, section := range []struct {
				label  string
				values []string
			}{{"Add", b.batch.Added}, {"Skip duplicate", b.batch.Skipped}, {"Invalid", b.batch.Invalid}} {
				for _, value := range section.values {
					add(fmt.Sprintf("%s: %q", section.label, value))
				}
			}
			if len(b.batch.Added) == 0 && len(b.batch.Invalid) == 0 {
				add("No new addresses. Choose Back or Cancel.")
			}
			if len(b.batch.Invalid) > 0 {
				add("Use plain addresses such as name@example.com, without names, quotes or spaces. Choose Back to fix invalid entries.")
			}
			add("")
			for i, label := range []string{"Save", "Back", "Cancel"} {
				if b.focus == i {
					anchor = len(body)
				}
				disabled := m.saving || (i == 0 && (len(b.batch.Invalid) > 0 || len(b.batch.Added) == 0))
				add(dialogChoice("[ "+label+" ]", b.focus == i, false, disabled))
			}
			help = []string{"tab/shift+tab move", "enter choose", "pgup/pgdown review", "esc cancel"}
		}
	} else if g.deleting != nil {
		title = "Delete saved email"
		add(fmt.Sprintf("Delete %q?", g.deleting.Address))
		add("")
		add("Existing credential fields keep their copied values.")
		add("This cannot be undone.")
		add("")
		buttons, line := dialogButtons("Cancel", "Delete", !g.confirm, g.confirm, true, m.saving, inner)
		anchor = len(body)
		if g.confirm {
			anchor += line
		}
		add(buttons)
		help = []string{"tab/left/right choose", "enter confirm", "esc cancel"}
	} else {
		extra := g.status
		if g.list.SettingFilter() {
			help = []string{"enter apply", "esc cancel filter"}
			filter := "/ " + g.list.FilterValue()
			add(terminalui.Message(filter, accent))
			extra = filter
			if g.status != "" {
				extra += "\n" + g.status
			}
		}
		m.fitEmailList(&g.list, g.rows, help, extra)
		if len(g.list.Items()) == 0 {
			add("No saved email addresses.")
			add("Press n to add your first batch.")
		} else {
			add(strings.TrimRight(g.list.View(), "\n"))
		}
		if g.status != "" {
			add(terminalui.Message(g.status, good))
		}
	}
	if g.err != "" {
		add(terminalui.Message(g.err, danger))
	}
	if m.saving {
		add("Saving…")
		anchor = len(body) - 1
		help = []string{"Saving…", "ctrl+c quit"}
	}
	view, offset := (terminalui.Frame{Header: terminalui.Header(strings.TrimSpace(m.vaultLabel + "  Emails")), Title: title, Body: strings.Join(body, "\n"), Help: help, Width: m.width, Height: m.height, MaxWidth: 64, Anchor: anchor, Offset: g.scroll, Destructive: g.deleting != nil}).Render()
	g.scroll = offset
	return view
}

func (m model) emailPickerView() string {
	p := m.editor.emailPicker
	body := "No saved email addresses.\nPress Esc to return and enter an address manually.\nManage saved addresses with m in the browser."
	help := []string{"j/k move", "enter choose", "/ filter", "esc back"}
	filter := ""
	if p.list.SettingFilter() {
		filter = "/ " + p.list.FilterValue()
		help = []string{"enter apply", "esc cancel filter"}
	}
	m.fitEmailList(&p.list, p.rows, help, filter)
	if len(p.list.Items()) > 0 {
		body = strings.TrimRight(p.list.View(), "\n")
	}
	if filter != "" {
		body = terminalui.Message(filter, accent) + "\n" + body
	}
	view, _ := (terminalui.Frame{Header: m.header(), Title: "Choose saved email", Body: body, Help: help, Width: m.width, Height: m.height, MaxWidth: 64}).Render()
	return view
}
