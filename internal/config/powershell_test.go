package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDefaultToolsKeepsShellTools(t *testing.T) {
	if got := ResolveDefaultTools([]string{"read", "-bash", "+powershell"}); len(got) != 2 || got[1] != "powershell" {
		t.Fatalf("tools = %v", got)
	}
	got := ReplaceShellTool([]string{"read", "bash", "powershell", "edit"}, true)
	if len(got) != 3 || got[1] != "powershell" {
		t.Fatalf("replace = %v", got)
	}
}

func TestPowerShellPreferenceOnlyOnWindows(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", dir)
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(`{"defaultTools":["read","powershell","edit"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	paths, _ := ResolvePaths()
	settings, err := LoadSettings(paths)
	if err != nil {
		t.Fatal(err)
	}
	enabled, err := settings.PowerShellEnabled("windows")
	if err != nil || !enabled {
		t.Fatalf("windows = %v %v", enabled, err)
	}
	enabled, err = settings.PowerShellEnabled("darwin")
	if err != nil || enabled {
		t.Fatalf("darwin = %v %v", enabled, err)
	}
	if err := settings.SetPowerShellEnabled(false); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !contains(data, `"bash"`) || contains(data, `"powershell"`) {
		t.Fatalf("settings = %s", data)
	}
}
