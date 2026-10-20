// Package terminalui owns cred's terminal presentation, never vault operations.
package terminalui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
)

var (
	Accent  = lipgloss.AdaptiveColor{Light: "#0550AE", Dark: "#7AA2F7"}
	Ink     = lipgloss.AdaptiveColor{Light: "#1F2328", Dark: "#C0CAF5"}
	Muted   = lipgloss.AdaptiveColor{Light: "#656D76", Dark: "#565F89"}
	Rule    = lipgloss.AdaptiveColor{Light: "#D0D7DE", Dark: "#3B4261"}
	Good    = lipgloss.AdaptiveColor{Light: "#1A7F37", Dark: "#9ECE6A"}
	Warning = lipgloss.AdaptiveColor{Light: "#9A6700", Dark: "#E0AF68"}
	Danger  = lipgloss.AdaptiveColor{Light: "#CF222E", Dark: "#F7768E"}
)

func Header(context string) string {
	return lipgloss.NewStyle().Bold(true).Foreground(Accent).Render("cred") + "  " + lipgloss.NewStyle().Foreground(Ink).Bold(true).Render(context)
}

func Help(items []string, width int) string {
	width = max(1, width)
	var lines []string
	line := ""
	for _, item := range items {
		if line != "" && lipgloss.Width(line)+4+lipgloss.Width(item) > width {
			lines = append(lines, line)
			line = ""
		}
		if line != "" {
			line += "    "
		}
		line += item
	}
	lines = append(lines, line)
	return ansi.Hardwrap(strings.Join(lines, "\n"), width, false)
}

// Frame keeps chrome outside the scrollable body. Anchor is the body row that
// should stay in view; offset is manual scrolling relative to that position.
type Frame struct {
	Header, Title, Body                     string
	Help                                    []string
	Width, Height, MaxWidth, Anchor, Offset int
	Destructive                             bool
}

func (f Frame) Render() (string, int) {
	width := min(max(1, f.Width), f.MaxWidth)
	if f.MaxWidth == 0 {
		width = min(64, max(1, f.Width))
	}
	if width < 20 || f.Height < 9 {
		return ansi.Truncate("Resize terminal", max(1, f.Width), ""), 0
	}
	inner := width - 4
	lines := strings.Split(ansi.Hardwrap(f.Body, inner, false), "\n")
	help := Help(f.Help, inner)
	available := f.Height - 8 - lipgloss.Height(help)
	if len(lines) > available {
		help = Help(append(append([]string{}, f.Help...), "pgup/pgdown scroll"), inner)
		available = f.Height - 8 - lipgloss.Height(help)
	}
	if available < 1 {
		return ansi.Truncate("Resize terminal", max(1, f.Width), ""), 0
	}
	base := min(max(0, f.Anchor+3-available), max(0, len(lines)-available))
	start := min(max(0, base+f.Offset), max(0, len(lines)-available))
	end := min(len(lines), start+available)
	divider := strings.Repeat("─", inner)
	if start > 0 || end < len(lines) {
		divider = ansi.Truncate(fmt.Sprintf(" %d–%d/%d ", start+1, end, len(lines))+divider, inner, "")
	}
	color := Accent
	if f.Destructive {
		color = Danger
	}
	title := ansi.Truncate(f.Title, inner, "…")
	content := lipgloss.NewStyle().Foreground(color).Bold(true).Render(title) + "\n\n" + strings.Join(lines[start:end], "\n") + "\n" + lipgloss.NewStyle().Foreground(Rule).Render(divider) + "\n" + lipgloss.NewStyle().Foreground(Muted).Render(help)
	panel := lipgloss.NewStyle().Width(inner+2).Padding(0, 1).Border(lipgloss.RoundedBorder()).BorderForeground(Rule).Render(content)
	header := f.Header
	if header == "" {
		header = Header("")
	}
	return ansi.Truncate(header, f.Width, "…") + "\n\n" + lipgloss.PlaceHorizontal(f.Width, lipgloss.Center, panel), start - base
}

