package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/wangzi5151/mcpscope/internal/mcp"
)

// ---------------------------------------------------------------------------
// Model
// ---------------------------------------------------------------------------

type tabID int

const (
	tabTools tabID = iota
	tabResources
	tabPrompts
	tabHistory
)

func (t tabID) label() string {
	switch t {
	case tabTools:
		return "Tools"
	case tabResources:
		return "Resources"
	case tabPrompts:
		return "Prompts"
	default:
		return "History"
	}
}

type screen int

const (
	screenBrowse screen = iota
	screenCall
	screenResult
	screenInfo
)

type historyEntry struct {
	At       time.Time
	Tool     string
	ArgsJSON string
	Output   string
	IsError  bool
}

type Model struct {
	client      *mcp.Client
	serverLabel string

	serverName         string
	serverVersion      string
	serverInstructions string

	tab    tabID
	screen screen

	tools     []mcp.Tool
	resources []mcp.Resource
	prompts   []mcp.Prompt
	history   []historyEntry

	toolList    list.Model
	resList     list.Model
	promptList  list.Model
	historyList list.Model

	ta textarea.Model
	vp viewport.Model

	callToolName string // tool being called / whose result is shown
	resultErr    bool
	calling      bool // a call/read is in flight

	loadingTools, loadingRes, loadingPrompts bool
	loadErr                                  string

	width, height int
	ready         bool
}

// ---------------------------------------------------------------------------
// List items
// ---------------------------------------------------------------------------

type toolItem struct{ tool mcp.Tool }

func (i toolItem) Title() string       { return i.tool.Name }
func (i toolItem) Description() string { return firstLine(i.tool.Description) }
func (i toolItem) FilterValue() string { return i.tool.Name }

type resourceItem struct{ r mcp.Resource }

func (i resourceItem) Title() string       { return i.r.Name }
func (i resourceItem) Description() string { return i.r.URI }
func (i resourceItem) FilterValue() string { return i.r.Name + " " + i.r.URI }

type promptItem struct{ p mcp.Prompt }

func (i promptItem) Title() string       { return i.p.Name }
func (i promptItem) Description() string { return firstLine(i.p.Description) }
func (i promptItem) FilterValue() string { return i.p.Name }

type historyItem struct {
	e historyEntry
}

func (i historyItem) Title() string {
	status := okStyle.Render("ok")
	if i.e.IsError {
		status = errStyle.Render("err")
	}
	return fmt.Sprintf("%s %s %s", i.e.At.Format("15:04:05"), status, i.e.Tool)
}
func (i historyItem) Description() string { return firstLine(i.e.ArgsJSON) }
func (i historyItem) FilterValue() string { return i.e.Tool }

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

type toolsLoadedMsg struct {
	tools []mcp.Tool
	err   error
}
type resourcesLoadedMsg struct {
	resources []mcp.Resource
	err       error
}
type promptsLoadedMsg struct {
	prompts []mcp.Prompt
	err     error
}
type callDoneMsg struct {
	entry historyEntry
}

// readDoneMsg carries a resource-read or prompt-render result to the
// result screen (not recorded in tool call history).
type readDoneMsg struct {
	title   string
	output  string
	isError bool
}

// ---------------------------------------------------------------------------
// Construction
// ---------------------------------------------------------------------------

func newDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.ShowDescription = true
	d.Styles.SelectedTitle = d.Styles.SelectedTitle.Foreground(lipgloss.Color("#7C6FF7")).Bold(true)
	return d
}

func newList(title string) list.Model {
	l := list.New(nil, newDelegate(), 0, 0)
	l.Title = title
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	return l
}

// NewModel builds the TUI model around an already-connected client.
func NewModel(client *mcp.Client, name, version, instructions string) Model {
	ta := textarea.New()
	ta.Placeholder = `{"key": "value"}`
	ta.ShowLineNumbers = false
	ta.CharLimit = 65536

	return Model{
		client:             client,
		serverLabel:        strings.TrimSpace(name + " " + version),
		serverName:         name,
		serverVersion:      version,
		serverInstructions: instructions,
		toolList:           newList("Tools"),
		resList:            newList("Resources"),
		promptList:         newList("Prompts"),
		historyList:        newList("Call history"),
		ta:                 ta,
		loadingTools:       true,
		loadingRes:         true,
		loadingPrompts:     true,
	}
}

