package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lihaozhe/pie-tui/internal/config"
)

func TestPluginsTabDisablesAndEnablesPackages(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"packages":["npm:one",{"source":"npm:filtered","extensions":["x"]}]}`), 0o600); err != nil {
		t.Fatal(err)
	}

	m := New(newFakeClient(), Options{CWD: filepath.Join(dir, "project")})
	tab := m.pluginsSettingsTab()
	var row *settingsRow
	for i := range tab.Rows {
		if strings.HasSuffix(tab.Rows[i].ID, "npm:one") {
			row = &tab.Rows[i]
		}
	}
	if row == nil {
		t.Fatalf("package row missing: %+v", tab.Rows)
	}
	row.Run(m, nil)

	var toggle *settingsRow
	for i := range m.settingsPanel.tabs[0].Rows {
		if m.settingsPanel.tabs[0].Rows[i].ID == "package-enabled" {
			toggle = &m.settingsPanel.tabs[0].Rows[i]
		}
	}
	if toggle == nil {
		t.Fatal("enabled row missing")
	}
	if err := toggle.SetBool(false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if toggle.Bool() {
		t.Fatal("model still enabled")
	}

	settings, err := config.LoadSettingsAt(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	entries, _ := settings.PackageEntries()
	if len(entries) != 2 || !entries[0].Disabled {
		t.Fatalf("entries = %+v", entries)
	}
	if err := settings.SetPackageDisabled("npm:filtered", true); err == nil {
		t.Fatal("filtered package disabled")
	}
}
