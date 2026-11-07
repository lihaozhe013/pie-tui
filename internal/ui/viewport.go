package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/lipgloss"
)

// blockKind classifies one entry of the transcript.
type blockKind int

const (
	blockUser blockKind = iota
	blockAssistant
	blockThinking
	blockTool
	blockDiff
	blockNotice
	blockError
)

// blockStatus is the lifecycle of a tool or notice block.
type blockStatus int

const (
	statusPending blockStatus = iota
	statusRunning
	statusOK
	statusError
)

// block is one rendered unit of the conversation.
//
// Assistant blocks keep a per-markdown-segment render cache so streaming only
// re-renders the unstable tail.
type block struct {
	kind   blockKind
	callID string
	text   string
	title  string
	meta   string
	body   string
	status blockStatus

	collapsed bool
	streaming bool
	revision  int

	cache struct {
		revision int
		width    int
		value    string
		valid    bool
	}
	stable struct {
		width int
		src   []string
		out   []string
	}
}

func newBlock(kind blockKind, text string) *block {
	return &block{kind: kind, text: text, revision: 1}
}

func (b *block) setText(text string) {
	if b.text == text {
		return
	}
	b.text = text
	b.revision++
}

func (b *block) invalidate() { b.revision++ }

// transcript owns the scrollback: a list of blocks rendered into a viewport.
type transcript struct {
	vp          viewport.Model
	blocks      []*block
	styles      styles
	md          markdownRenderer
	width       int
	height      int
	stickBottom bool
	dirty       bool
}

func newTranscript(st styles) *transcript {
	vp := viewport.New(0, 0)
	vp.MouseWheelEnabled = true
	vp.MouseWheelDelta = 3

	return &transcript{
		vp:          vp,
		styles:      st,
		md:          markdownRenderer{style: st.markdownStyle()},
		stickBottom: true,
		dirty:       true,
	}
}

// SetSize resizes the scrollback area.
func (t *transcript) SetSize(width, height int) {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	if t.width == width && t.height == height {
		return
	}
	t.width, t.height = width, height
	t.vp.Width, t.vp.Height = width, height
	t.dirty = true
}

// Add appends a block and schedules a re-render.
func (t *transcript) Add(b *block) {
	t.blocks = append(t.blocks, b)
	t.dirty = true
}

// Clear drops the whole scrollback.
func (t *transcript) Clear() {
	t.blocks = nil
	t.dirty = true
	t.vp.SetContent("")
	t.vp.GotoBottom()
}

// Blocks exposes the current blocks without copying.
func (t *transcript) Blocks() []*block { return t.blocks }

// MarkDirty schedules a re-render on the next View.
func (t *transcript) MarkDirty() { t.dirty = true }

// SetStyles swaps the palette and invalidates rendered caches.
func (t *transcript) SetStyles(st styles) {
	t.styles = st
	t.md = markdownRenderer{style: st.markdownStyle()}
	for _, b := range t.blocks {
		b.cache.valid = false
		b.stable.width = 0
		b.invalidate()
	}
	t.dirty = true
}

// LastAssistant returns the most recent assistant block, if any.
func (t *transcript) LastAssistant() *block {
	for i := len(t.blocks) - 1; i >= 0; i-- {
		if t.blocks[i].kind == blockAssistant {
			return t.blocks[i]
		}
	}
	return nil
}

// Tool returns the tool block with the given call ID.
func (t *transcript) Tool(callID string) *block {
	for i := len(t.blocks) - 1; i >= 0; i-- {
		if t.blocks[i].callID == callID {
			return t.blocks[i]
		}
	}
	return nil
}

// Update forwards scroll input to the viewport and tracks bottom stickiness.
func (t *transcript) Update(msg tea.Msg) tea.Cmd {
	wasSticky := t.stickBottom
	var cmd tea.Cmd
	t.vp, cmd = t.vp.Update(msg)
	if wasSticky && t.vp.AtBottom() {
		t.stickBottom = true
	} else {
		t.stickBottom = t.vp.AtBottom()
	}
	return cmd
}