// Run starts the interactive TUI.
func Run(client *mcp.Client, name, version, instructions string) error {
	p := tea.NewProgram(NewModel(client, name, version, instructions), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

func (m Model) fetchTools() tea.Msg {
	tools, err := m.client.ListTools(context.Background())
	return toolsLoadedMsg{tools: tools, err: err}
}

func (m Model) fetchResources() tea.Msg {
	res, err := m.client.ListResources(context.Background())
	return resourcesLoadedMsg{resources: res, err: err}
}

func (m Model) fetchPrompts() tea.Msg {
	prompts, err := m.client.ListPrompts(context.Background())
	return promptsLoadedMsg{prompts: prompts, err: err}
}

func (m Model) callTool(name, argsJSON string) tea.Cmd {
	return func() tea.Msg {
		var args map[string]any
		if strings.TrimSpace(argsJSON) != "" {
			if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
				return callDoneMsg{entry: historyEntry{
					At: time.Now(), Tool: name, ArgsJSON: argsJSON,
					Output: "invalid JSON arguments: " + err.Error(), IsError: true,
				}}
			}
		}
		res, err := m.client.CallTool(context.Background(), name, args)
		entry := historyEntry{At: time.Now(), Tool: name, ArgsJSON: argsJSON}
		if err != nil {
			entry.Output = "rpc error: " + err.Error()
			entry.IsError = true
			return callDoneMsg{entry: entry}
		}
		entry.Output = renderResult(res)
		entry.IsError = res.IsError
		return callDoneMsg{entry: entry}
	}
}

// readResourceCmd reads a resource and shows its content on the result screen.
func (m Model) readResourceCmd(uri string) tea.Cmd {
	return func() tea.Msg {
		contents, err := m.client.ReadResource(context.Background(), uri)
		if err != nil {
			return readDoneMsg{title: uri, output: "rpc error: " + err.Error(), isError: true}
		}
		var sb strings.Builder
		for i, c := range contents {
			if i > 0 {
				sb.WriteString("\n---\n")
			}
			sb.WriteString(c.Text)
		}
		out := sb.String()
		if strings.TrimSpace(out) == "" {
			out = dimStyle.Render("(empty resource)")
		}
		return readDoneMsg{title: uri, output: out}
	}
}

// getPromptCmd renders a prompt and shows its messages on the result screen.
func (m Model) getPromptCmd(name string) tea.Cmd {
	return func() tea.Msg {
		msgs, err := m.client.GetPrompt(context.Background(), name, nil)
		if err != nil {
			return readDoneMsg{title: "prompt: " + name, output: "rpc error: " + err.Error(), isError: true}
		}
		var sb strings.Builder
		for _, msg := range msgs {
			sb.WriteString(panelTitleStyle.Render(msg.Role) + "\n")
			if msg.Content.Text != "" {
				sb.WriteString(msg.Content.Text + "\n")
			} else {
				sb.WriteString(dimStyle.Render("(non-text content)") + "\n")
			}
			sb.WriteString("\n")
		}
		out := strings.TrimSpace(sb.String())
		if out == "" {
			out = dimStyle.Render("(empty prompt)")
		}
		return readDoneMsg{title: "prompt: " + name, output: out}
	}
}

func renderResult(r *mcp.CallResult) string {
	var sb strings.Builder
	if r.IsError {
		sb.WriteString(errStyle.Render("tool returned an error") + "\n\n")
	}
	if text := r.Text(); text != "" {
		sb.WriteString(text)
	}
	// Surface non-text blocks (images etc.) as a summary line.
	nonText := 0
	for _, b := range r.Content {
		if b.Type != "text" {
			nonText++
		}
	}
	if nonText > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(dimStyle.Render(fmt.Sprintf("(+%d non-text content block(s) not shown)", nonText)))
	}
	if sb.Len() == 0 {
		sb.WriteString(dimStyle.Render("(empty result)"))
	}
	return sb.String()
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.fetchTools, m.fetchResources, m.fetchPrompts)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ready = true
		m.resize()
		return m, nil

	case toolsLoadedMsg:
		m.loadingTools = false
		if msg.err != nil {
			m.loadErr = "tools/list: " + msg.err.Error()
			return m, nil
		}
		m.tools = msg.tools
		items := make([]list.Item, len(m.tools))
		for i, t := range m.tools {
			items[i] = toolItem{tool: t}
		}
		m.toolList.SetItems(items)
		m.resize()
		return m, nil

	case resourcesLoadedMsg:
		m.loadingRes = false
		if msg.err != nil {
			m.loadErr = "resources/list: " + msg.err.Error()
			return m, nil
		}
		m.resources = msg.resources
		items := make([]list.Item, len(m.resources))
		for i, r := range m.resources {
			items[i] = resourceItem{r: r}
		}
		m.resList.SetItems(items)
		m.resize()
		return m, nil

	case promptsLoadedMsg:
		m.loadingPrompts = false
		if msg.err != nil {
			m.loadErr = "prompts/list: " + msg.err.Error()
			return m, nil
		}
		m.prompts = msg.prompts
		items := make([]list.Item, len(m.prompts))
		for i, p := range m.prompts {
			items[i] = promptItem{p: p}
		}
		m.promptList.SetItems(items)
		m.resize()
		return m, nil

	case callDoneMsg:
		m.history = append(m.history, msg.entry)
		m.refreshHistoryList()
		m.callToolName = msg.entry.Tool
		m.resultErr = msg.entry.IsError
		m.vp.SetContent(msg.entry.Output)
		m.vp.GotoTop()
		m.screen = screenResult
		m.calling = false
		m.resize()
		return m, nil

	case readDoneMsg:
		m.callToolName = msg.title
		m.resultErr = msg.isError
		m.vp.SetContent(msg.output)
		m.vp.GotoTop()
		m.screen = screenResult
		m.calling = false
		m.resize()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Route to the focused component.
	var cmd tea.Cmd
	switch m.screen {
	case screenCall:
		m.ta, cmd = m.ta.Update(msg)
	case screenResult:
		m.vp, cmd = m.vp.Update(msg)
	default:
		cmd = m.updateActiveList(msg)
	}
	return m, cmd
}

