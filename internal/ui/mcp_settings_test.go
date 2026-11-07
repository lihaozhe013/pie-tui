package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lihaozhe/pie-tui/internal/config"
	"github.com/lihaozhe/pie-tui/internal/mcp"
)

func TestMCPTabListsAndTogglesServers(t *testing.T) {
	root := t.TempDir()
	agent := filepath.Join(root, "agent")
	project := filepath.Join(root, "project")
	t.Setenv("PI_CODING_AGENT_DIR", agent)
	if err := os.MkdirAll(agent, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agent, "mcp.json"), []byte(`{"mcpServers":{"pw":{"command":"playwright-mcp"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(project, ".pi"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".pi", "mcp.json"), []byte(`{"mcpServers":{"web":{"url":"https://example.com/mcp"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	m := New(newFakeClient(), Options{CWD: project})
	tab := m.mcpSettingsTab()
	var row *settingsRow
	for i := range tab.Rows {
		if strings.HasSuffix(tab.Rows[i].ID, "global-pw") {
			row = &tab.Rows[i]
		}
	}
	if row == nil {
		t.Fatalf("server row missing: %+v", tab.Rows)
	}
	if !strings.Contains(row.Current(), "stdio: playwright-mcp") {
		t.Fatalf("row = %q", row.Current())
	}
	row.Run(m, nil)

	var toggle *settingsRow
	for i := range m.settingsPanel.tabs[0].Rows {
		if m.settingsPanel.tabs[0].Rows[i].ID == "mcp-enabled" {
			toggle = &m.settingsPanel.tabs[0].Rows[i]
		}
	}
	if toggle == nil || toggle.SetBool == nil {
		t.Fatal("enabled row missing")
	}
	if err := toggle.SetBool(false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	file, _ := config.LoadMCP(m.paths)
	server, ok := file.Find("pw")
	if !ok || server.Enabled {
		t.Fatalf("server = %+v, %v", server, ok)
	}

	m.mcpSnapshot = mcp.Snapshot{Servers: []mcp.Status{{Name: "pw", State: "connected", Tools: []string{"click"}}}}
	tab = m.mcpSettingsTab()
	found := false
	for _, row := range tab.Rows {
		if strings.HasSuffix(row.ID, "global-pw") && strings.Contains(row.Current(), "connected") {
			found = true
		}
	}
	if !found {
		t.Fatalf("status not shown: %+v", tab.Rows)
	}
}
