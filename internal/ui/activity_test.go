package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/lihaozhe/pie-tui/internal/rpc"
)

func TestModelActivityBarTracksRun(t *testing.T) {
	t.Parallel()

	model, _ := newTestModel(t)
	model, _ = step(t, model, eventMsg{event: &rpc.AgentStartEvent{}})
	if got := model.activityLabel(); got != "Waiting for provider…" {
		t.Fatalf("after agent_start label = %q", got)
	}
	view := ansi.Strip(model.View())
	if !strings.Contains(view, "Waiting for provider…") {
		t.Fatalf("activity bar missing from view:\n%s", view)
	}
	if !strings.Contains(view, "Elapsed 0s") {
		t.Fatalf("elapsed clock missing from activity bar:\n%s", view)
	}

	model, _ = step(t, model, eventMsg{event: &rpc.MessageUpdateEvent{
		Assistant: rpc.AssistantMessageEvent{Type: "thinking_delta", ContentIndex: 0, Delta: "hmm"},
	}})
	if got := model.activityLabel(); got != "Thinking…" {
		t.Fatalf("thinking label = %q", got)
	}

	model, _ = step(t, model, eventMsg{event: &rpc.ToolExecutionStartEvent{
		ToolCallID: "call_1",
		ToolName:   "read",
		Args:       []byte(`{"path":"main.go"}`),
	}})
	if got := model.activityLabel(); got != "Reading main.go" {
		t.Fatalf("tool label = %q", got)
	}
	if view := ansi.Strip(model.View()); !strings.Contains(view, "Reading main.go") {
		t.Fatalf("running tool missing from activity bar:\n%s", view)
	}

	model, _ = step(t, model, eventMsg{event: &rpc.ToolExecutionEndEvent{
		ToolCallID: "call_1",
		ToolName:   "read",
	}})
	if got := model.activityLabel(); got != "Waiting for provider…" {
		t.Fatalf("after tool end label = %q", got)
	}

	model, _ = step(t, model, eventMsg{event: &rpc.AgentSettledEvent{}})
	if got := model.activityLabel(); got != "" {
		t.Fatalf("settled label = %q", got)
	}
	if view := ansi.Strip(model.View()); strings.Contains(view, "Waiting for provider…") {
		t.Fatalf("settled run still in activity bar:\n%s", view)
	}
}

func TestActivityRowKeepsFrameHeightAndSitsAboveEditor(t *testing.T) {
	t.Parallel()

	model, _ := newTestModel(t)
	for _, height := range []int{12, 24, 40} {
		model, _ = step(t, model, tea.WindowSizeMsg{Width: 80, Height: height})
		model, _ = step(t, model, eventMsg{event: &rpc.AgentStartEvent{}})
		model, _ = step(t, model, eventMsg{event: &rpc.ToolExecutionStartEvent{
			ToolCallID: "call_1",
			ToolName:   "read",
			Args:       []byte(`{"path":"main.go"}`),
		}})

		lines := strings.Split(model.View(), "\n")
		if len(lines) != height {
			t.Fatalf("height %d: busy view has %d lines", height, len(lines))
		}
		// The editor is a three-line box plus its hint, so the activity row is
		// the fifth line from the bottom, immediately above the input.
		activity := ansi.Strip(lines[len(lines)-5])
		if !strings.Contains(activity, "Reading main.go") {
			t.Fatalf("height %d: activity row = %q", height, activity)
		}

		model, _ = step(t, model, eventMsg{event: &rpc.AgentSettledEvent{}})
		if got := len(strings.Split(model.View(), "\n")); got != height {
			t.Fatalf("height %d: idle view has %d lines", height, got)
		}
	}
}

func TestActivityMarkerBlinks(t *testing.T) {
	t.Parallel()

	model, _ := newTestModel(t)
	model.busy = true
	model.activity.begin()

	model.frame = 0
	bright := ansi.Strip(model.renderActivity())
	model.frame = 8
	dim := ansi.Strip(model.renderActivity())

	if !strings.HasPrefix(bright, "●") {
		t.Fatalf("bright frame = %q", bright)
	}
	if !strings.HasPrefix(dim, "○") {
		t.Fatalf("dim frame = %q", dim)
	}
}

func TestActivityHiddenDuringTextStreamKeepsSlot(t *testing.T) {
	t.Parallel()

	model, _ := newTestModel(t)
	model, _ = step(t, model, tea.WindowSizeMsg{Width: 80, Height: 24})
	model, _ = step(t, model, eventMsg{event: &rpc.AgentStartEvent{}})
	model, _ = step(t, model, eventMsg{event: &rpc.MessageUpdateEvent{
		Assistant: rpc.AssistantMessageEvent{Type: "text_delta", ContentIndex: 0, Delta: "hello"},
	}})

	// The GUI hides the row's content while the model writes plain text, but
	// keeps the slot so the composer never moves.
	if got := model.activityLabel(); got != "" {
		t.Fatalf("text stream label = %q", got)
	}
	if got := model.activityRows(); got != 1 {
		t.Fatalf("text stream rows = %d, want 1", got)
	}
	if got := model.renderActivity(); got != "" {
		t.Fatalf("text stream render = %q", got)
	}
	lines := strings.Split(model.View(), "\n")
	if len(lines) != 24 {
		t.Fatalf("text stream view has %d lines, want 24", len(lines))
	}
	if activity := ansi.Strip(lines[len(lines)-5]); activity != "" {
		t.Fatalf("reserved activity row = %q, want blank", activity)
	}
}

