package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lihaozhe/pie-tui/internal/config"
)

func writeAgentFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestProviderAndModelDraftsWriteModelsJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", dir)

	m := New(newFakeClient(), Options{})

	provider := &providerDraft{
		ID:      "local",
		IsNew:   true,
		Custom:  true,
		Name:    "Local",
		BaseURL: "http://localhost:11434/v1",
		API:     "openai-completions",
		APIKey:  "ollama",
	}
	if err := m.saveProviderDraft(provider); err != nil {
		t.Fatalf("save provider: %v", err)
	}
	m.reloadConfig()
	if m.catalog.Provider("local") == nil {
		t.Fatalf("provider missing from catalog: %+v", m.catalog.Providers)
	}

	high := "high"
	model := &modelDraft{
		ProviderID:     "local",
		ID:             "qwen",
		IsNew:          true,
		Name:           "Qwen",
		ContextWindow:  8000,
		MaxTokens:      2000,
		Reasoning:      true,
		CostInput:      0.1,
		CostOutput:     0.2,
		CostCacheRead:  0.01,
		CostCacheWrite: 0.02,
		Levels:         map[string]*string{"high": &high, "max": nil},
	}
	if err := m.saveModelDraft(model); err != nil {
		t.Fatalf("save model: %v", err)
	}

	file, err := config.LoadModels(m.paths)
	if err != nil {
		t.Fatalf("load models: %v", err)
	}
	entry := config.FindModel(file.Provider("local"), "qwen")
	if entry == nil {
		t.Fatalf("model missing: %s", config.Marshal(file.Root()))
	}
	if entry.Get("contextWindow").IntOr(0) != 8000 || entry.Get("maxTokens").IntOr(0) != 2000 {
		t.Fatalf("model = %s", config.Marshal(entry))
	}
	if entry.Get("thinkingLevelMap").Get("high").StringOr("") != "high" {
		t.Fatalf("levels = %s", config.Marshal(entry))
	}
	if !entry.Get("thinkingLevelMap").Get("max").IsNull() {
		t.Fatalf("null level lost: %s", config.Marshal(entry))
	}

	if err := m.deleteModel("local", "qwen"); err != nil {
		t.Fatalf("delete model: %v", err)
	}
	if err := m.deleteProvider("local"); err != nil {
		t.Fatalf("delete provider: %v", err)
	}
	file, _ = config.LoadModels(m.paths)
	if file.Provider("local") != nil {
		t.Fatal("provider not deleted")
	}
}

func TestProviderEditorWritesBuiltinKeyToAuth(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", dir)
	writeAgentFile(t, dir, "models-store.json", `{"builtin":{"models":[{"id":"m"}]}}`)

	m := New(newFakeClient(), Options{})
	m.openProviderEditor("builtin", false)
	if m.settingsPanel == nil {
		t.Fatal("editor not open")
	}

	var keyRow settingsRow
	for _, row := range m.settingsPanel.tabs[0].Rows {
		if row.ID == "provider-apikey" {
			keyRow = row
		}
	}
	if keyRow.Apply == nil {
		t.Fatal("API key row missing")
	}
	if err := keyRow.Apply("sk-live"); err != nil {
		t.Fatalf("apply key: %v", err)
	}
	auth, err := config.LoadAuth(m.paths)
	if err != nil {
		t.Fatalf("auth: %v", err)
	}
	if auth.Type("builtin") != "api_key" {
		t.Fatalf("credential = %q", auth.Type("builtin"))
	}
	if m.catalog.Provider("builtin").CredentialType != "api_key" {
		t.Fatalf("catalog not reloaded: %+v", m.catalog.Provider("builtin"))
	}
}
