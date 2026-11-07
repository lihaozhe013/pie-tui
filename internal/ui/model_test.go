package ui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/lihaozhe/pie-tui/internal/rpc"
)

// fakeClient records commands and replays canned responses.
type fakeClient struct {
	mu         sync.Mutex
	commands   []rpc.Command
	sent       []rpc.Command
	events     chan rpc.Event
	responders map[string]func(rpc.Command) *rpc.Response
}

func newFakeClient() *fakeClient {
	return &fakeClient{
		events:     make(chan rpc.Event, 64),
		responders: map[string]func(rpc.Command) *rpc.Response{},
	}
}

func (f *fakeClient) respond(kind string, responder func(rpc.Command) *rpc.Response) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.responders[kind] = responder
}

func (f *fakeClient) Do(_ context.Context, command rpc.Command) (*rpc.Response, error) {
	kind, _ := command["type"].(string)

	f.mu.Lock()
	f.commands = append(f.commands, command)
	responder := f.responders[kind]
	f.mu.Unlock()

	if responder != nil {
		return responder(command), nil
	}
	return &rpc.Response{Command: kind, Success: true}, nil
}

func (f *fakeClient) DoOK(ctx context.Context, command rpc.Command) (*rpc.Response, error) {
	response, err := f.Do(ctx, command)
	if err != nil {
		return nil, err
	}
	if !response.Success {
		return response, fmt.Errorf("command %s failed: %s", response.Command, response.Error)
	}
	return response, nil
}

func (f *fakeClient) Send(command rpc.Command) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, command)
	return nil
}

func (f *fakeClient) Events() <-chan rpc.Event { return f.events }

func (f *fakeClient) Done() <-chan struct{} { return make(chan struct{}) }

func (f *fakeClient) Err() error { return nil }

func (f *fakeClient) StderrTail() []string { return nil }

func (f *fakeClient) lastCommand(kind string) (rpc.Command, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.commands) - 1; i >= 0; i-- {
		if f.commands[i]["type"] == kind {
			return f.commands[i], true
		}
	}
	return nil, false
}

func (f *fakeClient) countCommand(kind string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	count := 0
	for _, command := range f.commands {
		if command["type"] == kind {
			count++
		}
	}
	return count
}

func (f *fakeClient) lastSent(kind string) (rpc.Command, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.sent) - 1; i >= 0; i-- {
		if f.sent[i]["type"] == kind {
			return f.sent[i], true
		}
	}
	return nil, false
}

func step(t *testing.T, m *Model, msg tea.Msg) (*Model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(msg)
	if updated == nil {
		t.Fatal("Update returned a nil model")
	}
	return updated.(*Model), cmd
}

// runCmd executes a command tree and returns the produced messages.
func runCmd(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}
	var out []tea.Msg
	for _, nested := range batch {
		out = append(out, runCmd(t, nested)...)
	}
	return out
}

func keyMsg(name string) tea.KeyMsg {
	switch name {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "ctrl+k":
		return tea.KeyMsg{Type: tea.KeyCtrlK}
	case "ctrl+o":
		return tea.KeyMsg{Type: tea.KeyCtrlO}
	case "ctrl+t":
		return tea.KeyMsg{Type: tea.KeyCtrlT}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(name)}
	}
}