// View renders the visible slice of the scrollback.
func (t *transcript) View() string {
	if t.dirty {
		t.rebuild()
	}
	return t.vp.View()
}

// AtBottom reports whether the viewport is scrolled to the newest content.
func (t *transcript) AtBottom() bool { return t.vp.AtBottom() }

// ScrollToBottom pins the viewport to the newest content.
func (t *transcript) ScrollToBottom() {
	t.stickBottom = true
	t.vp.GotoBottom()
}

func (t *transcript) rebuild() {
	if t.width <= 0 || t.height <= 0 {
		return
	}
	parts := make([]string, 0, len(t.blocks))
	for _, b := range t.blocks {
		if out := t.render(b); out != "" {
			parts = append(parts, out)
		}
	}
	t.vp.SetContent(strings.Join(parts, "\n\n"))
	if t.stickBottom {
		t.vp.GotoBottom()
	}
	t.dirty = false
}

func (t *transcript) render(b *block) string {
	width := t.width
	if width < 8 {
		width = 8
	}

	if !b.streaming && b.cache.valid && b.cache.revision == b.revision && b.cache.width == width {
		return b.cache.value
	}

	var out string
	switch b.kind {
	case blockUser:
		out = t.renderUser(b, width)
	case blockAssistant:
		out = t.renderAssistant(b, width)
	case blockThinking:
		out = t.renderThinking(b, width)
	case blockTool:
		out = t.renderTool(b, width)
	case blockDiff:
		out = t.renderDiffBlock(b, width)
	case blockNotice:
		out = renderNotice(b.text, width, t.styles.notice)
	case blockError:
		out = renderNotice(b.text, width, t.styles.errorText)
	}

	b.cache.value = out
	b.cache.revision = b.revision
	b.cache.width = width
	b.cache.valid = true
	return out
}

func (t *transcript) renderUser(b *block, width int) string {
	prefix := t.styles.userPrefix.Render("› ")
	body := b.text
	if strings.TrimSpace(body) == "" {
		body = "(empty message)"
	}
	// Indent continuation lines under the prefix without reflowing the user's
	// own line breaks.
	lines := strings.Split(body, "\n")
	var out strings.Builder
	for i, line := range lines {
		if i == 0 {
			out.WriteString(prefix)
		} else {
			out.WriteString("  ")
		}
		out.WriteString(t.styles.userText.Width(width - 2).Render(line))
		out.WriteString("\n")
	}
	return strings.TrimRight(out.String(), "\n")
}

func (t *transcript) renderAssistant(b *block, width int) string {
	text := strings.TrimSuffix(b.text, "\n")
	if strings.TrimSpace(text) == "" {
		return ""
	}

	split := splitMarkdown(text)
	stable := &b.stable
	if stable.width != width {
		stable.width = width
		stable.src = nil
		stable.out = nil
	}
	if len(stable.src) > len(split.stable) {
		stable.src = stable.src[:len(split.stable)]
		stable.out = stable.out[:len(split.stable)]
	}
	for i, segment := range split.stable {
		if i < len(stable.src) && stable.src[i] == segment {
			continue
		}
		rendered := t.md.render(segment, width)
		if i < len(stable.src) {
			stable.src[i] = segment
			stable.out[i] = rendered
		} else {
			stable.src = append(stable.src, segment)
			stable.out = append(stable.out, rendered)
		}
	}

	parts := make([]string, 0, len(stable.out)+1)
	parts = append(parts, stable.out...)
	if split.tail != "" {
		tail := split.tail
		if split.fenceOpen {
			// Close the open fence so the tail renders as code instead of
			// flickering between paragraph and block styling.
			tail += "\n" + strings.Repeat(string(split.fenceChar), split.fenceLen)
		}
		parts = append(parts, t.md.render(tail, width))
	}

	return strings.TrimRight(strings.Join(parts, "\n\n"), "\n")
}

