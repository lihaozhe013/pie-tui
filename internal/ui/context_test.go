package ui

import (
	"strings"
	"testing"
)

func TestContextPanelShowsSnapshot(t *testing.T) {
	m := New(newFakeClient(), Options{})
	m.bridgeReady = true
	m.toolSnapshot = &toolSnapshot{
		WorkMode:     "plan",
		SystemPrompt: "You are pi.\n\nSecond section.",
		Tools: []snapshotTool{
			{Name: "read", Description: "Read a file", Active: true},
			{Name: "bash", Description: "Run a command", Active: false},
		},
	}
	m.showContextPanel()
	rows := map[string]string{}
	for _, row := range m.settingsPanel.tabs[0].Rows {
		if row.Current != nil {
			rows[row.ID] = row.Current()
		}
	}
	if !strings.Contains(rows["context-tools"], "1 active") || !strings.Contains(rows["context-tools"], "2 defined") {
		t.Fatalf("tools = %q", rows["context-tools"])
	}
	if !strings.Contains(rows["context-prompt"], "3 lines") {
		t.Fatalf("prompt = %q", rows["context-prompt"])
	}
	if rows["context-mode"] != "plan" {
		t.Fatalf("mode = %q", rows["context-mode"])
	}
	if rows["tool-read"] != "active" || rows["tool-bash"] != "inactive" {
		t.Fatalf("tool states = %v", rows)
	}
}

func TestContextPanelWithoutBridge(t *testing.T) {
	m := New(newFakeClient(), Options{})
	m.showContextPanel()
	if m.settingsPanel == nil {
		t.Fatal("panel missing")
	}
	found := false
	for _, row := range m.settingsPanel.tabs[0].Rows {
		if row.ID == "context-bridge" && row.Current != nil && row.Current() == "unavailable" {
			found = true
		}
	}
	if !found {
		t.Fatal("bridge status missing")
	}
}

func TestSnapshotMsgPopulatesTools(t *testing.T) {
	m := New(newFakeClient(), Options{})
	snapshot := toolSnapshot{Tools: []snapshotTool{{Name: "read", Active: true}}}
	model, _ := m.Update(snapshotMsg{snapshot: snapshot})
	m = model.(*Model)
	if m.toolSnapshot == nil || len(m.toolSnapshot.Tools) != 1 {
		t.Fatalf("snapshot = %+v", m.toolSnapshot)
	}
}