func TestActivityCompactionAndRetryLabels(t *testing.T) {
	t.Parallel()

	model, _ := newTestModel(t)
	model, _ = step(t, model, eventMsg{event: &rpc.CompactionStartEvent{Reason: "manual"}})
	if got := model.activityLabel(); got != "Compacting context…" {
		t.Fatalf("compaction label = %q", got)
	}
	model, _ = step(t, model, eventMsg{event: &rpc.CompactionEndEvent{Reason: "manual"}})
	if got := model.activityLabel(); got != "" {
		t.Fatalf("label after manual compaction = %q", got)
	}

	model, _ = step(t, model, eventMsg{event: &rpc.AgentStartEvent{}})
	model, _ = step(t, model, eventMsg{event: &rpc.AutoRetryStartEvent{Attempt: 2, MaxAttempts: 5}})
	if got := model.activityLabel(); got != "Retrying 2/5…" {
		t.Fatalf("retry label = %q", got)
	}
	model, _ = step(t, model, eventMsg{event: &rpc.AutoRetryEndEvent{Success: true}})
	if got := model.activityLabel(); got != "Waiting for provider…" {
		t.Fatalf("label after retry = %q", got)
	}
}

func TestActivityToolLabels(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		tool   activityTool
		expect string
	}{
		{"read", activityTool{name: "read", args: map[string]any{"path": "main.go"}}, "Reading main.go"},
		{"namespaced read", activityTool{name: "fs.read_file", args: map[string]any{"path": "a.go"}}, "Reading a.go"},
		{"read without path", activityTool{name: "read"}, "Reading…"},
		{"grep", activityTool{name: "grep", args: map[string]any{"pattern": "TODO"}}, `Searching "TODO"`},
		{"grep in path", activityTool{name: "grep", args: map[string]any{"pattern": "TODO", "path": "internal"}}, `Searching "TODO" in internal`},
		{"list", activityTool{name: "ls", args: map[string]any{"path": "internal"}}, "Listing internal"},
		{"bash", activityTool{name: "bash", args: map[string]any{"command": "go test ./...  "}}, "Running go test ./..."},
		{"bash without command", activityTool{name: "bash"}, "Running bash..."},
		{"edit", activityTool{name: "edit", args: map[string]any{"path": "main.go"}}, "Running edit..."},
		{"write", activityTool{name: "write", args: map[string]any{"path": "main.go"}}, "Running write..."},
		{"unknown", activityTool{name: "custom_tool"}, "Running custom_tool..."},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := activityToolLabel(tc.tool); got != tc.expect {
				t.Fatalf("label = %q, want %q", got, tc.expect)
			}
		})
	}

	grouped := activityToolsLabel([]activityTool{
		{callID: "1", name: "read", args: map[string]any{"path": "a.go"}},
		{callID: "2", name: "read", args: map[string]any{"path": "b.go"}},
		{callID: "3", name: "bash", args: map[string]any{"command": "go test ./..."}},
	})
	if grouped != "Reading 2 files · Running go test ./..." {
		t.Fatalf("grouped label = %q", grouped)
	}
}

func TestActivityTokenEstimateAndRate(t *testing.T) {
	t.Parallel()

	model, _ := newTestModel(t)
	model, _ = step(t, model, eventMsg{event: &rpc.AgentStartEvent{}})
	model, _ = step(t, model, eventMsg{event: &rpc.MessageUpdateEvent{
		Assistant: rpc.AssistantMessageEvent{Type: "thinking_delta", ContentIndex: 0, Delta: "abcdefgh"},
	}})

	// Eight characters at four chars per token: the "~N tokens" beside label.
	tokens, ok := model.activityTokenEstimate()
	if !ok || tokens != 2 {
		t.Fatalf("token estimate = %v, %v; want 2, true", tokens, ok)
	}
	if view := ansi.Strip(model.View()); !strings.Contains(view, "~2 tokens") {
		t.Fatalf("token estimate missing from view:\n%s", view)
	}

	// Two tokens two seconds after the anchor: 1.0 t/s.
	model.activity.rateAt = time.Now().Add(-2 * time.Second)
	model.activity.rateTokens = 0
	rate, ok := model.activityTokensPerSecond(time.Now())
	if !ok || rate != "1.0 t/s" {
		t.Fatalf("rate = %q, %v; want \"1.0 t/s\", true", rate, ok)
	}
	if view := ansi.Strip(model.View()); !strings.Contains(view, "1.0 t/s") {
		t.Fatalf("rate missing from view:\n%s", view)
	}
}

func TestActivityLabelEllipsisKeepsMeta(t *testing.T) {
	t.Parallel()

	model, _ := newTestModel(t)
	model, _ = step(t, model, tea.WindowSizeMsg{Width: 48, Height: 24})
	model, _ = step(t, model, eventMsg{event: &rpc.AgentStartEvent{}})
	model, _ = step(t, model, eventMsg{event: &rpc.ToolExecutionStartEvent{
		ToolCallID: "call_1",
		ToolName:   "bash",
		Args:       []byte(`{"command":"` + strings.Repeat("x", 200) + `"}`),
	}})

	line := ansi.Strip(model.renderActivity())
	if width := lipglossWidth(line); width > 48 {
		t.Fatalf("activity line is %d cells wide: %q", width, line)
	}
	if !strings.Contains(line, "Elapsed") {
		t.Fatalf("meta lost to the label: %q", line)
	}
}

func TestFormatTurnDuration(t *testing.T) {
	t.Parallel()

	cases := map[time.Duration]string{
		1500 * time.Millisecond:     "1s",
		65 * time.Second:            "1m 5s",
		2*time.Hour + 7*time.Minute: "2h 7m 0s",
	}
	for duration, want := range cases {
		if got := formatTurnDuration(duration); got != want {
			t.Fatalf("formatTurnDuration(%s) = %q, want %q", duration, got, want)
		}
	}
}
