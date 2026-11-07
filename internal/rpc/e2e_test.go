package rpc

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// The end-to-end test talks to a real pi binary. Enable it with
//
//	PI_TUI_E2E_BIN=$(command -v pi) go test ./internal/rpc -run TestRealPi -v
//
// Set PI_TUI_E2E_PROMPT=1 to also run one minimal model round trip, which
// spends provider tokens.
func realPiClient(t *testing.T) *Client {
	t.Helper()

	bin := os.Getenv("PI_TUI_E2E_BIN")
	if bin == "" {
		t.Skip("set PI_TUI_E2E_BIN to exercise a real pi binary")
	}

	client, err := Start(context.Background(), Options{
		Bin:            bin,
		Args:           []string{"--mode", "rpc", "--no-session"},
		ShutdownGrace:  5 * time.Second,
		TerminateGrace: 3 * time.Second,
		Logf:           t.Logf,
	})
	if err != nil {
		t.Fatalf("start real pi: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestRealPiHandshake(t *testing.T) {
	client := realPiClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	response, err := client.DoOK(ctx, GetState())
	if err != nil {
		t.Fatalf("get_state: %v", err)
	}
	state, err := DecodeTyped[State](response)
	if err != nil {
		t.Fatalf("decode state: %v", err)
	}
	// --no-session makes the session ephemeral, so only the ID is guaranteed.
	if state.SessionID == "" {
		t.Fatalf("expected a session id, got %+v", state)
	}
	t.Logf("state: model=%v thinking=%q messages=%d", state.Model, state.ThinkingLevel, state.MessageCount)

	modelsResponse, err := client.DoOK(ctx, GetAvailableModels())
	if err != nil {
		t.Fatalf("get_available_models: %v", err)
	}
	models, err := DecodeTyped[ModelsPayload](modelsResponse)
	if err != nil {
		t.Fatalf("decode models: %v", err)
	}
	if len(models.Models) == 0 {
		t.Skip("no configured models, skipping the rest")
	}

	levelsResponse, err := client.DoOK(ctx, GetAvailableThinkingLevels())
	if err != nil {
		t.Fatalf("get_available_thinking_levels: %v", err)
	}
	levels, err := DecodeTyped[ThinkingLevelsPayload](levelsResponse)
	if err != nil {
		t.Fatalf("decode levels: %v", err)
	}
	if len(levels.Levels) == 0 {
		t.Fatal("expected at least one thinking level")
	}

	commandsResponse, err := client.DoOK(ctx, GetCommands())
	if err != nil {
		t.Fatalf("get_commands: %v", err)
	}
	for _, command := range mustDecode[CommandsPayload](t, commandsResponse).Commands {
		if command.Name == "" {
			t.Fatalf("command without name: %+v", command)
		}
	}
}

func TestRealPiPromptStreamsEvents(t *testing.T) {
	if os.Getenv("PI_TUI_E2E_PROMPT") != "1" {
		t.Skip("set PI_TUI_E2E_PROMPT=1 to run a model round trip")
	}
	client := realPiClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	if _, err := client.DoOK(ctx, Prompt("Reply with the single word: ok")); err != nil {
		t.Fatalf("prompt: %v", err)
	}

	var (
		sawStart bool
		settled  bool
		text     strings.Builder
		timedOut bool
	)

	deadline := time.After(120 * time.Second)
	for !settled && !timedOut {
		select {
		case event, ok := <-client.Events():
			if !ok {
				t.Fatalf("event stream closed: %v", client.Err())
			}
			switch typed := event.(type) {
			case *AgentStartEvent:
				sawStart = true
			case *MessageUpdateEvent:
				if typed.Assistant.Type == "text_delta" {
					text.WriteString(typed.Assistant.Delta)
				}
			case *AgentSettledEvent:
				settled = true
			}
		case <-deadline:
			timedOut = true
		}
	}

	if timedOut {
		t.Fatalf("timed out, saw start=%v text=%q", sawStart, text.String())
	}
	if !sawStart {
		t.Fatal("missing agent_start")
	}
	if strings.TrimSpace(text.String()) == "" {
		t.Fatal("no assistant text streamed")
	}
	t.Logf("streamed %d bytes: %q", text.Len(), text.String())
}

func mustDecode[T any](t *testing.T, response *Response) T {
	t.Helper()
	value, err := DecodeTyped[T](response)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return value
}
