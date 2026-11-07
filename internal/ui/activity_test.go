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
	if got := model.activityLabel(); got != "Waiting for model…" {
		t.Fatalf("after agent_start label = %q", got)
	}
	if view := ansi.Strip(model.View()); !strings.Contains(view, "Waiting for model…") {
		t.Fatalf("activity bar missing from view:\n%s", view)
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
	if got := model.activityLabel(); got != "Waiting for model…" {
		t.Fatalf("after tool end label = %q", got)
	}

	model, _ = step(t, model, eventMsg{event: &rpc.AgentSettledEvent{}})
	if got := model.activityLabel(); got != "" {
		t.Fatalf("settled label = %q", got)
	}
	if view := ansi.Strip(model.View()); strings.Contains(view, "Waiting for model…") {
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
	if got := model.activityLabel(); got != "Waiting for model…" {
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
		{"read", activityTool{name: "read", detail: "main.go"}, "Reading main.go"},
		{"namespaced read", activityTool{name: "fs.read_file", detail: "a.go"}, "Reading a.go"},
		{"grep", activityTool{name: "grep", detail: "TODO"}, "Searching for TODO"},
		{"list", activityTool{name: "ls", detail: "internal"}, "Listing internal"},
		{"bash", activityTool{name: "bash", detail: "go test ./..."}, "Running go test ./..."},
		{"edit", activityTool{name: "edit", detail: "main.go"}, "Editing main.go"},
		{"write", activityTool{name: "write", detail: "main.go"}, "Writing main.go"},
		{"unknown", activityTool{name: "custom_tool", detail: "payload"}, "Running custom_tool payload"},
		{"unknown without detail", activityTool{name: "custom_tool"}, "Running custom_tool…"},
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
		{callID: "1", name: "read", detail: "a.go"},
		{callID: "2", name: "read", detail: "b.go"},
		{callID: "3", name: "bash", detail: "go test ./..."},
	})
	if grouped != "Reading 2 files · Running go test ./..." {
		t.Fatalf("grouped label = %q", grouped)
	}
}

func TestFormatElapsed(t *testing.T) {
	t.Parallel()

	cases := map[time.Duration]string{
		1500 * time.Millisecond:     "1s",
		65 * time.Second:            "1m 05s",
		2*time.Hour + 7*time.Minute: "2h 07m",
	}
	for duration, want := range cases {
		if got := formatElapsed(duration); got != want {
			t.Fatalf("formatElapsed(%s) = %q, want %q", duration, got, want)
		}
	}
}
