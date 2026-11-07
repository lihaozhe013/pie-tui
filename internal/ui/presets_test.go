package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lihaozhe/pie-tui/internal/config"
	"github.com/lihaozhe/pie-tui/internal/session"
)

func TestCurrentPresetFromEntries(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", t.TempDir())

	m := New(newFakeClient(), Options{})
	m.presetFile = config.LoadPresets(m.paths)
	m.logEntries = []session.LogEntry{
		{Type: "custom", CustomType: session.ToolSelectionEntryType, Data: []byte(`{"version":1,"tools":["read","grep"],"extensionTools":"all"}`)},
	}
	m.selection, m.hasSelection = session.ReadToolSelection(m.logEntries)
	m.currentMode = session.ReadWorkMode(m.logEntries)
	// Two tools match no built-in preset, so the historical label applies.
	if got := m.currentPresetID(); got != "default" {
		t.Fatalf("current = %q", got)
	}

	m.currentMode = "plan"
	if got := m.currentPresetID(); got != "plan" {
		t.Fatalf("plan current = %q", got)
	}

	m.hasSelection = false
	m.currentMode = ""
	if got := m.currentPresetID(); got != "configured" {
		t.Fatalf("configured = %q", got)
	}
}

func TestApplyPresetSendsBridgeCommands(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", t.TempDir())

	client := newFakeClient()
	m := New(client, Options{})
	m.bridgeReady = true
	m.presetFile = config.LoadPresets(m.paths)

	msg, ok := m.applyPreset("plan")().(presetAppliedMsg)
	if !ok || msg.err != nil || msg.selection != "plan" {
		t.Fatalf("msg = %+v", msg)
	}

	var prompts []string
	for _, command := range client.commands {
		if command["type"] == "prompt" {
			prompts = append(prompts, command["message"].(string))
		}
	}
	if len(prompts) != 2 || !strings.Contains(prompts[0], "/pie-tools all read,grep,find,ls") {
		t.Fatalf("prompts = %v", prompts)
	}
	if !strings.HasSuffix(prompts[1], "/pie-mode plan") {
		t.Fatalf("mode prompt = %q", prompts[1])
	}
}

func TestPresetEditorSavesAndReloads(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", t.TempDir())

	m := New(newFakeClient(), Options{})
	draft := &presetDraft{
		IsNew:  true,
		Preset: config.CustomPreset{Name: "Review", Tools: []string{"read", "grep"}, IncludeExtensions: false},
	}
	if err := m.savePresetDraft(draft); err != nil {
		t.Fatalf("save: %v", err)
	}
	if draft.Preset.ID == "" || draft.IsNew {
		t.Fatalf("draft = %+v", draft)
	}
	file := config.LoadPresets(m.paths)
	if len(file.Presets()) != 1 || file.Presets()[0].Name != "Review" || file.Presets()[0].IncludeExtensions {
		t.Fatalf("presets = %+v", file.Presets())
	}

	if err := m.savePresetDraft(&presetDraft{Preset: config.CustomPreset{Name: "Review", Tools: []string{"read"}}}); err == nil {
		t.Fatal("expected a duplicate-name error")
	}

	if err := m.deletePresetDraft(draft); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(config.LoadPresets(m.paths).Presets()) != 0 {
		t.Fatal("preset not deleted")
	}
}

func TestCtrlGOpensPresetPicker(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", t.TempDir())
	m := New(newFakeClient(), Options{})
	m.width, m.height = 100, 30
	m.ready = true
	m.layout()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	model := updated.(*Model)
	if model.overlay == nil || model.overlay.action != "preset" {
		t.Fatalf("overlay action = %v", overlayAction(model.overlay))
	}
}

func overlayAction(overlay *overlay) string {
	if overlay == nil {
		return ""
	}
	return overlay.action
}
