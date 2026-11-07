package ui

import (
	"sort"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"
	"github.com/rivo/uniseg"
)

// composerMaxRows caps the number of rows the composer shows before the rest
// of the buffer scrolls. It matches the cap of most chat composers: enough for
// a paragraph, not so much that the transcript disappears.
const composerMaxRows = 8

// composer is the multi-line prompt editor. Enter submits, so newline insertion
// is bound to explicit key combinations.
type composer struct {
	ta      textarea.Model
	styles  styles
	width   int
	maxRows int // visible row cap, lowered on very short terminals
	rows    int // visual rows the current buffer occupies after soft wrapping
	offset  int // first visible row, mirrored from the textarea viewport

	history []string
	histAt  int
	draft   string
}

func newComposer(st styles) *composer {
	ta := textarea.New()
	ta.Placeholder = "Ask pi anything…  (ctrl+p for commands · /help)"
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	// MaxHeight also limits the number of logical lines; leave it unlimited and
	// cap the visible height ourselves instead.
	ta.MaxHeight = 0
	ta.SetHeight(1)
	ta.Prompt = ""
	ta.FocusedStyle.Placeholder = ta.FocusedStyle.Placeholder.Foreground(st.theme.Muted)
	ta.KeyMap.InsertNewline = key.NewBinding(
		key.WithKeys("alt+enter", "ctrl+j"),
		key.WithHelp("alt+enter", "newline"),
	)
	ta.Focus()

	return &composer{ta: ta, styles: st, maxRows: composerMaxRows, rows: 1, histAt: -1}
}

// SetWidth reserves the rightmost column for the scrollbar.
func (c *composer) SetWidth(width int) {
	if width < 10 {
		width = 10
	}
	if c.width == width {
		return
	}
	c.width = width
	c.ta.SetWidth(maxInt(1, width-1))
	c.resize()
}

// SetMaxRows lowers the visible row cap so the composer fits a short terminal.
// The configured cap is always the upper bound, so growing the terminal again
// restores it.
func (c *composer) SetMaxRows(rows int) {
	rows = minInt(rows, composerMaxRows)
	if rows < 1 {
		rows = 1
	}
	if c.maxRows == rows {
		return
	}
	c.maxRows = rows
	c.resize()
}

// SetStyles swaps the palette used by the editor's placeholder.
func (c *composer) SetStyles(st styles) {
	c.styles = st
	c.ta.FocusedStyle.Placeholder = c.ta.FocusedStyle.Placeholder.Foreground(st.theme.Muted)
}

func (c *composer) Value() string { return c.ta.Value() }
func (c *composer) Reset() {
	c.ta.Reset()
	c.ta.SetHeight(1)
	c.offset = 0
	c.rows = 1
	c.histAt = -1
	c.draft = ""
}
func (c *composer) Empty() bool { return strings.TrimSpace(c.ta.Value()) == "" }
func (c *composer) Height() int { return maxInt(1, c.ta.Height()) }
func (c *composer) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	c.ta, cmd = c.ta.Update(msg)
	c.resize()
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

// View renders the editor with a one-column scrollbar when the buffer is
// taller than the visible area.
func (c *composer) View() string {
	view := c.ta.View()
	height := c.ta.Height()
	if c.rows <= height || height < 1 {
		return view
	}

	track := c.scrollbar(height)
	lines := strings.Split(view, "\n")
	for i, line := range lines {
		if i >= len(track) {
			break
		}
		lines[i] = line + track[i]
	}
	return strings.Join(lines, "\n")
}

// scrollbar builds the vertical track: a dim rail with a thumb whose size and
// position reflect the visible fraction of the buffer.
func (c *composer) scrollbar(height int) []string {
	track := make([]string, height)
	if height < 1 || c.rows <= height {
		return track
	}

	thumb := maxInt(1, height*height/c.rows)
	thumb = minInt(thumb, height)
	travel := c.rows - height
	position := 0
	if travel > 0 && height > thumb {
		position = c.offset * (height - thumb) / travel
	}

	for i := range track {
		if i >= position && i < position+thumb {
			track[i] = c.styles.scrollThumb.Render("█")
		} else {
			track[i] = c.styles.scrollRail.Render("│")
		}
	}
	return track
}

