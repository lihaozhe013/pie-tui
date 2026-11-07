package ui

import (
	"github.com/charmbracelet/glamour/ansi"
	glamourstyles "github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
)

// theme is the dark, low-noise palette of the client. Every color is adaptive so
// the UI stays readable on light terminals without a separate theme file.
type theme struct {
	Base        lipgloss.AdaptiveColor
	Muted       lipgloss.AdaptiveColor
	Faint       lipgloss.AdaptiveColor
	Accent      lipgloss.AdaptiveColor
	AccentSoft  lipgloss.AdaptiveColor
	User        lipgloss.AdaptiveColor
	Assistant   lipgloss.AdaptiveColor
	Thinking    lipgloss.AdaptiveColor
	Tool        lipgloss.AdaptiveColor
	Success     lipgloss.AdaptiveColor
	Warn        lipgloss.AdaptiveColor
	Danger      lipgloss.AdaptiveColor
	DiffAdd     lipgloss.AdaptiveColor
	DiffDel     lipgloss.AdaptiveColor
	Border      lipgloss.AdaptiveColor
	Panel       lipgloss.AdaptiveColor
	Activity    lipgloss.AdaptiveColor
	ActivityDim lipgloss.AdaptiveColor
}

func defaultTheme() theme {
	return theme{
		Base:        lipgloss.AdaptiveColor{Light: "#2B2B33", Dark: "#D5D6DB"},
		Muted:       lipgloss.AdaptiveColor{Light: "#5F6068", Dark: "#8A8B94"},
		Faint:       lipgloss.AdaptiveColor{Light: "#84858D", Dark: "#5C5D66"},
		Accent:      lipgloss.AdaptiveColor{Light: "#4F85D1", Dark: "#8BB9FA"},
		AccentSoft:  lipgloss.AdaptiveColor{Light: "#638EC9", Dark: "#6F9AD6"},
		User:        lipgloss.AdaptiveColor{Light: "#1667A8", Dark: "#9AD1FF"},
		Assistant:   lipgloss.AdaptiveColor{Light: "#2B2B33", Dark: "#D5D6DB"},
		Thinking:    lipgloss.AdaptiveColor{Light: "#6B6C74", Dark: "#8A8B94"},
		Tool:        lipgloss.AdaptiveColor{Light: "#4F85D1", Dark: "#A9C7F0"},
		Success:     lipgloss.AdaptiveColor{Light: "#1F7A45", Dark: "#8BD5A4"},
		Warn:        lipgloss.AdaptiveColor{Light: "#8A5B00", Dark: "#E4C07A"},
		Danger:      lipgloss.AdaptiveColor{Light: "#B02A2A", Dark: "#F08A8A"},
		DiffAdd:     lipgloss.AdaptiveColor{Light: "#1F7A45", Dark: "#8BD5A4"},
		DiffDel:     lipgloss.AdaptiveColor{Light: "#B02A2A", Dark: "#F08A8A"},
		Border:      lipgloss.AdaptiveColor{Light: "#C6C7CE", Dark: "#33343D"},
		Panel:       lipgloss.AdaptiveColor{Light: "#F2F2F5", Dark: "#1C1D24"},
		Activity:    lipgloss.AdaptiveColor{Light: "#1667D8", Dark: "#5AA9FF"},
		ActivityDim: lipgloss.AdaptiveColor{Light: "#7FA9DC", Dark: "#2F5D8F"},
	}
}

// styles holds prebuilt Lip Gloss styles for one theme.
type styles struct {
	theme theme

	header            lipgloss.Style
	headerKey         lipgloss.Style
	statusBar         lipgloss.Style
	activityMarker    lipgloss.Style
	activityMarkerDim lipgloss.Style
	activityLabel     lipgloss.Style
	activityMeta      lipgloss.Style

	userPrefix      lipgloss.Style
	userText        lipgloss.Style
	assistantPrefix lipgloss.Style
	toolLine        lipgloss.Style
	toolName        lipgloss.Style
	toolMeta        lipgloss.Style
	thinkingHead    lipgloss.Style
	thinkingBody    lipgloss.Style
	notice          lipgloss.Style
	errorText       lipgloss.Style
	muted           lipgloss.Style
	accent          lipgloss.Style

	editorBox   lipgloss.Style
	editorHint  lipgloss.Style
	scrollRail  lipgloss.Style
	scrollThumb lipgloss.Style
	completion  lipgloss.Style
	compActive  lipgloss.Style
	overlayBox  lipgloss.Style
	overlayHead lipgloss.Style
	overlayHint lipgloss.Style
	pickerItem  lipgloss.Style
	pickerSel   lipgloss.Style
	diffAdd     lipgloss.Style
	diffDel     lipgloss.Style
	diffMeta    lipgloss.Style
}

