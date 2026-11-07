package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackageEntriesAndDisable(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", dir)
	path := filepath.Join(dir, "settings.json")
	content := `{
  // keep this comment
  "packages": [
    "npm:plain",
    {"source": "npm:filtered", "extensions": ["one"]},
    {"source": "npm:off", "extensions": [], "skills": [], "prompts": [], "themes": []},
    {"source": "npm:auto", "autoload": true}
  ],
  "theme": "dark"
}
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	paths, err := ResolvePaths()
	if err != nil {
		t.Fatal(err)
	}
	settings, err := LoadSettings(paths)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	entries, err := settings.PackageEntries()
	if err != nil {
		t.Fatalf("entries: %v", err)
	}
	if len(entries) != 4 {
		t.Fatalf("entries = %+v", entries)
	}
	if entries[0].Disabled || entries[1].Disabled || !entries[1].Filtered || !entries[2].Disabled || entries[3].Disabled {
		t.Fatalf("entries = %+v", entries)
	}

	if err := settings.SetPackageDisabled("npm:filtered", true); err == nil || !strings.Contains(err.Error(), "resource filters") {
		t.Fatalf("filtered disable = %v", err)
	}
	if err := settings.SetPackageDisabled("npm:plain", true); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if err := settings.SetPackageDisabled("npm:off", false); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if err := settings.SetPackageDisabled("npm:nope", true); err == nil {
		t.Fatal("unknown source accepted")
	}

	data, _ := os.ReadFile(path)
	text := string(data)
	if !strings.Contains(text, `"theme": "dark"`) {
		t.Fatalf("unknown keys dropped:\n%s", text)
	}
	if strings.Index(text, `"packages"`) > strings.Index(text, `"theme"`) {
		t.Fatalf("member order changed:\n%s", text)
	}
	if !strings.Contains(text, `"npm:off"`) {
		t.Fatalf("disabled package not re-enabled as a source:\n%s", text)
	}
	if strings.Contains(text, `"extensions": [],`) == false {
		t.Fatalf("plain entry not disabled in place:\n%s", text)
	}

	settings, _ = LoadSettings(paths)
	entries, _ = settings.PackageEntries()
	for _, entry := range entries {
		switch entry.Source {
		case "npm:plain":
			if !entry.Disabled {
				t.Fatalf("plain not disabled: %+v", entry)
			}
		case "npm:auto":
			if entry.Disabled || !entry.Filtered {
				t.Fatalf("autoload entry changed: %+v", entry)
			}
		}
	}
}

func TestLoadSettingsAtProject(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".pi", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"packages":["npm:local"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	settings, err := LoadSettingsAt(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := settings.SetPackageDisabled("npm:local", true); err != nil {
		t.Fatalf("disable: %v", err)
	}
	entries, _ := settings.PackageEntries()
	if len(entries) != 1 || !entries[0].Disabled {
		t.Fatalf("entries = %+v", entries)
	}
}
