package tui

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/emircancakmakk/cred/internal/terminalui"
)

const (
	revealFor  = 5 * time.Second
	secretMask = "••••••"
	valueLimit = 40
)

var (
	accent = terminalui.Accent
	ink    = terminalui.Ink
	muted  = terminalui.Muted
	rule   = terminalui.Rule
	danger = terminalui.Danger
	good   = terminalui.Good
)

type Copier interface {
	Copy(text string) error
}

type Field struct {
	Name   string
	Value  string
	Secret bool
}

type Service struct {
	Name   string
	Fields []Field
}

type Project struct {
	Name     string
	Services []Service
}

type screen int

const (
	screenProjects screen = iota
	screenServices
	screenFields
)

type hideReveal struct{ id int }

type revealState struct {
	name string
	id   int
}

var (
	keyUp     = key.NewBinding(key.WithKeys("up", "k"))
	keyDown   = key.NewBinding(key.WithKeys("down", "j"))
	keyOpen   = key.NewBinding(key.WithKeys("enter"))
	keyShow   = key.NewBinding(key.WithKeys("s"))
	keyBack   = key.NewBinding(key.WithKeys("esc", "left"))
	keyQuit   = key.NewBinding(key.WithKeys("q"))
	keyFilter = key.NewBinding(key.WithKeys("/"))
	keyStop   = key.NewBinding(key.WithKeys("ctrl+c"))
)

func Run(projects []Project, start string, copier Copier, in io.Reader, out io.Writer, vaultLabel ...string) (string, error) {
	return run(projects, start, copier, nil, in, out, vaultLabel...)
}

// RunEditable opens the browser with persistent CRUD support.
func RunEditable(projects []Project, start string, copier Copier, commit Commit, in io.Reader, out io.Writer, vaultLabel ...string) (string, error) {
	return run(projects, start, copier, commit, in, out, vaultLabel...)
}

func run(projects []Project, start string, copier Copier, commit Commit, in io.Reader, out io.Writer, vaultLabel ...string) (string, error) {
	m, err := newModel(projects, start, copier)
	if err != nil {
		return "", err
	}
	if len(vaultLabel) > 0 {
		m.vaultLabel = vaultLabel[0]
	}
	m.commit = commit
	opts := []tea.ProgramOption{tea.WithAltScreen()}
	if in != nil {
		opts = append(opts, tea.WithInput(in))
	}
	if out != nil {
		opts = append(opts, tea.WithOutput(out))
	}
	final, err := tea.NewProgram(m, opts...).Run()
	if err != nil {
		return "", fmt.Errorf("run selector: %w", err)
	}
	done, ok := final.(model)
	if !ok || done.lastCopied == "" {
		return "", nil
	}
	return done.lastCopied, nil
}

type model struct {
	commit       Commit
	editor       *editor
	deleting     *deletion
	dialogScroll int
	saving       bool
	vaultLabel   string
	projects     []Project
	projectList  list.Model
	serviceList  list.Model
	fieldList    list.Model
	mode         screen
	projectIx    int
	reveal       *revealState
	copier       Copier
	status       string
	lastCopied   string
	width        int
	height       int
	rows         *rowDelegate
}

func newModel(projects []Project, start string, copier Copier) (model, error) {
	sortProjects(projects)
	reveal := &revealState{}
	rows := &rowDelegate{width: 76}
	m := model{
		projects:    projects,
		projectList: newBrowserList("project", "projects", rows, projectItems(projects)),
		serviceList: newBrowserList("service", "services", rows, nil),
		fieldList:   newBrowserList("field", "fields", rows, nil),
		reveal:      reveal,
		copier:      copier,
		width:       80,
		height:      24,
		rows:        rows,
	}
	m = m.resize(80, 24)
	if start == "" {
		return m, nil
	}
	for i, project := range projects {
		if project.Name == start {
			m.projectList.Select(i)
			return m.openProject(i), nil
		}
	}
	return model{}, fmt.Errorf("project %q not found", start)
}

