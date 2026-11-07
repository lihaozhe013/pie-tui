package modelcatalog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lihaozhe/pie-tui/internal/config"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestLoadMergesStoreModelsAndAuth(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	paths := config.Paths{Dir: dir}
	write(t, filepath.Join(dir, "models-store.json"), `{
  "builtin": {
    "name": "Built In",
    "models": [
      {"id": "one", "name": "One", "reasoning": true, "contextWindow": 1000, "cost": {"input": 1, "output": 2, "cacheRead": 0, "cacheWrite": 0}, "thinkingLevelMap": {"high": "high", "max": null}},
      {"id": "two", "name": "Two"}
    ]
  }
}`)
	write(t, filepath.Join(dir, "models.json"), `{
  "providers": {
    "builtin": {
      "baseUrl": "https://proxy.example",
      "modelOverrides": {
        "two": {"cost": {"input": 3}, "contextWindow": 2000}
      }
    },
    "local": {
      "name": "Local",
      "baseUrl": "http://localhost:11434/v1",
      "api": "openai-completions",
      "apiKey": "ollama",
      "models": [{"id": "qwen", "name": "Qwen", "contextWindow": 8000}]
    }
  }
}`)
	write(t, filepath.Join(dir, "auth.json"), `{"builtin": {"type": "api_key", "key": "x"}}`)

	catalog, err := Load(paths)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(catalog.Providers) != 2 {
		t.Fatalf("providers = %+v", catalog.Providers)
	}
	builtin := catalog.Provider("builtin")
	if builtin == nil || builtin.BaseURL != "https://proxy.example" || builtin.CredentialType != "api_key" {
		t.Fatalf("builtin = %+v", builtin)
	}
	if builtin.Name != "Built In" {
		t.Fatalf("name = %q", builtin.Name)
	}
	two := catalog.Model("builtin/two")
	if two == nil || two.ContextWindow != 2000 || two.Cost.Input != 3 {
		t.Fatalf("two = %+v", two)
	}
	if !two.Override {
		t.Fatal("override not marked")
	}
	local := catalog.Provider("local")
	if local == nil || !local.Custom || local.API != "openai-completions" || len(local.Models) != 1 {
		t.Fatalf("local = %+v", local)
	}
	if !local.Models[0].Custom {
		t.Fatal("custom model not marked")
	}
	one := catalog.Model("builtin/one")
	if !one.SupportsThinkingLevel("high") || one.SupportsThinkingLevel("max") {
		t.Fatalf("thinking map not applied: %+v", one.ThinkingLevelMap)
	}
}

func TestMergeRuntimeFillsNames(t *testing.T) {
	t.Parallel()

	paths := config.Paths{Dir: t.TempDir()}
	write(t, filepath.Join(paths.Dir, "models-store.json"), `{"p": {"models": [{"id": "m"}]}}`)
	catalog, err := Load(paths)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	catalog.MergeRuntime([]Model{{ProviderID: "p", ID: "m", Name: "Runtime Name", ContextWindow: 4096}})
	model := catalog.Model("p/m")
	if model.Name != "Runtime Name" || model.ContextWindow != 4096 {
		t.Fatalf("model = %+v", model)
	}
}
