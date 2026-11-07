package rpc

import (
	"encoding/json"
	"reflect"
	"testing"
)

func encode(t *testing.T, command Command) map[string]any {
	t.Helper()
	payload, err := json.Marshal(command)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return decoded
}

func TestCommandBuilders(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		command Command
		want    map[string]any
	}{
		{"prompt", Prompt("hi"), map[string]any{"type": "prompt", "message": "hi"}},
		{"steer", Steer("go left"), map[string]any{"type": "steer", "message": "go left"}},
		{"follow up", FollowUp("later"), map[string]any{"type": "follow_up", "message": "later"}},
		{"aborter", Abort(), map[string]any{"type": "abort"}},
		{"clear queue", ClearQueue(), map[string]any{"type": "clear_queue"}},
		{"set model", SetModel("anthropic", "claude"), map[string]any{"type": "set_model", "provider": "anthropic", "modelId": "claude"}},
		{"set thinking", SetThinkingLevel("high"), map[string]any{"type": "set_thinking_level", "level": "high"}},
		{"compact plain", Compact(""), map[string]any{"type": "compact"}},
		{"compact custom", Compact("keep tests"), map[string]any{"type": "compact", "customInstructions": "keep tests"}},
		{"bash", Bash("ls -la", true), map[string]any{"type": "bash", "command": "ls -la", "excludeFromContext": true}},
		{"switch session", SwitchSession("/tmp/s.jsonl"), map[string]any{"type": "switch_session", "sessionPath": "/tmp/s.jsonl"}},
		{"entries without cursor", GetEntries(""), map[string]any{"type": "get_entries"}},
		{"fork", Fork("abc"), map[string]any{"type": "fork", "entryId": "abc"}},
		{"ui value", ExtensionUIValue("u1", "Yes"), map[string]any{"type": "extension_ui_response", "id": "u1", "value": "Yes"}},
		{"ui confirm", ExtensionUIConfirmed("u2", true), map[string]any{"type": "extension_ui_response", "id": "u2", "confirmed": true}},
		{"ui cancel", ExtensionUICancelled("u3"), map[string]any{"type": "extension_ui_response", "id": "u3", "cancelled": true}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := encode(t, tc.command); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPromptWithImages(t *testing.T) {
	t.Parallel()

	command := encode(t, PromptWithImages("what is this", []Image{
		{Type: "image", Data: "AAA", MimeType: "image/png"},
	}))
	images, ok := command["images"].([]any)
	if !ok || len(images) != 1 {
		t.Fatalf("images = %v", command["images"])
	}
	first, _ := images[0].(map[string]any)
	if first["mimeType"] != "image/png" {
		t.Fatalf("image = %v", first)
	}

	// No images means no key at all, so the wire stays minimal.
	if _, present := encode(t, PromptWithImages("plain", nil))["images"]; present {
		t.Fatal("unexpected images key")
	}
}

func TestResponsePayloadDecoding(t *testing.T) {
	t.Parallel()

	response := &Response{
		Success: true,
		Data:    json.RawMessage(`{"model":{"id":"m1","name":"Model","provider":"p"},"isStreaming":true,"thinkingLevel":"low","messageCount":3}`),
	}

	state, err := DecodeTyped[State](response)
	if err != nil {
		t.Fatalf("decode state: %v", err)
	}
	if state.Model == nil || state.Model.Label() != "p/m1" || !state.IsStreaming || state.MessageCount != 3 {
		t.Fatalf("state = %+v", state)
	}

	empty, err := DecodeTyped[State](&Response{})
	if err != nil {
		t.Fatalf("decode empty: %v", err)
	}
	if empty.Model != nil {
		t.Fatalf("expected empty state, got %+v", empty)
	}

	stats, err := DecodeTyped[SessionStats](&Response{Data: json.RawMessage(`{"tokens":{"total":10},"contextUsage":{"tokens":5,"contextWindow":100,"percent":5}}`)})
	if err != nil {
		t.Fatalf("decode stats: %v", err)
	}
	if stats.Tokens.Total != 10 || stats.ContextUsage == nil || stats.ContextUsage.Percent != 5 {
		t.Fatalf("stats = %+v", stats)
	}
}