func newBrowserList(singular, plural string, rows *rowDelegate, items []list.Item) list.Model {
	browser := list.New(items, rows, 80, 8)
	browser.SetShowTitle(false)
	browser.SetShowStatusBar(false)
	browser.SetShowHelp(false)
	browser.SetShowFilter(false)
	browser.SetShowPagination(false)
	browser.SetStatusBarItemName(singular, plural)
	browser.FilterInput.Prompt = "/ "
	browser.Styles.FilterPrompt = lipgloss.NewStyle().Foreground(accent)
	browser.Styles.FilterCursor = lipgloss.NewStyle().Foreground(accent)
	browser.Styles.NoItems = lipgloss.NewStyle().Foreground(muted).PaddingLeft(2)
	browser.KeyMap.Quit.SetEnabled(false)
	browser.KeyMap.ForceQuit.SetEnabled(false)

	browser.KeyMap.PrevPage.SetKeys("h", "pgup", "b", "u")
	return browser
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case saved:
		return m.finishSave(msg), nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case hideReveal:
		if msg.id == m.reveal.id {
			m.reveal.name = ""
		}
	case tea.KeyMsg:
		if key.Matches(msg, keyStop) {
			return m.fit(), tea.Quit
		}
		if m.editor != nil || m.deleting != nil {
			return m.updateDialog(msg)
		}
		if m.active().SettingFilter() {
			m, cmd = m.forward(msg)
			break
		}
		var next tea.Model
		next, cmd = m.onKey(msg)
		m = next.(model)
	default:
		if m.editor != nil || m.deleting != nil {
			return m.updateDialog(msg)
		}
		m, cmd = m.forward(msg)
	}
	return m.fit(), cmd
}

func (m model) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.String() == "n":
		return m.startEditor(false)
	case msg.String() == "e":
		return m.startEditor(true)
	case msg.String() == "d":
		return m.startDelete(), nil
	case key.Matches(msg, keyQuit):
		return m, tea.Quit
	case key.Matches(msg, keyBack):
		return m.back()
	case key.Matches(msg, keyOpen):
		return m.onEnter()
	case key.Matches(msg, keyShow):
		return m.onShow()
	case key.Matches(msg, keyUp), key.Matches(msg, keyDown):
		before := m.active().Index()
		m, cmd := m.forward(msg)
		if m.active().Index() != before {
			m.status = ""
		}
		return m, cmd
	case key.Matches(msg, keyFilter):
		return m.forward(msg)
	default:
		return m, nil
	}
}

func (m model) onEnter() (model, tea.Cmd) {
	switch m.mode {
	case screenProjects:
		item, ok := m.projectList.SelectedItem().(projectItem)
		if !ok {
			return m, nil
		}
		return m.openProject(item.index), nil
	case screenServices:
		item, ok := m.serviceList.SelectedItem().(serviceItem)
		if !ok {
			return m, nil
		}
		return m.openService(item.index), nil
	default:
		return m.copyField()
	}
}

func (m model) copyField() (model, tea.Cmd) {
	item, ok := m.fieldList.SelectedItem().(fieldItem)
	if !ok {
		return m, nil
	}
	service, ok := m.currentService()
	if !ok {
		return m, nil
	}
	if m.copier == nil {
		m.status = "copy failed"
		return m, nil
	}
	if err := m.copier.Copy(item.value); err != nil {
		m.status = "copy failed"
		return m, nil
	}
	project := m.projects[m.projectIx]
	m.lastCopied = project.Name + "/" + service.Name + "/" + item.name
	m.status = "copied " + item.name
	return m, nil
}

func (m model) onShow() (model, tea.Cmd) {
	if m.mode != screenFields {
		return m, nil
	}
	item, ok := m.fieldList.SelectedItem().(fieldItem)
	if !ok || !item.secret {
		return m, nil
	}
	m.reveal.id++
	m.reveal.name = item.name
	id := m.reveal.id
	return m, tea.Tick(revealFor, func(time.Time) tea.Msg {
		return hideReveal{id: id}
	})
}

func (m model) back() (model, tea.Cmd) {
	switch m.mode {
	case screenFields:
		m.fieldList.ResetFilter()
		m.mode = screenServices
		m.clearReveal()
		m.status = ""
		return m, nil
	case screenServices:
		m.serviceList.ResetFilter()
		m.mode = screenProjects
		m.clearReveal()
		m.status = ""
		return m, nil
	default:
		return m, tea.Quit
	}
}

func (m model) openProject(index int) model {
	m.projectList.ResetFilter()
	m.projectList.Select(index)
	m.projectIx = index
	project := m.projects[index]
	items := make([]list.Item, len(project.Services))
	for i, service := range project.Services {
		items[i] = serviceItem{name: service.Name, index: i, fields: len(service.Fields)}
	}
	m.serviceList.Title = project.Name
	m.serviceList.ResetFilter()
	m.serviceList.SetItems(items)
	m.serviceList.ResetSelected()
	m.mode = screenServices
	m.clearReveal()
	m.status = ""
	return m.fit()
}