func Input(label string, input textinput.Model, focused, invalid bool, width int) string {
	color := Rule
	marker := "  "
	if focused {
		color = Accent
		marker = "> "
	}
	if invalid {
		color = Danger
	}
	input.Width = max(1, width-5)
	pos := input.Position()
	input.CursorEnd()
	input.CursorStart()
	input.SetCursor(pos)
	labelView := lipgloss.NewStyle().Foreground(Ink).Bold(focused).Render(marker + label)
	box := lipgloss.NewStyle().Width(max(1, width-2)).Padding(0, 1).Border(lipgloss.NormalBorder()).BorderForeground(color).Render(ansi.Truncate(input.View(), max(1, width-4), ""))
	return labelView + "\n" + box
}
func Choice(label string, focused, destructive, disabled bool) string {
	style := lipgloss.NewStyle().Foreground(Ink)
	marker := "  "
	if destructive {
		style = style.Foreground(Danger)
	}
	if focused {
		marker = "> "
		style = style.Bold(true).Foreground(Accent)
		if destructive {
			style = style.Foreground(Danger)
		}
	}
	if disabled {
		marker = "  "
		style = style.Bold(false).Foreground(Muted)
	}
	return style.Render(marker + label)
}

// Actions lays out one shared button row, stacking only when necessary.
func Actions(labels []string, focus int, width int) (string, []int) {
	var b strings.Builder
	anchors := make([]int, len(labels))
	column, row := 0, 0
	for i, label := range labels {
		button := Choice("[ "+label+" ]", i == focus, false, false)
		size := lipgloss.Width(button)
		if column > 0 {
			if column+2+size > width {
				b.WriteString("\n")
				row++
				column = 0
			} else {
				b.WriteString("  ")
				column += 2
			}
		}
		anchors[i] = row
		b.WriteString(button)
		column += size
	}
	return b.String(), anchors
}
func Buttons(first, second string, firstFocus, secondFocus, destructiveSecond, disabled bool, width int) (string, int) {
	a := Choice("[ "+first+" ]", firstFocus, false, disabled)
	b := Choice("[ "+second+" ]", secondFocus, destructiveSecond, disabled)
	if lipgloss.Width(a)+2+lipgloss.Width(b) > width {
		return a + "\n" + b, 1
	}
	return a + "  " + b, 0
}

func Message(text string, color lipgloss.TerminalColor) string {
	return lipgloss.NewStyle().Foreground(color).Render(text)
}
func IsTerminal(w io.Writer) bool { f, ok := w.(*os.File); return ok && term.IsTerminal(int(f.Fd())) }

// Writer adds semantic styling only to human-facing terminal output. It keeps
// bytes unchanged for pipes and files, and never handles raw secret output.
type Writer struct{ io.Writer }

func (w Writer) Write(p []byte) (int, error) {
	if !IsTerminal(w.Writer) || os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return w.Writer.Write(p)
	}
	var result strings.Builder
	for _, line := range strings.SplitAfter(string(p), "\n") {
		text := strings.TrimSuffix(line, "\n")
		trimmed := strings.TrimSpace(text)
		color := Ink
		switch {
		case strings.HasPrefix(trimmed, "cred:"), strings.HasPrefix(trimmed, "Error:"):
			color = Danger
		case strings.Contains(trimmed, "cancelled"), strings.Contains(trimmed, "incomplete"), strings.HasPrefix(trimmed, "Next step:"), strings.HasPrefix(trimmed, "CRED_VAULT is set"):
			color = Warning
		case strings.HasPrefix(trimmed, "created "), strings.HasPrefix(trimmed, "updated "), strings.HasPrefix(trimmed, "deleted "), strings.HasPrefix(trimmed, "added "), strings.HasPrefix(trimmed, "copied "), strings.HasPrefix(trimmed, "Ready:"), strings.HasPrefix(trimmed, "connected "):
			color = Good
		}
		if color == Ink {
			if label, value, ok := strings.Cut(text, ": "); ok {
				result.WriteString(lipgloss.NewStyle().Bold(true).Foreground(Accent).Render(label+":") + " " + Message(value, Ink))
			} else if strings.HasSuffix(text, ":") {
				result.WriteString(lipgloss.NewStyle().Bold(true).Foreground(Accent).Render(text))
			} else {
				result.WriteString(Message(text, color))
			}
		} else {
			result.WriteString(Message(text, color))
		}
		if strings.HasSuffix(line, "\n") {
			result.WriteString("\n")
		}
	}
	_, err := io.WriteString(w.Writer, result.String())
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func Output(w io.Writer) io.Writer {
	if IsTerminal(w) {
		return Writer{w}
	}
	return w
}