func newTestModel(t *testing.T) (*Model, *fakeClient) {
	t.Helper()

	client := newFakeClient()
	client.respond("get_state", func(rpc.Command) *rpc.Response {
		return &rpc.Response{
			Command: "get_state",
			Success: true,
			Data:    []byte(`{"model":{"id":"m1","name":"Model One","provider":"prov"},"thinkingLevel":"high","sessionName":"demo"}`),
		}
	})
	client.respond("get_available_models", func(rpc.Command) *rpc.Response {
		return &rpc.Response{
			Command: "get_available_models",
			Success: true,
			Data:    []byte(`{"models":[{"id":"m1","name":"Model One","provider":"prov"},{"id":"m2","name":"Model Two","provider":"prov"}]}`),
		}
	})
	client.respond("get_available_thinking_levels", func(rpc.Command) *rpc.Response {
		return &rpc.Response{Command: "get_available_thinking_levels", Success: true, Data: []byte(`{"levels":["off","low","high"]}`)}
	})
	client.respond("get_commands", func(rpc.Command) *rpc.Response {
		return &rpc.Response{Command: "get_commands", Success: true, Data: []byte(`{"commands":[{"name":"fix-tests","description":"Fix failing tests","source":"prompt"}]}`)}
	})

	model := New(client, Options{Version: "test", CWD: "/work/dir"})
	model, _ = step(t, model, tea.WindowSizeMsg{Width: 100, Height: 30})
	model, _ = step(t, model, model.handshake()())
	return model, client
}