func (m model) openService(index int) model {
	m.serviceList.ResetFilter()
	m.serviceList.Select(index)
	service := m.projects[m.projectIx].Services[index]
	items := make([]list.Item, len(service.Fields))
	for i, field := range service.Fields {
		items[i] = fieldItem{
			name:   field.Name,
			value:  field.Value,
			secret: field.Secret,
			reveal: m.reveal,
		}
	}
	project := m.projects[m.projectIx]
	m.fieldList.Title = project.Name + " / " + service.Name
	m.fieldList.ResetFilter()
	m.fieldList.SetItems(items)
	m.fieldList.ResetSelected()
	m.mode = screenFields
	m.clearReveal()
	m.status = ""
	return m.fit()
}

func (m model) clearReveal() {
	m.reveal.id++
	m.reveal.name = ""
}

func (m model) forward(msg tea.Msg) (model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.mode {
	case screenServices:
		m.serviceList, cmd = m.serviceList.Update(msg)
	case screenFields:
		m.fieldList, cmd = m.fieldList.Update(msg)
	default:
		m.projectList, cmd = m.projectList.Update(msg)
	}
	return m, cmd
}

func (m model) fit() model {
	return m.resize(m.width, m.height)
}

func (m model) resize(width, height int) model {
	if width < 1 {
		width = 80
	}
	if height < 1 {
		height = 24
	}
	m.width = width
	m.height = height
	if m.rows != nil {
		m.rows.width = max(1, min(96, width)-4)
	}
	m.projectList.SetSize(max(1, min(96, width)-4), m.paneHeight(m.projectList))
	m.serviceList.SetSize(max(1, min(96, width)-4), m.paneHeight(m.serviceList))
	m.fieldList.SetSize(max(1, min(96, width)-4), m.paneHeight(m.fieldList))
	return m
}

func (m model) paneHeight(lst list.Model) int {
	n := len(lst.VisibleItems())
	if n < 1 {
		n = 1
	}
	maxHeight := m.height - 9 - lipgloss.Height(wrapHelp(m.browserHelp(), max(1, min(96, m.width)-4)))
	if maxHeight < 3 {
		maxHeight = 3
	}
	if n > maxHeight {
		return maxHeight
	}
	return n
}

func (m model) View() string {
	if m.editor != nil || m.deleting != nil {
		return m.dialogView()
	}
	var b strings.Builder
	if m.active().SettingFilter() {
		b.WriteString(terminalui.Message("/ "+m.active().FilterValue(), accent) + "\n")
	}
	if len(m.active().Items()) == 0 {
		noun := "projects"
		switch m.mode {
		case screenServices:
			noun = "services"
		case screenFields:
			noun = "fields"
		}
		b.WriteString("No " + noun + ".")
		if m.commit != nil {
			b.WriteString("\nPress n to create the first item.")
		}
	} else {
		b.WriteString(strings.TrimRight(m.active().View(), "\n"))
	}
	if m.status != "" {
		color := good
		if m.status == "copy failed" {
			color = danger
		} else if m.status == "editing unavailable" {
			color = terminalui.Warning
		}
		b.WriteString("\n" + terminalui.Message(m.status, color))
	}
	view, _ := (terminalui.Frame{Header: m.header(), Title: m.crumb(), Body: b.String(), Help: m.browserHelp(), Width: m.width, Height: m.height, MaxWidth: 96}).Render()
	return view
}
func (m model) header() string {
	return terminalui.Header(strings.TrimSpace(m.vaultLabel + "  " + m.crumb()))
}

func (m model) crumb() string {
	switch m.mode {
	case screenServices:
		if m.projectIx >= 0 && m.projectIx < len(m.projects) {
			return m.projects[m.projectIx].Name
		}
	case screenFields:
		service, ok := m.currentService()
		if ok && m.projectIx >= 0 && m.projectIx < len(m.projects) {
			return m.projects[m.projectIx].Name + "  /  " + service.Name
		}
	}
	return "Projects"
}

func (m model) footer() string {
	width := max(1, m.width)
	bar := lipgloss.NewStyle().Foreground(rule).Render(strings.Repeat("─", width))
	help := lipgloss.NewStyle().Foreground(muted).Render(wrapHelp(m.browserHelp(), width))
	return "\n" + bar + "\n" + help
}

func (m model) browserHelp() []string {
	if m.active().SettingFilter() {
		return []string{"enter apply", "esc cancel"}
	}
	open := "enter select"
	if m.mode == screenFields {
		open = "enter copy"
	}
	help := []string{"j/k move", open, "esc back", "/ filter"}
	if m.mode == screenFields {
		help = append(help, "s show")
	}
	if m.commit != nil {
		help = append(help, "n new")
		if m.active().SelectedItem() != nil {
			help = append(help, "e edit", "d delete")
		}
	}
	return append(help, "q quit")
}

// Keep each shortcut together where space allows; hard-wrap only tiny terminals.
func wrapHelp(items []string, width int) string { return terminalui.Help(items, width) }

