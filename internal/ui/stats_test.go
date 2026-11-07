package ui

import (
	"strings"
	"testing"

	"github.com/lihaozhe/pie-tui/internal/rpc"
)

func TestStatsPanelShowsCounters(t *testing.T) {
	m := New(newFakeClient(), Options{})
	m.state.Model = &rpc.ModelInfo{ID: "m", Provider: "p"}
	m.stats = rpc.SessionStats{
		UserMessages: 2, AssistantMessages: 3, ToolCalls: 4, TotalMessages: 5,
		Tokens:       rpc.TokenTotals{Input: 1000, Output: 2000, Total: 3000},
		Cost:         0.0123,
		ContextUsage: &rpc.ContextUsage{Tokens: 3000, ContextWindow: 100000, Percent: 3},
	}
	m.showStatsPanel()
	if m.settingsPanel == nil || m.overlay == nil || m.overlay.action != "stats" {
		t.Fatal("stats panel not open")
	}
	rows := map[string]string{}
	for _, row := range m.settingsPanel.tabs[0].Rows {
		if row.Current != nil {
			rows[row.ID] = row.Current()
		}
	}
	if !strings.Contains(rows["stats-messages"], "2 user") || !strings.Contains(rows["stats-messages"], "4 tool calls") {
		t.Fatalf("messages = %q", rows["stats-messages"])
	}
	if rows["stats-tokens"] != "3.0k" {
		t.Fatalf("tokens = %q", rows["stats-tokens"])
	}
	if !strings.Contains(rows["stats-context"], "3%") {
		t.Fatalf("context = %q", rows["stats-context"])
	}
	if !strings.Contains(rows["stats-cost"], "0.0123") {
		t.Fatalf("cost = %q", rows["stats-cost"])
	}
}

func TestContextUsageAppliesThePieCap(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", t.TempDir())
	m := New(newFakeClient(), Options{})
	m.stats.ContextUsage = &rpc.ContextUsage{Tokens: 175000, ContextWindow: 1000000, Percent: 17.5}
	usage := m.contextUsage()
	if usage == nil || usage.ContextWindow != 350000 {
		t.Fatalf("usage = %+v", usage)
	}
	if usage.Percent < 49.9 || usage.Percent > 50.1 {
		t.Fatalf("percent = %v", usage.Percent)
	}
	if m.stats.ContextUsage.ContextWindow != 1000000 {
		t.Fatal("native usage was modified")
	}
}
