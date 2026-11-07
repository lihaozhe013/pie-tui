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
	Alias   string // canonical name when Value is an alias, e.g. "/model"
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
		return completionLess(c.filtered[i], c.filtered[j], prefix)
	})
	if c.cursor >= len(c.filtered) {
		c.cursor = maxInt(0, len(c.filtered)-1)
	}
	c.visible = len(c.filtered) > 0
}

func (c *completion) Hide() { c.visible = false }

// completionLess ranks candidates: an exact name beats a prefix match, a prefix
// match beats a substring match, and canonical commands precede their aliases so
// the popup does not open on an alias.
func completionLess(a, b completionItem, prefix string) bool {
	aValue, bValue := strings.ToLower(a.Value), strings.ToLower(b.Value)
	if (aValue == prefix) != (bValue == prefix) {
		return aValue == prefix
	}
	aPrefix := strings.HasPrefix(aValue, prefix)
	bPrefix := strings.HasPrefix(bValue, prefix)
	if aPrefix != bPrefix {
		return aPrefix
	}
	if (a.Alias != "") != (b.Alias != "") {
		return a.Alias == ""
	}
	return a.Value < b.Value
}

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