func (m model) active() list.Model {
	switch m.mode {
	case screenServices:
		return m.serviceList
	case screenFields:
		return m.fieldList
	default:
		return m.projectList
	}
}

func (m model) currentService() (Service, bool) {
	if m.projectIx < 0 || m.projectIx >= len(m.projects) {
		return Service{}, false
	}
	item, ok := m.serviceList.SelectedItem().(serviceItem)
	if !ok {
		return Service{}, false
	}
	services := m.projects[m.projectIx].Services
	if item.index < 0 || item.index >= len(services) {
		return Service{}, false
	}
	return services[item.index], true
}

func sortProjects(projects []Project) {
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Name < projects[j].Name
	})
	for i := range projects {
		services := projects[i].Services
		sort.Slice(services, func(a, b int) bool {
			return services[a].Name < services[b].Name
		})
		for j := range services {
			fields := services[j].Fields
			sort.Slice(fields, func(a, b int) bool {
				return fields[a].Name < fields[b].Name
			})
			services[j].Fields = fields
		}
		projects[i].Services = services
	}
}

func projectItems(projects []Project) []list.Item {
	items := make([]list.Item, len(projects))
	for i, project := range projects {
		items[i] = projectItem{name: project.Name, count: len(project.Services), index: i}
	}
	return items
}

func serviceCount(n int) string {
	if n == 1 {
		return "1 service"
	}
	return fmt.Sprintf("%d services", n)
}

type projectItem struct {
	name  string
	count int
	index int
}

func (p projectItem) Title() string       { return p.name }
func (p projectItem) Meta() string        { return serviceCount(p.count) }
func (p projectItem) FilterValue() string { return p.name }

type serviceItem struct {
	name   string
	index  int
	fields int
}

func (s serviceItem) Title() string       { return s.name }
func (s serviceItem) Meta() string        { return fieldCount(s.fields) }
func (s serviceItem) FilterValue() string { return s.name }

func fieldCount(n int) string {
	if n == 1 {
		return "1 field"
	}
	return fmt.Sprintf("%d fields", n)
}

type fieldItem struct {
	name   string
	value  string
	secret bool
	reveal *revealState
}

func (f fieldItem) Title() string { return f.name }

func (f fieldItem) Meta() string {
	if f.secret && (f.reveal == nil || f.reveal.name != f.name) {
		return secretMask
	}
	if !f.secret {
		return clip(f.value, valueLimit)
	}
	return f.value
}

func (f fieldItem) FilterValue() string { return f.name }

type rowDelegate struct {
	width int
}

func (rowDelegate) Height() int                         { return 1 }
func (rowDelegate) Spacing() int                        { return 0 }
func (rowDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d *rowDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	row, ok := item.(interface {
		Title() string
		Meta() string
	})
	if !ok {
		return
	}
	width := max(1, d.width)
	selected := index == m.Index() && m.FilterState() != list.Filtering
	marker := "  "
	nameStyle := lipgloss.NewStyle().Foreground(ink)
	if selected {
		marker = "> "
		nameStyle = nameStyle.Bold(true).Foreground(accent)
	} else if m.FilterState() == list.Filtering && m.FilterValue() == "" {
		nameStyle = lipgloss.NewStyle().Foreground(muted)
	}
	meta := ansi.Truncate(row.Meta(), max(0, width-12), "…")
	if width < 12 {
		fmt.Fprint(w, ansi.Truncate(marker+row.Title(), width, "…"))
		return
	}
	metaStyled := lipgloss.NewStyle().Foreground(muted).Render(meta)
	metaWidth := lipgloss.Width(metaStyled)
	room := width - lipgloss.Width(marker) - metaWidth - 2
	if room < 8 {
		room = 8
	}
	name := ansi.Truncate(row.Title(), room, "…")
	shown := nameStyle.Render(name)
	if m.FilterState() == list.Filtering || m.FilterState() == list.FilterApplied {
		if matches := m.MatchesForItem(index); matchesInside(matches, name) {
			shown = lipgloss.StyleRunes(name, matches, nameStyle.Underline(true), nameStyle)
		}
	}
	fmt.Fprintf(w, "%s%s", marker, shown)
	if meta == "" {
		return
	}
	gap := width - lipgloss.Width(marker) - lipgloss.Width(name) - metaWidth
	if gap < 2 {
		gap = 2
	}
	fmt.Fprintf(w, "%s%s", strings.Repeat(" ", gap), metaStyled)
}

func matchesInside(matches []int, name string) bool {
	n := len([]rune(name))
	for _, index := range matches {
		if index < 0 || index >= n {
			return false
		}
	}
	return true
}

func clip(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	if n < 1 {
		return ""
	}
	return string(runes[:n-1]) + "…"
}