func TestModelRendersHeader(t *testing.T) {
	t.Parallel()

	model, _ := newTestModel(t)
	view := ansi.Strip(model.View())

	for _, want := range []string{"pie-tui", "prov/m1", "think:high", "demo"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func TestModelSubmitsPrompt(t *testing.T) {
	t.Parallel()

	model, client := newTestModel(t)
	model.composer.SetText("hello agent")

	model, cmd := step(t, model, keyMsg("enter"))
	runCmd(t, cmd)

	command, ok := client.lastCommand("prompt")
	if !ok {
		t.Fatal("prompt command was not sent")
	}
	if command["message"] != "hello agent" {
		t.Fatalf("message = %v", command["message"])
	}
	if model.composer.Value() != "" {
		t.Fatalf("composer not cleared: %q", model.composer.Value())
	}
	if view := ansi.Strip(model.View()); !strings.Contains(view, "hello agent") {
		t.Fatalf("user message not rendered:\n%s", view)
	}
}

func TestModelQueuesFollowUpWhileBusy(t *testing.T) {
	t.Parallel()

	model, client := newTestModel(t)
	model, _ = step(t, model, eventMsg{event: &rpc.AgentStartEvent{}})
	model.composer.SetText("more work")

	_, cmd := step(t, model, keyMsg("enter"))
	runCmd(t, cmd)

	if _, ok := client.lastCommand("follow_up"); !ok {
		t.Fatal("expected a follow_up command while streaming")
	}
	if client.countCommand("prompt") != 0 {
		t.Fatal("prompt must not be sent while streaming")
	}
}

func TestModelStreamsAssistantText(t *testing.T) {
	t.Parallel()

	model, _ := newTestModel(t)
	model, _ = step(t, model, eventMsg{event: &rpc.AgentStartEvent{}})

	for _, delta := range []string{"Hello", " from", " pi"} {
		model, _ = step(t, model, eventMsg{event: &rpc.MessageUpdateEvent{
			Assistant: rpc.AssistantMessageEvent{Type: "text_delta", ContentIndex: 0, Delta: delta},
		}})
	}
	model, _ = step(t, model, eventMsg{event: &rpc.MessageEndEvent{
		Message: rpc.Message{Role: "assistant", Content: rpc.Content{{Type: "text", Text: "Hello from pi"}}},
	}})
	model, _ = step(t, model, eventMsg{event: &rpc.AgentSettledEvent{}})

	if model.busy {
		t.Fatal("agent_settled must clear the busy flag")
	}
	assistant := model.transcript.LastAssistant()
	if assistant == nil || assistant.text != "Hello from pi" {
		t.Fatalf("assistant block = %+v", assistant)
	}
	if assistant.streaming {
		t.Fatal("assistant block still marked streaming")
	}
	if view := ansi.Strip(model.View()); !strings.Contains(view, "Hello from pi") {
		t.Fatalf("streamed text not rendered:\n%s", view)
	}
}

func TestModelEscapeAbortsAndRestoresQueue(t *testing.T) {
	t.Parallel()

	model, client := newTestClientWithAbort(t)
	model, _ = step(t, model, eventMsg{event: &rpc.AgentStartEvent{}})

	_, cmd := step(t, model, keyMsg("esc"))
	for _, msg := range runCmd(t, cmd) {
		if _, ok := msg.(abortMsg); ok {
			model, _ = step(t, model, msg)
		}
	}

	if client.countCommand("clear_queue") != 1 {
		t.Fatalf("clear_queue count = %d", client.countCommand("clear_queue"))
	}
	if client.countCommand("abort") != 1 {
		t.Fatalf("abort count = %d", client.countCommand("abort"))
	}
	if got := model.composer.Value(); !strings.Contains(got, "queued steering") {
		t.Fatalf("queued text not restored: %q", got)
	}
}

func newTestClientWithAbort(t *testing.T) (*Model, *fakeClient) {
	t.Helper()

	model, client := newTestModel(t)
	client.respond("clear_queue", func(rpc.Command) *rpc.Response {
		return &rpc.Response{Command: "clear_queue", Success: true, Data: []byte(`{"steering":["queued steering"],"followUp":[]}`)}
	})
	client.respond("abort", func(rpc.Command) *rpc.Response {
		return &rpc.Response{Command: "abort", Success: true}
	})
	return model, client
}

func TestModelToolDiffAndExpansion(t *testing.T) {
	t.Parallel()

	model, _ := newTestModel(t)
	model, _ = step(t, model, eventMsg{event: &rpc.ToolExecutionStartEvent{
		ToolCallID: "call_1",
		ToolName:   "edit",
		Args:       []byte(`{"path":"main.go","edits":[{"oldText":"old line","newText":"new line"}]}`),
	}})
	model, _ = step(t, model, eventMsg{event: &rpc.ToolExecutionEndEvent{
		ToolCallID: "call_1",
		ToolName:   "edit",
		Result:     []byte(`{"content":[{"type":"text","text":"applied"}]}`),
	}})

	block := model.transcript.Tool("call_1")
	if block == nil || block.kind != blockDiff {
		t.Fatalf("diff block = %+v", block)
	}
	if !strings.Contains(block.meta, "+1 -1") {
		t.Fatalf("meta = %q", block.meta)
	}
	if collapsed := ansi.Strip(model.View()); strings.Contains(collapsed, "new line") {
		t.Fatalf("collapsed diff leaked lines:\n%s", collapsed)
	}

	model, _ = step(t, model, keyMsg("ctrl+o"))
	if view := ansi.Strip(model.View()); !strings.Contains(view, "new line") {
		t.Fatalf("expanded diff missing lines:\n%s", view)
	}
}

func TestModelSlashHelpOpensOverlay(t *testing.T) {
	t.Parallel()

	model, client := newTestModel(t)
	model.composer.SetText("/help")

	model, cmd := step(t, model, keyMsg("enter"))
	runCmd(t, cmd)

	if model.overlay == nil {
		t.Fatal("help overlay did not open")
	}
	if client.countCommand("prompt") != 0 {
		t.Fatal("builtin commands must not be forwarded to pi")
	}

	model, _ = step(t, model, keyMsg("esc"))
	if model.overlay != nil {
		t.Fatal("esc must close the overlay")
	}
}

func TestModelModelPickerAppliesSelection(t *testing.T) {
	t.Parallel()

	model, client := newTestModel(t)
	model, _ = step(t, model, keyMsg("ctrl+k"))
	if model.overlay == nil {
		t.Fatal("model picker did not open")
	}

	model.overlay.move(1)
	model, cmd := step(t, model, keyMsg("enter"))
	runCmd(t, cmd)

	command, ok := client.lastCommand("set_model")
	if !ok {
		t.Fatal("set_model was not sent")
	}
	if command["provider"] != "prov" || command["modelId"] != "m2" {
		t.Fatalf("command = %v", command)
	}
}

func TestModelThinkingCycle(t *testing.T) {
	t.Parallel()

	model, client := newTestModel(t)
	client.respond("cycle_thinking_level", func(rpc.Command) *rpc.Response {
		return &rpc.Response{Command: "cycle_thinking_level", Success: true, Data: []byte(`{"level":"low"}`)}
	})

	model, cmd := step(t, model, keyMsg("shift+tab"))
	msgs := runCmd(t, cmd)
	for _, msg := range msgs {
		if _, ok := msg.(responseMsg); ok {
			model, _ = step(t, model, msg)
		}
	}

	if model.state.ThinkingLevel != "low" {
		t.Fatalf("thinking level = %q", model.state.ThinkingLevel)
	}
}

func TestModelExtensionDialogQueue(t *testing.T) {
	t.Parallel()

	model, client := newTestModel(t)

	model, _ = step(t, model, eventMsg{event: &rpc.ExtensionUIRequestEvent{
		ID: "u1", Method: "confirm", Title: "Allow command?", Message: "run rm -rf?",
	}})
	if model.overlay == nil || model.overlay.kind != overlayConfirm {
		t.Fatal("confirm dialog did not open")
	}

	// A second dialog must wait instead of replacing the first.
	model, _ = step(t, model, eventMsg{event: &rpc.ExtensionUIRequestEvent{
		ID: "u2", Method: "select", Title: "Pick", Options: []string{"a", "b"},
	}})
	if len(model.dialogs) != 1 {
		t.Fatalf("queued dialogs = %d", len(model.dialogs))
	}

	model, cmd := step(t, model, keyMsg("y"))
	runCmd(t, cmd)

	sent, ok := client.lastSent("extension_ui_response")
	if !ok || sent["confirmed"] != true || sent["id"] != "u1" {
		t.Fatalf("sent = %v", sent)
	}
	if model.overlay == nil || model.overlay.kind != overlaySelect {
		t.Fatal("queued dialog did not open after the first was answered")
	}
}

func TestModelNotifiesViaStatus(t *testing.T) {
	t.Parallel()

	model, _ := newTestModel(t)
	model, _ = step(t, model, eventMsg{event: &rpc.ExtensionUIRequestEvent{
		ID: "u3", Method: "setStatus", StatusKey: "ext", StatusText: "indexing",
	}})

	if view := ansi.Strip(model.View()); !strings.Contains(view, "indexing") {
		t.Fatalf("status text missing:\n%s", view)
	}

	model, _ = step(t, model, eventMsg{event: &rpc.ExtensionUIRequestEvent{
		ID: "u4", Method: "setStatus", StatusKey: "ext",
	}})
	if _, ok := model.statuses["ext"]; ok {
		t.Fatal("empty statusText must clear the entry")
	}
}

func TestModelDisconnectShowsError(t *testing.T) {
	t.Parallel()

	model, _ := newTestModel(t)
	model, _ = step(t, model, closedMsg{})

	if !model.disconnected {
		t.Fatal("expected disconnected state")
	}
	if view := ansi.Strip(model.View()); !strings.Contains(view, "pi exited") {
		t.Fatalf("disconnect hint missing:\n%s", view)
	}

	// Input must not reach pi after the child is gone.
	model.composer.SetText("ignored")
	_, cmd := step(t, model, keyMsg("enter"))
	if cmd != nil {
		t.Fatal("submit must be inert while disconnected")
	}
}

func TestModelUnknownEventIsIgnored(t *testing.T) {
	t.Parallel()

	model, _ := newTestModel(t)
	model, _ = step(t, model, eventMsg{event: &rpc.UnknownEvent{Type: "future_event", Raw: []byte(`{}`)}})
	if view := ansi.Strip(model.View()); !strings.Contains(view, "pie-tui") {
		t.Fatalf("view broke after unknown event:\n%s", view)
	}
}

func TestModelWorksWithoutModelPickerData(t *testing.T) {
	t.Parallel()

	model, _ := newTestModel(t)
	model.models = nil
	model, _ = step(t, model, keyMsg("ctrl+k"))
	if model.overlay != nil {
		t.Fatal("picker must not open without models")
	}
	if model.toast == "" {
		t.Fatal("expected a toast explaining the missing models")
	}
}
