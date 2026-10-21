package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/emircancakmakk/cred/internal/terminalui"
)

// dialogView shares the panel geometry between forms and destructive actions.
// Only the body scrolls, keeping the title and keyboard help visible.
func (m model) dialogView() string {
	view, _ := m.renderDialog()
	return view
}

func (m model) renderDialog() (string, int) {
	if m.editor != nil && m.editor.emailPicker != nil {
		return m.emailPickerView(), 0
	}
	if e := m.editor; e != nil && e.picker != nil {
		e.picker.Width, e.picker.Height = m.width, m.height
		return e.picker.View(), e.picker.Scroll
	}
	width := min(64, m.width)
	if width < 20 || m.height < 9 {
		return ansi.Truncate("Resize terminal", max(1, m.width), ""), 0
	}
	inner := width - 4 // border and one cell of padding on either side
	var body []string
	anchor := 0
	add := func(s string) {
		body = append(body, strings.Split(ansi.Hardwrap(s, inner, false), "\n")...)
	}
	title := ""
	help := []string{"tab/shift+tab move", "enter next", "esc cancel"}
	if e := m.editor; e != nil {
		title = "New " + string(e.change.Kind)
		if e.change.Action == Edit {
			title = "Edit " + string(e.change.Kind)
		}
		add(dialogInput("Name", e.name, e.focus == 0, e.nameErr != "", inner))
		if e.nameErr != "" {
			add(lipgloss.NewStyle().Foreground(danger).Render(e.nameErr))
		}
		if e.change.Kind == FieldKind {
			help = append(help, "ctrl+g generate")
			add("")
			if e.focus == 1 {
				anchor = len(body)
			}
			add(dialogInput("Value", e.value, e.focus == 1, false, inner))
			if e.secret {
				state := "Hidden"
				if e.value.EchoMode == textinput.EchoNormal {
					state = "Visible"
				}
				add(terminalui.Message("  "+state, terminalui.Muted))
				if e.focus == 1 {
					help = append(help, "ctrl+r show/hide")
				}
			}
			add("")
			if e.focus == 2 {
				anchor = len(body)
			}
			check := "[ ] Secret"
			if e.secret {
				check = "[x] Secret"
			}
			add(dialogChoice(check, e.focus == 2, false, m.saving))
			if e.focus == 2 {
				help[1] = "space toggle"
			}
			add("")
			if e.focus == 3 {
				anchor = len(body)
				help[1] = "enter generate"
			}
			add(dialogChoice("[ Generate password ]", e.focus == 3, false, m.saving))
			if e.generateErr != "" {
				add(lipgloss.NewStyle().Foreground(danger).Render(e.generateErr))
			}
			if e.isEmail() {
				help = append(help, "ctrl+e saved emails")
				add("")
				if e.focus == 4 {
					anchor = len(body)
					help[1] = "enter choose"
				}
				add(dialogChoice("[ Choose saved email ]", e.focus == 4, false, m.saving))
			}
		}
		add("")
		buttons, buttonLine := dialogButtons("Save", "Cancel", e.focus == e.saveFocus(), e.focus == e.saveFocus()+1, false, m.saving, inner)
		if e.focus >= e.saveFocus() {
			anchor = len(body)
			if e.focus == e.saveFocus()+1 {
				anchor += buttonLine
			}
			help[1] = "enter choose"
		}
		add(buttons)
		if e.err != "" {
			add("")
			add(lipgloss.NewStyle().Foreground(danger).Render(e.err))
		}
	} else if d := m.deleting; d != nil {
		title = "Delete " + string(d.change.Kind)
		add(lipgloss.NewStyle().Foreground(ink).Bold(true).Render(fmt.Sprintf("Delete %q?", d.change.Name)))
		if d.summary != "" {
			add("")
			add(lipgloss.NewStyle().Foreground(muted).Render(d.summary))
		}
		add("")
		add(lipgloss.NewStyle().Foreground(danger).Render("This cannot be undone."))
		add("")
		buttons, buttonLine := dialogButtons("Cancel", "Delete", !d.confirm, d.confirm, true, m.saving, inner)
		anchor = len(body)
		if d.confirm {
			anchor += buttonLine
		}
		add(buttons)
		if d.err != "" {
			add("")
			add(lipgloss.NewStyle().Foreground(danger).Render(d.err))
		}
		help = []string{"←/→ or tab choose", "enter confirm", "esc cancel"}
	}
	if m.saving {
		add("")
		add(lipgloss.NewStyle().Foreground(accent).Render("Saving…"))
		help = []string{"Saving…", "ctrl+c quit"}
		anchor = len(body) - 1
	}

	return (terminalui.Frame{Header: m.header(), Title: title, Body: strings.Join(body, "\n"), Help: help, Width: m.width, Height: m.height, MaxWidth: 64, Anchor: anchor, Offset: m.dialogScroll, Destructive: m.deleting != nil}).Render()
}

func dialogInput(label string, input textinput.Model, focused, invalid bool, width int) string {
	return terminalui.Input(label, input, focused, invalid, width)
}
func dialogChoice(label string, focused, destructive, disabled bool) string {
	return terminalui.Choice(label, focused, destructive, disabled)
}
func dialogButtons(first, second string, firstFocus, secondFocus, destructiveSecond, disabled bool, width int) (string, int) {
	return terminalui.Buttons(first, second, firstFocus, secondFocus, destructiveSecond, disabled, width)
}
