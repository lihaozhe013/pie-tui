package ui

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lihaozhe/pie-tui/internal/config"
	"github.com/lihaozhe/pie-tui/internal/session"
)

func goalEntryForTest(t *testing.T, status, objective string) session.LogEntry {
	t.Helper()
	data := `{"version":1,"kind":"set","goal":{"goalId":"g1","objective":"` + objective + `","status":"` + status + `","usage":{"tokensUsed":1234}}}`
	if !json.Valid([]byte(data)) {
		t.Fatal("bad fixture")
	}
	return session.LogEntry{Type: "custom", CustomType: session.GoalEntryType, Data: json.RawMessage(data)}
}

func TestGoalToggleRequiresThePackage(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", dir)

	m := New(newFakeClient(), Options{})
	m.goalAvailable = false

	var toggle settingsRow
	for _, row := range m.generalSettingsTab().Rows {
		if row.ID == "goal-enabled" {
			toggle = row
		}
	}
	if toggle.SetBool == nil {
		t.Fatal("goal row missing")
	}
	if err := toggle.SetBool(true); err == nil || !strings.Contains(err.Error(), "pi install npm:pi-better-goal") {
		t.Fatalf("enable without the package = %v", err)
	}

	prefs, err := config.LoadTUIConfig(m.paths)
	if err != nil {
		t.Fatalf("prefs: %v", err)
	}
	if prefs.GoalEnabled() {
		t.Fatal("partial state written")
	}

	m.goalAvailable = true
	if err := toggle.SetBool(true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if !m.goalEnabled {
		t.Fatal("model not updated")
	}
	prefs, _ = config.LoadTUIConfig(m.paths)
	if !prefs.GoalEnabled() {
		t.Fatal("preference not saved")
	}
}

func TestGoalPanelReflectsStatus(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", dir)

	m := New(newFakeClient(), Options{})
	m.goalEnabled = true
	m.goalAvailable = false
	panel := m.goalPanel()
	if !strings.Contains(panel.tabs[0].Rows[1].Current(), "pi install npm:pi-better-goal") {
		t.Fatalf("install hint missing: %+v", panel.tabs[0].Rows)
	}

	m.goalAvailable = true
	m.goal, m.hasGoal = session.ReadGoal([]session.LogEntry{goalEntryForTest(t, "paused", "ship parity")})
	panel = m.goalPanel()
	ids := map[string]bool{}
	for _, row := range panel.tabs[0].Rows {
		ids[row.ID] = true
	}
	for _, want := range []string{"goal-status", "goal-resume", "goal-complete", "goal-clear"} {
		if !ids[want] {
			t.Fatalf("row %q missing from %v", want, ids)
		}
	}
	if ids["goal-pause"] {
		t.Fatal("pause shown for a paused goal")
	}
	if got := m.goalStatusSegment(); got != "goal:paused" {
		t.Fatalf("header = %q", got)
	}
}

func TestSettingsLeavesTheGoalPanel(t *testing.T) {
	m := New(newFakeClient(), Options{})
	m.goalAvailable = true
	m.goalEnabled = true
	m.openGoalPanel()
	if m.settingsPanel == nil || !m.settingsPanel.single {
		t.Fatal("goal panel not open")
	}
	m.openSettings()
	if m.settingsPanel.single {
		t.Fatal("main settings replaced by the goal panel")
	}
}

func TestEntriesMsgRestoresGoalAndSelection(t *testing.T) {
	m := New(newFakeClient(), Options{})
	m.settingsPanel = newSettingsState(m)

	entries := []session.LogEntry{
		goalEntryForTest(t, "active", "parity"),
		{Type: "custom", CustomType: session.WorkModeEntryType, Data: json.RawMessage(`{"version":1,"mode":"plan"}`)},
	}
	model, _ := m.Update(entriesMsg{entries: entries})
	m = model.(*Model)
	if !m.hasGoal || m.goal.Objective != "parity" || m.currentMode != "plan" {
		t.Fatalf("entries not applied: goal=%+v mode=%q", m.goal, m.currentMode)
	}
}