func (m *Model) updateActiveList(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch m.tab {
	case tabTools:
		m.toolList, cmd = m.toolList.Update(msg)
	case tabResources:
		m.resList, cmd = m.resList.Update(msg)
	case tabPrompts:
		m.promptList, cmd = m.promptList.Update(msg)
	case tabHistory:
		m.historyList, cmd = m.historyList.Update(msg)
	}
	return cmd
}

// filterActive reports whether the user is actively editing the filter
// on the current tab's list. (Note: FilterInput.Focused() is true even on a
// fresh list in bubbles v0.20, so it can't be used for this.)
func (m Model) filterActive() bool {
	switch m.tab {
	case tabTools:
		return m.toolList.FilterState() == list.Filtering
	case tabResources:
		return m.resList.FilterState() == list.Filtering
	case tabPrompts:
		return m.promptList.FilterState() == list.Filtering
	default:
		return m.historyList.FilterState() == list.Filtering
	}
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Global quit.
	if key == "ctrl+c" {
		return m, tea.Quit
	}

	switch m.screen {
	case screenCall:
		switch key {
		case "esc":
			m.screen = screenBrowse
			m.ta.Blur()
			return m, nil
		case "ctrl+s":
			args := m.ta.Value()
			name := m.callToolName
			m.ta.Blur()
			m.screen = screenBrowse // result screen arrives with callDoneMsg
			m.calling = true
			return m, m.callTool(name, args)
		}
		var cmd tea.Cmd
		m.ta, cmd = m.ta.Update(msg)
		return m, cmd

	case screenResult:
		switch key {
		case "esc", "q":
			m.screen = screenBrowse
			return m, nil
		}
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd

	case screenInfo:
		// Any of these closes the info panel.
		switch key {
		case "esc", "q", "i", "enter":
			m.screen = screenBrowse
			return m, nil
		}
		return m, nil
	}

	// screenBrowse keys. If the user is editing the list filter, all keys
	// belong to it (typing "q" in a filter must not quit the app).
	if m.filterActive() {
		var cmd tea.Cmd
		cmd = m.updateActiveList(msg)
		return m, cmd
	}
	switch key {
	case "q":
		return m, tea.Quit
	case "tab", "right", "l":
		m.tab = (m.tab + 1) % 4
		return m, nil
	case "shift+tab", "left", "h":
		m.tab = (m.tab + 3) % 4
		return m, nil
	case "1":
		m.tab = tabTools
		return m, nil
	case "2":
		m.tab = tabResources
		return m, nil
	case "3":
		m.tab = tabPrompts
		return m, nil
	case "4":
		m.tab = tabHistory
		return m, nil
	case "i":
		m.screen = screenInfo
		return m, nil
	case "r":
		// Re-run the selected history entry: pre-fill its arguments.
		if m.tab == tabHistory {
			if sel, ok := m.historyList.SelectedItem().(historyItem); ok {
				m.callToolName = sel.e.Tool
				m.ta.SetValue(sel.e.ArgsJSON)
				if strings.TrimSpace(m.ta.Value()) == "" {
					m.ta.SetValue("{}")
				}
				m.ta.Focus()
				m.screen = screenCall
				m.resize()
			}
		}
		return m, nil
	case "enter":
		switch m.tab {
		case tabTools:
			if sel, ok := m.toolList.SelectedItem().(toolItem); ok {
				m.callToolName = sel.tool.Name
				m.ta.SetValue(argsTemplate(sel.tool.InputSchema))
				m.ta.Focus()
				m.screen = screenCall
				m.resize()
			}
		case tabResources:
			if sel, ok := m.resList.SelectedItem().(resourceItem); ok {
				m.calling = true
				return m, m.readResourceCmd(sel.r.URI)
			}
		case tabPrompts:
			if sel, ok := m.promptList.SelectedItem().(promptItem); ok {
				m.calling = true
				return m, m.getPromptCmd(sel.p.Name)
			}
		case tabHistory:
			// Open the selected entry in the scrollable result view.
			if sel, ok := m.historyList.SelectedItem().(historyItem); ok {
				m.callToolName = sel.e.Tool + " @ " + sel.e.At.Format("15:04:05")
				m.resultErr = sel.e.IsError
				m.vp.SetContent("Arguments:\n" + sel.e.ArgsJSON + "\n\nOutput:\n" + sel.e.Output)
				m.vp.GotoTop()
				m.screen = screenResult
				m.resize()
			}
		}
		return m, nil
	}

	var cmd tea.Cmd
	cmd = m.updateActiveList(msg)
	return m, cmd
}

