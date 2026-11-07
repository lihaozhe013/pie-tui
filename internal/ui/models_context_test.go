package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContextWindowOverrideEditor(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", dir)
	path := filepath.Join(dir, "models.json")
	if err := os.WriteFile(path, []byte(`{"providers":{"p":{"models":[{"id":"m","contextWindow":1000}],"modelOverrides":{"m":{"maxTokens":77}}}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store := filepath.Join(dir, "models-store.json")
	storeData := `{"providers":{"p":{"name":"Provider P","models":[{"id":"m","name":"Model M","contextWindow":1000}]}}}`
	if err := os.WriteFile(store, []byte(storeData), 0o600); err != nil {
		t.Fatal(err)
	}
	m := New(newFakeClient(), Options{CWD: dir})
	if m.catalog == nil || m.catalog.Provider("p") == nil {
		t.Skip("catalog fixture did not load")
	}
	m.openContextProviderPanel("p")
	var row settingsRow
	for _, candidate := range m.settingsPanel.tabs[0].Rows {
		if candidate.ID == "context-m" {
			row = candidate
		}
	}
	if row.Apply == nil || row.Current() != "default" {
		t.Fatalf("row = %+v current=%q", row, row.Current())
	}
	if err := row.Apply("200000"); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if row.Current() != "200.0k" {
		t.Fatalf("current = %q", row.Current())
	}
	var parsed map[string]any
	data, _ := os.ReadFile(path)
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	overrides := parsed["providers"].(map[string]any)["p"].(map[string]any)["modelOverrides"].(map[string]any)
	entry := overrides["m"].(map[string]any)
	if entry["contextWindow"].(float64) != 200000 || entry["maxTokens"].(float64) != 77 {
		t.Fatalf("overrides = %v", entry)
	}
	if err := row.Apply(""); err != nil {
		t.Fatalf("clear: %v", err)
	}
	data, _ = os.ReadFile(path)
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	overrides = parsed["providers"].(map[string]any)["p"].(map[string]any)["modelOverrides"].(map[string]any)
	entry = overrides["m"].(map[string]any)
	if _, ok := entry["contextWindow"]; ok || entry["maxTokens"].(float64) != 77 {
		t.Fatalf("cleared overrides = %v", entry)
	}
}

func TestModelsTabShowsProjectShadow(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", t.TempDir())
	if err := os.MkdirAll(filepath.Join(dir, ".pi"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".pi", "settings.json"), []byte(`{"enabledModels":["p/m"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	m := New(newFakeClient(), Options{CWD: dir})
	tab := m.modelsSettingsTab()
	var scope string
	for _, row := range tab.Rows {
		if row.ID == "project-scope" {
			scope = row.Current()
		}
	}
	if !strings.Contains(scope, "enabledModels") || !strings.Contains(scope, "project shadows") {
		t.Fatalf("scope = %q", scope)
	}
}
