package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestToolsTabCodemodeRows(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"codemode":{"mode":"only"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	m := New(newFakeClient(), Options{})
	tab := m.toolsSettingsTab()
	rows := map[string]settingsRow{}
	for _, row := range tab.Rows {
		rows[row.ID] = row
	}
	preference, ok := rows["codemode-preference"]
	if !ok || preference.Apply == nil {
		t.Fatalf("preference row missing: %+v", rows)
	}
	if preference.Current() != "automatic" {
		t.Fatalf("preference = %q", preference.Current())
	}
	if err := preference.Apply("always"); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if preference.Current() != "always" {
		t.Fatalf("preference after apply = %q", preference.Current())
	}

	budget := rows["codemode-budget"]
	if budget.Apply == nil {
		t.Fatal("budget row missing")
	}
	if err := budget.Apply("5000"); err != nil {
		t.Fatalf("budget: %v", err)
	}
	if err := budget.Apply(""); err != nil {
		t.Fatalf("clear budget: %v", err)
	}
	if err := budget.Apply("1000001"); err == nil {
		t.Fatal("out-of-range budget accepted")
	}

	mode := rows["codemode-mode"]
	if mode.Current() != "only" {
		t.Fatalf("mode = %q", mode.Current())
	}
	if err := mode.Apply("on"); err != nil {
		t.Fatalf("mode apply: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "settings.json"))
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("settings: %v", err)
	}
	tools, _ := parsed["defaultTools"].([]any)
	if len(tools) != 1 || tools[0] != "+codemode" {
		t.Fatalf("defaultTools = %v", parsed["defaultTools"])
	}
	codemode, _ := parsed["codemode"].(map[string]any)
	if len(codemode) != 0 {
		t.Fatalf("codemode = %v", codemode)
	}
}
