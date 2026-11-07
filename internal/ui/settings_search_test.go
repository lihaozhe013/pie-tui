package ui

import (
	"strings"
	"testing"
)

func TestSettingsSearchFindsAndJumps(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", t.TempDir())
	m := New(newFakeClient(), Options{})
	m.width, m.height = 100, 30
	m.ready = true
	m.layout()
	m.openSettings()

	m.searchSettings("theme")
	if m.overlay == nil || m.overlay.action != "settings-jump" {
		t.Fatalf("overlay = %+v", m.overlay)
	}
	found := false
	var value string
	for _, item := range m.overlay.items {
		if strings.Contains(item.Label, "Appearance") && strings.Contains(item.Label, "Theme") {
			found = true
			value = item.Value
		}
	}
	if !found {
		t.Fatalf("items = %+v", m.overlay.items)
	}
	m.overlay = nil
	if cmd := m.applySettingsJump(value); cmd != nil {
		t.Fatalf("unexpected command: %v", cmd)
	}
	if m.settingsPanel.currentTab().ID != "appearance" {
		t.Fatalf("tab = %q", m.settingsPanel.currentTab().ID)
	}
	row := m.settingsPanel.currentRow()
	if row == nil || row.ID != "theme" {
		t.Fatalf("row = %+v", row)
	}
}

func TestSettingsSearchNoMatch(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", t.TempDir())
	m := New(newFakeClient(), Options{})
	m.openSettings()
	m.searchSettings("zzzz-not-a-setting")
	if m.overlay != nil && m.overlay.action == "settings-jump" {
		t.Fatal("picker opened for no matches")
	}
}

func TestSettingsTabID(t *testing.T) {
	cases := map[string]string{
		"models": "models", "model": "models", "theme": "appearance",
		"agents": "instructions", "packages": "plugins", "TOOLS": "tools",
	}
	for input, want := range cases {
		got, ok := settingsTabID(input)
		if !ok || got != want {
			t.Fatalf("%q -> %q %v, want %q", input, got, ok, want)
		}
	}
	if _, ok := settingsTabID("nope"); ok {
		t.Fatal("unknown tab accepted")
	}
}