func (m *Model) refreshHistoryList() {
	items := make([]list.Item, len(m.history))
	// Newest first.
	for i, e := range m.history {
		items[len(m.history)-1-i] = historyItem{e: e}
	}
	m.historyList.SetItems(items)
}

func (m *Model) resize() {
	if !m.ready {
		return
	}
	listH := m.height - 7
	if listH < 5 {
		listH = 5
	}
	leftW := m.width / 3
	if leftW < 24 {
		leftW = 24
	}
	m.toolList.SetSize(leftW, listH)
	m.resList.SetSize(leftW, listH)
	m.promptList.SetSize(leftW, listH)
	m.historyList.SetSize(leftW, listH)
	m.ta.SetWidth(m.width - 8)
	m.vp.Width = m.width - 4
	m.vp.Height = m.height - 8
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (m Model) View() string {
	if !m.ready {
		return "connecting…"
	}
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("◉ mcpscope") + dimStyle.Render("  "+m.serverLabel))
	sb.WriteString("\n")
	sb.WriteString(m.tabsView())
	sb.WriteString("\n")

	switch m.screen {
	case screenCall:
		sb.WriteString(m.callView())
	case screenResult:
		sb.WriteString(m.resultView())
	case screenInfo:
		sb.WriteString(m.infoView())
	default:
		sb.WriteString(m.browseView())
	}

	sb.WriteString("\n")
	sb.WriteString(m.helpView())
	if m.loadErr != "" {
		sb.WriteString("\n" + errStyle.Render("⚠ "+m.loadErr))
	}
	return sb.String()
}

