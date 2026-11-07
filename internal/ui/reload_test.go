package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/lihaozhe/pie-tui/internal/rpc"
)

func TestReloadPiNeedsTheBridge(t *testing.T) {
	m := New(newFakeClient(), Options{})
	if cmd := m.reloadPi(); cmd != nil {
		t.Fatal("reload ran without the bridge")
	}
}

func TestReloadPiSendsTheBridgeCommand(t *testing.T) {
	client := newFakeClient()
	var prompts []string
	client.respond("prompt", func(command rpc.Command) *rpc.Response {
		text, _ := command["message"].(string)
		prompts = append(prompts, text)
		return &rpc.Response{Command: "prompt", Success: true, Data: []byte(`{"handled":true}`)}
	})
	m := New(client, Options{})
	m.bridgeReady = true
	msg, ok := m.reloadPi()().(reloadMsg)
	if !ok || msg.err != nil {
		t.Fatalf("msg = %+v", msg)
	}
	if len(prompts) != 1 || !strings.Contains(prompts[0], "/pie-reload") {
		t.Fatalf("prompts = %v", prompts)
	}
}

func TestPresetAppliedReportsErrors(t *testing.T) {
	m := New(newFakeClient(), Options{})
	updated, _ := m.Update(presetAppliedMsg{selection: "plan"})
	m = updated.(*Model)
	if !strings.Contains(m.toast, "plan") {
		t.Fatalf("toast = %q", m.toast)
	}
	m.transcript.Clear()
	updated, _ = m.Update(presetAppliedMsg{selection: "plan", err: errors.New("bridge down")})
	m = updated.(*Model)
	if len(m.transcript.blocks) != 1 || m.transcript.blocks[0].kind != blockError {
		t.Fatalf("error not reported: %+v", m.transcript.blocks)
	}
}

func TestReloadKeepsSettingsOpen(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", t.TempDir())
	m := New(newFakeClient(), Options{})
	m.openSettings()
	if m.overlay == nil {
		t.Fatal("settings not open")
	}
	updated, _ := m.Update(reloadMsg{})
	m = updated.(*Model)
	if m.overlay == nil {
		t.Fatal("reload closed the settings overlay")
	}
}
