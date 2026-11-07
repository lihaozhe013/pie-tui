package ui

import (
	"strings"
	"testing"
)

func TestEnterSendsToggleAndComposerKeys(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", t.TempDir())
	model, client := newTestModel(t)
	model.openSettings()
	if model.overlay == nil || model.overlay.kind != overlaySettings {
		t.Fatal("settings did not open")
	}
	model.overlay = nil
	model.settingsPanel = nil

	var toggle *settingsRow
	tab := model.appearanceSettingsTab()
	for i := range tab.Rows {
		if tab.Rows[i].ID == "enter-sends" {
			toggle = &tab.Rows[i]
		}
	}
	if toggle == nil || toggle.SetBool == nil {
		t.Fatal("enter-sends row missing")
	}
	if err := toggle.SetBool(false); err != nil {
		t.Fatalf("toggle: %v", err)
	}
	if model.enterToSend {
		t.Fatal("model flag not updated")
	}

	model.composer.SetText("first line")
	model, _ = step(t, model, keyMsg("enter"))
	if !strings.Contains(model.composer.Value(), "\n") {
		t.Fatalf("enter did not insert a newline: %q", model.composer.Value())
	}
	if _, ok := client.lastCommand("prompt"); ok {
		t.Fatal("enter submitted while enterToSend is off")
	}
	model, cmd := step(t, model, keyMsg("alt+enter"))
	runCmd(t, cmd)
	if _, ok := client.lastCommand("prompt"); !ok {
		t.Fatal("alt+enter did not submit")
	}
}