func (m Model) tabsView() string {
	tabs := []tabID{tabTools, tabResources, tabPrompts, tabHistory}
	var parts []string
	for _, t := range tabs {
		label := fmt.Sprintf("%d %s", int(t)+1, t.label())
		if t == m.tab {
			parts = append(parts, tabActiveStyle.Render(label))
		} else {
			parts = append(parts, tabInactiveStyle.Render(label))
		}
	}
	return strings.Join(parts, " ")
}

func (m Model) browseView() string {
	left := m.activeListView()
	right := m.detailView()
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (m Model) activeListView() string {
	switch m.tab {
	case tabTools:
		if m.loadingTools {
			return panelStyle.Render("loading tools…")
		}
		return m.toolList.View()
	case tabResources:
		if m.loadingRes {
			return panelStyle.Render("loading resources…")
		}
		return m.resList.View()
	case tabPrompts:
		if m.loadingPrompts {
			return panelStyle.Render("loading prompts…")
		}
		return m.promptList.View()
	default:
		if len(m.history) == 0 {
			return panelStyle.Render(dimStyle.Render("no calls yet — pick a tool and press enter"))
		}
		return m.historyList.View()
	}
}

func (m Model) detailView() string {
	var body string
	switch m.tab {
	case tabTools:
		body = m.toolDetail()
	case tabResources:
		body = m.resourceDetail()
	case tabPrompts:
		body = m.promptDetail()
	default:
		body = m.historyDetail()
	}
	w := m.width - m.width/3 - 6
	if w < 20 {
		w = 20
	}
	return panelStyle.Width(w).Render(body)
}

func (m Model) toolDetail() string {
	sel, ok := m.toolList.SelectedItem().(toolItem)
	if !ok {
		return dimStyle.Render("no tools — is the server advertising any?")
	}
	var sb strings.Builder
	sb.WriteString(panelTitleStyle.Render(sel.tool.Name))
	sb.WriteString("\n\n")
	if sel.tool.Description != "" {
		sb.WriteString(sel.tool.Description + "\n\n")
	} else {
		sb.WriteString(dimStyle.Render("(no description)") + "\n\n")
	}
	sb.WriteString(panelTitleStyle.Render("Input schema"))
	sb.WriteString("\n")
	sb.WriteString(prettyJSON(sel.tool.InputSchema))
	return sb.String()
}

func (m Model) resourceDetail() string {
	sel, ok := m.resList.SelectedItem().(resourceItem)
	if !ok {
		return dimStyle.Render("no resources")
	}
	var sb strings.Builder
	sb.WriteString(panelTitleStyle.Render(sel.r.Name) + "\n\n")
	sb.WriteString(dimStyle.Render("URI: ") + sel.r.URI + "\n")
	if sel.r.MimeType != "" {
		sb.WriteString(dimStyle.Render("MIME: ") + sel.r.MimeType + "\n")
	}
	if sel.r.Description != "" {
		sb.WriteString("\n" + sel.r.Description)
	}
	return sb.String()
}

func (m Model) promptDetail() string {
	sel, ok := m.promptList.SelectedItem().(promptItem)
	if !ok {
		return dimStyle.Render("no prompts")
	}
	var sb strings.Builder
	sb.WriteString(panelTitleStyle.Render(sel.p.Name) + "\n\n")
	if sel.p.Description != "" {
		sb.WriteString(sel.p.Description)
	} else {
		sb.WriteString(dimStyle.Render("(no description)"))
	}
	return sb.String()
}

func (m Model) historyDetail() string {
	sel, ok := m.historyList.SelectedItem().(historyItem)
	if !ok {
		return dimStyle.Render("select a call to see its output")
	}
	var sb strings.Builder
	status := okStyle.Render("ok")
	if sel.e.IsError {
		status = errStyle.Render("error")
	}
	sb.WriteString(panelTitleStyle.Render(sel.e.Tool) + "  " + status + "\n")
	sb.WriteString(dimStyle.Render(sel.e.At.Format("2006-01-02 15:04:05")) + "\n\n")
	sb.WriteString(panelTitleStyle.Render("Arguments") + "\n")
	sb.WriteString(sel.e.ArgsJSON + "\n\n")
	sb.WriteString(panelTitleStyle.Render("Output") + "\n")
	sb.WriteString(sel.e.Output)
	return sb.String()
}

func (m Model) callView() string {
	var sb strings.Builder
	sb.WriteString(panelTitleStyle.Render("Call "+m.callToolName) + "\n")
	sb.WriteString(dimStyle.Render("Edit the JSON arguments, then press ") + keyStyle.Render("ctrl+s") + dimStyle.Render(" to run."))
	sb.WriteString("\n\n")
	sb.WriteString(m.ta.View())
	return panelStyle.Render(sb.String())
}

func (m Model) resultView() string {
	var sb strings.Builder
	status := okStyle.Render("ok")
	if m.resultErr {
		status = errStyle.Render("error")
	}
	sb.WriteString(panelTitleStyle.Render("Result: "+m.callToolName) + "  " + status + "\n\n")
	sb.WriteString(m.vp.View())
	return sb.String()
}

func (m Model) infoView() string {
	var sb strings.Builder
	sb.WriteString(panelTitleStyle.Render("Server info") + "\n\n")
	name := m.serverName
	if name == "" {
		name = "(unknown)"
	}
	sb.WriteString(dimStyle.Render("Name:      ") + name + "\n")
	sb.WriteString(dimStyle.Render("Version:   ") + orDefault(m.serverVersion, "(unknown)") + "\n")
	sb.WriteString(dimStyle.Render("Protocol:  ") + mcp.ProtocolVersion + "\n\n")
	sb.WriteString(panelTitleStyle.Render("Instructions") + "\n")
	sb.WriteString(orDefault(strings.TrimSpace(m.serverInstructions), dimStyle.Render("(none provided)")))
	return panelStyle.Render(sb.String())
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func (m Model) helpView() string {
	switch m.screen {
	case screenCall:
		return strings.Join([]string{
			keyHelp("ctrl+s", "run"),
			keyHelp("esc", "cancel"),
		}, "   ")
	case screenResult:
		return strings.Join([]string{
			keyHelp("↑/↓", "scroll"),
			keyHelp("esc", "back"),
			keyHelp("q", "quit"),
		}, "   ")
	case screenInfo:
		return strings.Join([]string{
			keyHelp("esc/i", "back"),
			keyHelp("q", "quit"),
		}, "   ")
	default:
		base := []string{
			keyHelp("tab", "switch tab"),
			keyHelp("↑/↓ j/k", "navigate"),
			keyHelp("/", "filter"),
			keyHelp("i", "server info"),
		}
		switch m.tab {
		case tabTools:
			base = append(base, keyHelp("enter", "call tool"))
		case tabResources:
			base = append(base, keyHelp("enter", "read resource"))
		case tabPrompts:
			base = append(base, keyHelp("enter", "render prompt"))
		case tabHistory:
			base = append(base, keyHelp("enter", "view"))
			base = append(base, keyHelp("r", "re-run"))
		}
		base = append(base, keyHelp("q", "quit"))
		help := strings.Join(base, "   ")
		if m.calling {
			help = dimStyle.Render("⏳ working…") + "   " + help
		}
		return help
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func prettyJSON(v any) string {
	if v == nil {
		return dimStyle.Render("(none)")
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return dimStyle.Render("(unprintable)")
	}
	return string(b)
}

// argsTemplate builds a starter JSON object from a tool's input schema,
// pre-filling each property with its example, default, or a type-based zero.
func argsTemplate(schema map[string]any) string {
	props, _ := schema["properties"].(map[string]any)
	if len(props) == 0 {
		return "{}"
	}
	m := make(map[string]any, len(props))
	for name, ps := range props {
		m[name] = placeholder(ps)
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(b)
}

func placeholder(ps any) any {
	pm, ok := ps.(map[string]any)
	if !ok {
		return ""
	}
	if ex, ok := pm["example"]; ok {
		return ex
	}
	if d, ok := pm["default"]; ok {
		return d
	}
	switch pm["type"] {
	case "integer", "number":
		return 0
	case "boolean":
		return false
	case "array":
		return []any{}
	case "object":
		return map[string]any{}
	default:
		return ""
	}
}