func (t *transcript) renderThinking(b *block, width int) string {
	head := t.styles.thinkingHead.Render("○ thinking")
	dots := b.meta
	if dots != "" {
		head = t.styles.thinkingHead.Render("○ thinking · " + dots)
	}
	if b.collapsed || strings.TrimSpace(b.text) == "" {
		return head
	}
	body := t.styles.thinkingBody.Width(width - 2).Render(strings.TrimRight(b.text, "\n"))
	return head + "\n" + indentLines(body, "  ")
}

// Auto-preview thresholds keep the transcript scannable the way mature CLI
// agents do: short command output and small diffs stay visible like a
// terminal session while longer bodies fold to their one-line summary.
// Ctrl+o reveals everything; expandedBodyCap bounds what that renders.
const (
	autoToolPreviewLines = 8
	autoDiffPreviewLines = 16
	expandedBodyCap      = 200
)

// bodyLines counts the rendered lines of a block body.
func bodyLines(body string) int {
	body = strings.TrimRight(body, "\n")
	if body == "" {
		return 0
	}
	return strings.Count(body, "\n") + 1
}

// autoCollapsed reports whether a body exceeds its default preview size.
func autoCollapsed(kind blockKind, body string) bool {
	if kind == blockDiff {
		return bodyLines(body) > autoDiffPreviewLines
	}
	return bodyLines(body) > autoToolPreviewLines
}

func (t *transcript) renderTool(b *block, width int) string {
	marker := "~"
	color := t.styles.toolLine
	switch b.status {
	case statusOK:
		marker = "●"
	case statusError:
		marker = "!"
		color = t.styles.errorText
	case statusPending:
		marker = "○"
	}
	if b.status == statusRunning {
		marker = "~"
	}

	head := color.Render(marker) + " " + t.styles.toolName.Render(b.title)
	if b.meta != "" {
		head += " " + t.styles.toolMeta.Render(b.meta)
	}
	if b.status == statusError {
		head += " " + t.styles.errorText.Render("failed")
	}

	if strings.TrimSpace(b.body) == "" {
		return head
	}

	lines := strings.Split(strings.TrimRight(b.body, "\n"), "\n")
	if b.collapsed {
		return head + "\n  " + t.styles.faint.Render(fmt.Sprintf("… +%d lines (ctrl+o)", len(lines)))
	}

	hidden := 0
	if len(lines) > expandedBodyCap {
		hidden = len(lines) - expandedBodyCap
		lines = lines[:expandedBodyCap]
	}
	wrapped := t.styles.toolMeta.Width(width - 2).Render(strings.Join(lines, "\n"))
	out := head + "\n" + indentLines(wrapped, "  ")
	if hidden > 0 {
		out += "\n  " + t.styles.faint.Render(fmt.Sprintf("… +%d more lines", hidden))
	}
	return out
}

func (t *transcript) renderDiffBlock(b *block, width int) string {
	head := t.styles.toolName.Render("● " + b.title)
	if b.meta != "" {
		head += " " + t.renderDiffMeta(b.meta)
	}
	if strings.TrimSpace(b.body) == "" {
		return head
	}

	lines := strings.Split(strings.TrimRight(b.body, "\n"), "\n")
	if b.collapsed {
		return head + "\n  " + t.styles.faint.Render(fmt.Sprintf("… +%d lines (ctrl+o)", len(lines)))
	}

	hidden := 0
	if len(lines) > expandedBodyCap {
		hidden = len(lines) - expandedBodyCap
		lines = lines[:expandedBodyCap]
	}
	clip := lipgloss.NewStyle().MaxWidth(maxInt(width-2, 8))
	var out strings.Builder
	out.WriteString(head)
	for _, line := range lines {
		style := t.styles.diffMeta
		switch {
		case strings.HasPrefix(line, "+"):
			style = t.styles.diffAdd
		case strings.HasPrefix(line, "-"):
			style = t.styles.diffDel
		}
		out.WriteString("\n  ")
		out.WriteString(style.Render(clip.Render(line)))
	}
	if hidden > 0 {
		out.WriteString("\n  " + t.styles.faint.Render(fmt.Sprintf("… +%d more lines", hidden)))
	}
	return out.String()
}

