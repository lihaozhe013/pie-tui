package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lihaozhe/pie-tui/internal/rpc"
)

func TestModelPickerHonorsEnabledModels(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, "models-store.json"), []byte(`{"p":{"models":[{"id":"a"},{"id":"b"}]}}`), 0o600); err != nil {
		t.Fatalf("store: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"enabledModels":["p/a"]}`), 0o600); err != nil {
		t.Fatalf("settings: %v", err)
	}

	m := New(newFakeClient(), Options{})
	m.width, m.height = 100, 30
	m.ready = true
	m.layout()
	m.models = []rpc.ModelInfo{
		{ID: "a", Provider: "p", Name: "A"},
		{ID: "b", Provider: "p", Name: "B"},
	}
	m.reloadCatalog()

	m.openModelPicker()
	if m.overlay == nil {
		t.Fatal("picker not open")
	}
	values := make(map[string]bool)
	for _, item := range m.overlay.items {
		values[item.Value] = true
	}
	if !values["p/a"] || values["p/b"] {
		t.Fatalf("items = %v", values)
	}
}

func TestModelSwitchAppliesSavedThinkingLevel(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"modelThinkingLevels":{"p/a":"low"}}`), 0o600); err != nil {
		t.Fatalf("settings: %v", err)
	}

	client := newFakeClient()
	m := New(client, Options{})
	m.models = []rpc.ModelInfo{{ID: "a", Provider: "p", Name: "A"}}

	msg := m.setModelByLabel("p/a")().(responseMsg)
	if msg.err != nil {
		t.Fatalf("set model: %v", msg.err)
	}
	var sawModel, sawThinking bool
	for _, command := range client.commands {
		switch command["type"] {
		case "set_model":
			sawModel = true
		case "set_thinking_level":
			sawThinking = command["level"] == "low"
		}
	}
	if !sawModel || !sawThinking {
		t.Fatalf("commands = %v", client.commands)
	}
}
