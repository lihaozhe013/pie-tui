package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// openStats shows the session counters and refreshes them while the panel is
// open. The same numbers power the status bar's token estimate.
func (m *Model) openStats() tea.Cmd {
	m.showStatsPanel()
	return m.refreshStats()
}

func (m *Model) showStatsPanel() {
	tab := settingsTab{ID: "stats", Label: "Statistics"}
	stats := m.stats
	usage := stats.ContextUsage
	model := "no model"
	if m.state.Model != nil {
		model = m.state.Model.Label()
	}
	session := m.state.SessionName
	if session == "" {
		session = m.state.SessionID
	}
	if session == "" {
		session = "(unsaved)"
	}

	tab.Rows = []settingsRow{
		{ID: "stats-session", Label: "Session", Kind: settingsInfo, Current: func() string { return session }},
		{ID: "stats-model", Label: "Model", Kind: settingsInfo, Current: func() string { return model }},
		{ID: "stats-messages", Label: "Messages", Kind: settingsInfo, Current: func() string {
			return fmt.Sprintf("%d user · %d assistant · %d tool calls · %d total",
				stats.UserMessages, stats.AssistantMessages, stats.ToolCalls, stats.TotalMessages)
		}},
		{ID: "stats-tokens", Label: "Tokens", Kind: settingsInfo, Current: func() string {
			return formatTokens(stats.Tokens.Total)
		}, Help: fmt.Sprintf("input %s · output %s · cache read %s · cache write %s",
			formatTokens(stats.Tokens.Input), formatTokens(stats.Tokens.Output),
			formatTokens(stats.Tokens.CacheRead), formatTokens(stats.Tokens.CacheWrite))},
		{ID: "stats-cost", Label: "Cost", Kind: settingsInfo, Current: func() string {
			return fmt.Sprintf("$%.4f", stats.Cost)
		}},
		{ID: "stats-context", Label: "Context", Kind: settingsInfo, Current: func() string {
			if usage == nil || usage.ContextWindow <= 0 {
				return "(not reported)"
			}
			return fmt.Sprintf("%s / %s · %.0f%%", formatTokens(usage.Tokens), formatTokens(usage.ContextWindow), usage.Percent)
		}},
		{ID: "stats-file", Label: "Session file", Kind: settingsInfo, Current: func() string {
			if stats.SessionFile == "" {
				return "(unsaved)"
			}
			return stats.SessionFile
		}},
		{ID: "stats-refresh", Label: "Refresh", Kind: settingsAction, Run: func(m *Model, s *settingsState) tea.Cmd {
			return m.refreshStats()
		}},
	}
	panel := newSettingsPanel("Statistics", []settingsTab{tab})
	panel.action = "stats"
	m.settingsPanel = panel
	m.openOverlay(&overlay{kind: overlaySettings, action: "stats", settings: panel})
}