// renderDiffMeta colors the counts in a diff summary such as "+12 -3
// (truncated)": added lines green, removed lines red, anything else dim.
func (t *transcript) renderDiffMeta(meta string) string {
	parts := strings.Fields(meta)
	for i, part := range parts {
		switch {
		case strings.HasPrefix(part, "+") && len(part) > 1:
			parts[i] = t.styles.diffAdd.Render(part)
		case strings.HasPrefix(part, "-") && len(part) > 1:
			parts[i] = t.styles.diffDel.Render(part)
		default:
			parts[i] = t.styles.diffMeta.Render(part)
		}
	}
	return strings.Join(parts, " ")
}

func renderNotice(text string, width int, style lipgloss.Style) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	return indentLines(style.Width(width).Render(text), "")
}

func indentLines(s, prefix string) string {
	if prefix == "" {
		return s
	}
	lines := strings.Split(s, "\n")
	for i := range lines {
		if lines[i] == "" {
			continue
		}
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}

// markdownRenderer caches one glamour renderer per width.
type markdownRenderer struct {
	style    ansi.StyleConfig
	width    int
	renderer *glamour.TermRenderer
	failed   bool
}

func (m *markdownRenderer) render(src string, width int) string {
	src = strings.TrimSpace(src)
	if src == "" {
		return ""
	}
	renderer := m.rendererFor(width)
	if renderer == nil {
		return wrapPlain(src, width)
	}
	out, err := renderer.Render(src)
	if err != nil {
		return wrapPlain(src, width)
	}
	return strings.TrimRight(out, "\n")
}

func (m *markdownRenderer) rendererFor(width int) *glamour.TermRenderer {
	if m.failed {
		return nil
	}
	if m.renderer != nil && m.width == width {
		return m.renderer
	}
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStyles(m.style),
		glamour.WithWordWrap(width),
		glamour.WithPreservedNewLines(),
	)
	if err != nil {
		m.failed = true
		return nil
	}
	m.renderer = renderer
	m.width = width
	return renderer
}

func wrapPlain(src string, width int) string {
	if width < 1 {
		return src
	}
	return lipgloss.NewStyle().Width(width).Render(src)
}

// markdownSplit separates complete markdown segments from the still-growing
// tail. Segments only split on blank lines outside fenced code blocks.
type markdownSplit struct {
	stable    []string
	tail      string
	fenceOpen bool
	fenceChar byte
	fenceLen  int
}

func splitMarkdown(src string) markdownSplit {
	lines := strings.Split(src, "\n")

	var (
		stable   []string
		start    int
		fenceCh  byte
		fenceLen int
	)

	for i, line := range lines {
		if ch, n, ok := fenceMarker(line); ok {
			switch {
			case fenceCh == 0:
				fenceCh, fenceLen = ch, n
			case ch == fenceCh && n >= fenceLen && strings.TrimSpace(strings.TrimLeft(line, " \t")[n:]) == "":
				fenceCh, fenceLen = 0, 0
			}
			continue
		}
		if fenceCh == 0 && strings.TrimSpace(line) == "" {
			if i > start {
				stable = append(stable, strings.Join(lines[start:i], "\n"))
			}
			start = i + 1
		}
	}

	return markdownSplit{
		stable:    stable,
		tail:      strings.TrimRight(strings.Join(lines[start:], "\n"), "\n"),
		fenceOpen: fenceCh != 0,
		fenceChar: fenceCh,
		fenceLen:  fenceLen,
	}
}

// fenceMarker reports the fence delimiter of a line, if it starts one.
func fenceMarker(line string) (byte, int, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	if len(trimmed) < 3 {
		return 0, 0, false
	}
	ch := trimmed[0]
	if ch != '`' && ch != '~' {
		return 0, 0, false
	}
	count := 0
	for count < len(trimmed) && trimmed[count] == ch {
		count++
	}
	if count < 3 {
		return 0, 0, false
	}
	return ch, count, true
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