// resize recomputes the visual height after an edit and keeps the mirrored
// scroll offset aligned with the textarea's cursor-driven scrolling.
func (c *composer) resize() {
	width := c.ta.Width()
	if width < 1 {
		width = 1
	}

	previous := maxInt(1, c.ta.Height())
	c.rows = wrappedRows(c.ta.Value(), width)

	visible := minInt(c.rows, c.maxRows)
	visible = maxInt(1, visible)
	if visible != c.ta.Height() {
		c.ta.SetHeight(visible)
	}

	c.syncScroll(previous, visible)
}

// syncScroll mirrors textarea.repositionView and then clamps the offset to the
// new viewport, so the scrollbar matches what the textarea shows.
func (c *composer) syncScroll(previous, visible int) {
	if c.rows <= previous {
		c.offset = 0
	} else {
		cursor := c.cursorRow()
		minimum := c.offset
		maximum := minimum + previous - 1
		switch {
		case cursor < minimum:
			c.offset = cursor
		case cursor > maximum:
			c.offset += cursor - maximum
		}
	}

	if limit := c.rows - visible; c.offset > limit {
		c.offset = limit
	}
	if c.offset < 0 {
		c.offset = 0
	}
}

// cursorRow is the visual row the cursor sits on, counting soft wraps.
func (c *composer) cursorRow() int {
	width := c.ta.Width()
	if width < 1 {
		return 0
	}

	lines := strings.Split(c.ta.Value(), "\n")
	row := c.ta.Line()
	if row < 0 {
		row = 0
	}
	if row > len(lines)-1 {
		row = len(lines) - 1
	}

	total := 0
	for i := 0; i < row; i++ {
		total += len(wrapTextLine([]rune(lines[i]), width))
	}
	return total + c.ta.LineInfo().RowOffset
}

// wrappedRows counts the visual rows of a value at the given width.
func wrappedRows(value string, width int) int {
	rows := 0
	for _, line := range strings.Split(value, "\n") {
		rows += len(wrapTextLine([]rune(line), width))
	}
	return maxInt(1, rows)
}

// wrapTextLine mirrors the soft wrapping applied inside bubbles/textarea. The
// composer needs the same row count to size itself and place the scrollbar, so
// revisit this helper whenever the textarea dependency is upgraded.
func wrapTextLine(runes []rune, width int) [][]rune {
	if width < 1 {
		return [][]rune{{}}
	}

	var (
		lines  = [][]rune{{}}
		word   []rune
		row    int
		spaces int
	)

	for _, r := range runes {
		if unicode.IsSpace(r) {
			spaces++
		} else {
			word = append(word, r)
		}

		if spaces > 0 {
			if uniseg.StringWidth(string(lines[row]))+uniseg.StringWidth(string(word))+spaces > width {
				row++
				lines = append(lines, []rune{})
				lines[row] = append(lines[row], word...)
			} else {
				lines[row] = append(lines[row], word...)
			}
			lines[row] = append(lines[row], repeatSpaces(spaces)...)
			spaces = 0
			word = nil
			continue
		}

		lastWidth := runewidth.RuneWidth(word[len(word)-1])
		if uniseg.StringWidth(string(word))+lastWidth > width {
			if len(lines[row]) > 0 {
				row++
				lines = append(lines, []rune{})
			}
			lines[row] = append(lines[row], word...)
			word = nil
		}
	}

	if uniseg.StringWidth(string(lines[row]))+uniseg.StringWidth(string(word))+spaces >= width {
		lines = append(lines, []rune{})
		lines[row+1] = append(lines[row+1], word...)
		spaces++
		lines[row+1] = append(lines[row+1], repeatSpaces(spaces)...)
	} else {
		lines[row] = append(lines[row], word...)
		spaces++
		lines[row] = append(lines[row], repeatSpaces(spaces)...)
	}

	return lines
}

func repeatSpaces(n int) []rune {
	return []rune(strings.Repeat(" ", n))
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
