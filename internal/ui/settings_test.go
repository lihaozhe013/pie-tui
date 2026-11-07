package ui

import (
	"strings"
	"testing"

	"github.com/lihaozhe/pie-tui/internal/config"
)

func TestSettingsPanelTogglesSharedSettings(t *testing.T) {
	agentDir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", agentDir)

	m := New(newFakeClient(), Options{})
	m.width, m.height = 100, 32
	m.ready = true
	m.layout()

	m.openSettings()
	if m.overlay == nil || m.overlay.kind != overlaySettings {
		t.Fatalf("settings overlay not open: %+v", m.overlay)
	}
	if m.settingsPanel == nil || len(m.settingsPanel.tabs) != 8 {
		t.Fatalf("tabs = %+v", m.settingsPanel)
	}

	// The first row is the cache-miss notice toggle.
	m.settingsPanel.row = 0
	if _, cmd := m.activateSettingsRow(); cmd != nil {
		t.Fatalf("unexpected command")
	}
	settings, err := config.LoadSettings(m.paths)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if on, _ := settings.ShowCacheMissNotices(); !on {
		t.Fatal("cache notices not saved")
	}

	// Toggling again turns it off without touching other keys.
	m.activateSettingsRow()
	settings, _ = config.LoadSettings(m.paths)
	if on, _ := settings.ShowCacheMissNotices(); on {
		t.Fatal("cache notices not cleared")
	}

	if !strings.Contains(m.overlay.View(m.styles), "Settings") {
		t.Fatalf("view = %q", m.overlay.View(m.styles))
	}
}

func TestSettingsPanelRequiresProjectTabs(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", t.TempDir())

	m := New(newFakeClient(), Options{})
	m.width, m.height = 90, 30
	m.ready = true
	m.layout()
	m.openSettings()

	ids := make(map[string]bool)
	for _, tab := range m.settingsPanel.tabs {
		ids[tab.ID] = true
	}
	for _, want := range []string{"general", "instructions", "appearance", "models", "tools", "skills", "plugins", "mcp"} {
		if !ids[want] {
			t.Fatalf("missing tab %q in %v", want, ids)
		}
	}
}

func TestThemeChangePersists(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", t.TempDir())

	m := New(newFakeClient(), Options{})
	m.width, m.height = 100, 30
	m.ready = true
	m.layout()
	m.openSettings()

	// Appearance is the third tab; find the theme choice among its rows.
	m.settingsPanel.active = 2
	m.settingsPanel.row = 0
	var row *settingsRow
	for i := range m.settingsPanel.currentTab().Rows {
		if m.settingsPanel.currentTab().Rows[i].ID == "theme" {
			row = &m.settingsPanel.currentTab().Rows[i]
		}
	}
	if row == nil || row.Kind != settingsChoice {
		t.Fatalf("theme row = %+v", row)
	}
	if err := row.Apply("catppuccin-mocha"); err != nil {
		t.Fatalf("apply: %v", err)
	}
	prefs, err := config.LoadTUIConfig(m.paths)
	if err != nil {
		t.Fatalf("prefs: %v", err)
	}
	if prefs.Theme() != "catppuccin-mocha" {
		t.Fatalf("theme = %q", prefs.Theme())
	}
	if m.styles.theme.Accent.Dark != themePalettes["catppuccin-mocha"].Accent {
		t.Fatalf("theme not applied to styles: %+v", m.styles.theme.Accent)
	}
}
