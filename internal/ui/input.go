package ui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

// composer is the multi-line prompt editor. Enter submits, so newline insertion
// is bound to explicit key combinations.
type composer struct {
	ta      textarea.Model
	width   int
	history []string
	histAt  int
	draft   string
}

func newComposer(st styles) *composer {
	ta := textarea.New()
	ta.Placeholder = "Ask pi anything…  (/help for commands)"
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.MaxHeight = 8
	ta.SetHeight(1)
	ta.Prompt = ""
	ta.FocusedStyle.Placeholder = ta.FocusedStyle.Placeholder.Foreground(st.theme.Muted)
	ta.KeyMap.InsertNewline = key.NewBinding(
		key.WithKeys("alt+enter", "ctrl+j"),
		key.WithHelp("alt+enter", "newline"),
	)
	ta.Focus()

	return &composer{ta: ta, histAt: -1}
}

func (c *composer) SetWidth(width int) {
	if width < 10 {
		width = 10
	}
	if c.width == width {
		return
	}
	c.width = width
	c.ta.SetWidth(width)
}

func (c *composer) Value() string { return c.ta.Value() }
func (c *composer) Reset() {
	c.ta.Reset()
	c.ta.SetHeight(1)
	c.histAt = -1
	c.draft = ""
}
func (c *composer) Empty() bool { return strings.TrimSpace(c.ta.Value()) == "" }
func (c *composer) Height() int { return maxInt(1, c.ta.Height()) }
func (c *composer) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	c.ta, cmd = c.ta.Update(msg)
	return cmd
}

func (c *composer) SetValue(value string) {
	c.ta.SetValue(value)
	c.resize()
}

// SetText replaces the content and moves the cursor to the end.
func (c *composer) SetText(value string) {
	c.ta.SetValue(value)
	c.ta.CursorEnd()
	c.resize()
}

func (c *composer) InsertNewline() {
	c.ta.InsertString("\n")
	c.resize()
}

func (c *composer) resize() {
	lines := strings.Count(c.ta.Value(), "\n") + 1
	if lines > c.ta.MaxHeight {
		lines = c.ta.MaxHeight
	}
	c.ta.SetHeight(maxInt(1, lines))
}

// PushHistory records a submitted prompt.
func (c *composer) PushHistory(value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	if len(c.history) == 0 || c.history[len(c.history)-1] != value {
		c.history = append(c.history, value)
	}
	if len(c.history) > 100 {
		c.history = c.history[len(c.history)-100:]
	}
	c.histAt = -1
	c.draft = ""
}

// HistoryPrev replaces the editor with the previous submitted prompt.
func (c *composer) HistoryPrev() bool {
	if len(c.history) == 0 {
		return false
	}
	if c.histAt == -1 {
		c.draft = c.ta.Value()
		c.histAt = len(c.history) - 1
	} else if c.histAt > 0 {
		c.histAt--
	} else {
		return false
	}
	c.SetText(c.history[c.histAt])
	return true
}

// HistoryNext walks forward through submitted prompts.
func (c *composer) HistoryNext() bool {
	if c.histAt == -1 {
		return false
	}
	if c.histAt < len(c.history)-1 {
		c.histAt++
		c.SetText(c.history[c.histAt])
		return true
	}
	c.histAt = -1
	c.SetText(c.draft)
	return true
}

// completionItem is one slash command candidate.
type completionItem struct {
	Value   string // "/model"
	Detail  string // one-line description
	Kind    string // builtin, extension, prompt, skill
	Builtin bool
}

// completion is the slash command popup state.
type completion struct {
	items    []completionItem
	filtered []completionItem
	cursor   int
	visible  bool
}

func (c *completion) SetItems(items []completionItem) {
	c.items = items
	c.Refresh("")
}

// Refresh recomputes the visible candidates for the current input.
func (c *completion) Refresh(input string) {
	trimmed := strings.TrimSpace(input)
	if !strings.HasPrefix(trimmed, "/") || strings.ContainsAny(trimmed, " \n") {
		c.visible = false
		c.filtered = nil
		c.cursor = 0
		return
	}

	prefix := strings.ToLower(trimmed)
	c.filtered = c.filtered[:0]
	for _, item := range c.items {
		value := strings.ToLower(item.Value)
		if strings.HasPrefix(value, prefix) || strings.Contains(value, prefix) {
			c.filtered = append(c.filtered, item)
		}
	}
	sort.SliceStable(c.filtered, func(i, j int) bool {
		iPrefix := strings.HasPrefix(strings.ToLower(c.filtered[i].Value), prefix)
		jPrefix := strings.HasPrefix(strings.ToLower(c.filtered[j].Value), prefix)
		if iPrefix != jPrefix {
			return iPrefix
		}
		return c.filtered[i].Value < c.filtered[j].Value
	})
	if c.cursor >= len(c.filtered) {
		c.cursor = maxInt(0, len(c.filtered)-1)
	}
	c.visible = len(c.filtered) > 0
}

func (c *completion) Hide() { c.visible = false }

func (c *completion) Move(delta int) {
	if len(c.filtered) == 0 {
		return
	}
	c.cursor = (c.cursor + delta + len(c.filtered)) % len(c.filtered)
}

func (c *completion) Selected() (completionItem, bool) {
	if !c.visible || len(c.filtered) == 0 {
		return completionItem{}, false
	}
	if c.cursor >= len(c.filtered) {
		c.cursor = len(c.filtered) - 1
	}
	return c.filtered[c.cursor], true
}

// builtinCommands are handled inside the client instead of pi.
func builtinCommands() []completionItem {
	return []completionItem{
		{Value: "/model", Detail: "switch model", Kind: "builtin", Builtin: true},
		{Value: "/think", Detail: "cycle thinking level", Kind: "builtin", Builtin: true},
		{Value: "/new", Detail: "start a new session", Kind: "builtin", Builtin: true},
		{Value: "/resume", Detail: "resume another session", Kind: "builtin", Builtin: true},
		{Value: "/compact", Detail: "compact the conversation", Kind: "builtin", Builtin: true},
		{Value: "/name", Detail: "rename this session", Kind: "builtin", Builtin: true},
		{Value: "/export", Detail: "export the session as HTML", Kind: "builtin", Builtin: true},
		{Value: "/stats", Detail: "show token and context usage", Kind: "builtin", Builtin: true},
		{Value: "/clear", Detail: "clear the scrollback", Kind: "builtin", Builtin: true},
		{Value: "/help", Detail: "show key bindings", Kind: "builtin", Builtin: true},
		{Value: "/quit", Detail: "exit pie-tui", Kind: "builtin", Builtin: true},
	}
}