func newStyles(t theme) styles {
	return styles{
		theme: t,

		header: lipgloss.NewStyle().
			Foreground(t.Accent).
			Bold(true),
		headerKey: lipgloss.NewStyle().Foreground(t.Muted),
		statusBar: lipgloss.NewStyle().Foreground(t.Muted),

		activityMarker:    lipgloss.NewStyle().Foreground(t.Activity).Bold(true),
		activityMarkerDim: lipgloss.NewStyle().Foreground(t.ActivityDim),
		activityLabel:     lipgloss.NewStyle().Foreground(t.Activity).Bold(true),
		activityMeta:      lipgloss.NewStyle().Foreground(t.Faint),

		userPrefix:      lipgloss.NewStyle().Foreground(t.User).Bold(true),
		userText:        lipgloss.NewStyle().Foreground(t.Base),
		assistantPrefix: lipgloss.NewStyle().Foreground(t.Accent).Bold(true),
		toolLine:        lipgloss.NewStyle().Foreground(t.Muted),
		toolName:        lipgloss.NewStyle().Foreground(t.Tool),
		toolMeta:        lipgloss.NewStyle().Foreground(t.Faint),
		thinkingHead:    lipgloss.NewStyle().Foreground(t.Thinking).Italic(true),
		thinkingBody:    lipgloss.NewStyle().Foreground(t.Thinking).Italic(true),
		notice:          lipgloss.NewStyle().Foreground(t.Warn),
		errorText:       lipgloss.NewStyle().Foreground(t.Danger),
		muted:           lipgloss.NewStyle().Foreground(t.Muted),
		accent:          lipgloss.NewStyle().Foreground(t.Accent),

		editorBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Border).
			Padding(0, 1),
		editorHint:  lipgloss.NewStyle().Foreground(t.Faint),
		scrollRail:  lipgloss.NewStyle().Foreground(t.Faint),
		scrollThumb: lipgloss.NewStyle().Foreground(t.AccentSoft),
		completion: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Border).
			Padding(0, 1),
		compActive: lipgloss.NewStyle().Foreground(t.Accent).Bold(true),
		overlayBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.AccentSoft).
			Padding(1, 2),
		overlayHead: lipgloss.NewStyle().Foreground(t.Accent).Bold(true),
		overlayHint: lipgloss.NewStyle().Foreground(t.Faint),
		pickerItem:  lipgloss.NewStyle().Foreground(t.Base),
		pickerSel: lipgloss.NewStyle().
			Foreground(t.Accent).
			Bold(true),
		diffAdd:  lipgloss.NewStyle().Foreground(t.DiffAdd),
		diffDel:  lipgloss.NewStyle().Foreground(t.DiffDel),
		diffMeta: lipgloss.NewStyle().Foreground(t.Faint),
	}
}

// markdownStyle derives a glamour style from the palette so code blocks and
// headings match the surrounding chrome.
func (s styles) markdownStyle() ansi.StyleConfig {
	style := glamourstyles.DarkStyleConfig

	style.Document.Margin = uintPtr(0)
	style.Document.BlockPrefix = ""
	style.Document.BlockSuffix = ""
	style.Document.Color = colorPtr(s.theme.Base.Dark)

	for _, heading := range []*ansi.StyleBlock{&style.H1, &style.H2, &style.H3, &style.H4, &style.H5, &style.H6} {
		heading.Color = colorPtr(s.theme.Accent.Dark)
		heading.Bold = boolPtr(true)
	}
	style.Heading.Color = colorPtr(s.theme.Accent.Dark)

	style.Emph.Color = colorPtr(s.theme.AccentSoft.Dark)
	style.Strong.Color = colorPtr(s.theme.Base.Dark)
	style.Strong.Bold = boolPtr(true)
	style.Link.Color = colorPtr(s.theme.User.Dark)
	style.LinkText.Color = colorPtr(s.theme.User.Dark)
	style.LinkText.Underline = boolPtr(true)
	style.HorizontalRule.Color = colorPtr(s.theme.Faint.Dark)

	style.Code.Color = colorPtr(s.theme.Warn.Dark)
	style.Code.BackgroundColor = colorPtr(s.theme.Panel.Dark)

	style.CodeBlock.BackgroundColor = colorPtr(s.theme.Panel.Dark)
	style.CodeBlock.Color = colorPtr(s.theme.Base.Dark)
	style.CodeBlock.Margin = uintPtr(1)

	style.BlockQuote.Indent = uintPtr(1)
	style.BlockQuote.IndentToken = stringPtr("│ ")
	style.BlockQuote.Color = colorPtr(s.theme.Muted.Dark)
	style.BlockQuote.IndentToken = stringPtr("│ ")

	style.List.LevelIndent = 2
	style.Item.Color = colorPtr(s.theme.AccentSoft.Dark)
	style.Enumeration.Color = colorPtr(s.theme.AccentSoft.Dark)

	style.Table.CenterSeparator = stringPtr("┼")
	style.Table.ColumnSeparator = stringPtr("│")
	style.Table.RowSeparator = stringPtr("─")
	style.Table.Color = colorPtr(s.theme.Base.Dark)

	return style
}

func colorPtr(value string) *string  { return &value }
func stringPtr(value string) *string { return &value }
func boolPtr(value bool) *bool       { return &value }
func uintPtr(value uint) *uint       { return &value }
