package rpc

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestParseLineKnownEvents(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		line string
		want func(t *testing.T, ev Event)
	}{
		{
			name: "agent_start",
			line: `{"type":"agent_start"}`,
			want: func(t *testing.T, ev Event) {
				if _, ok := ev.(*AgentStartEvent); !ok {
					t.Fatalf("got %T", ev)
				}
			},
		},
		{
			name: "agent_end",
			line: `{"type":"agent_end","messages":[{"role":"assistant","content":"hi"}],"willRetry":true}`,
			want: func(t *testing.T, ev Event) {
				end, ok := ev.(*AgentEndEvent)
				if !ok {
					t.Fatalf("got %T", ev)
				}
				if !end.WillRetry || len(end.Messages) != 1 || end.Messages[0].Text() != "hi" {
					t.Fatalf("unexpected payload: %+v", end)
				}
			},
		},
		{
			name: "agent_settled",
			line: `{"type":"agent_settled","aborted":true}`,
			want: func(t *testing.T, ev Event) {
				settled := ev.(*AgentSettledEvent)
				if !settled.Aborted {
					t.Fatal("expected aborted")
				}
			},
		},
		{
			name: "message_update",
			line: `{"type":"message_update","usage":{"input":10,"output":2,"totalTokens":12,"cost":{"total":0.01}},"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"Hello "}}`,
			want: func(t *testing.T, ev Event) {
				update, ok := ev.(*MessageUpdateEvent)
				if !ok {
					t.Fatalf("got %T", ev)
				}
				if update.Assistant.Type != "text_delta" || update.Assistant.Delta != "Hello " {
					t.Fatalf("unexpected nested event: %+v", update.Assistant)
				}
				if update.Usage == nil || update.Usage.TotalTokens != 12 {
					t.Fatalf("unexpected usage: %+v", update.Usage)
				}
			},
		},
		{
			name: "message_end with block content",
			line: `{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"done"},{"type":"toolCall","id":"c1","name":"bash","arguments":{"command":"ls"}}],"timestamp":1}}`,
			want: func(t *testing.T, ev Event) {
				end := ev.(*MessageEndEvent)
				if end.Message.Text() != "done" {
					t.Fatalf("text: %q", end.Message.Text())
				}
				tools := end.Message.Tools()
				if len(tools) != 1 || tools[0].Name != "bash" || tools[0].ID != "c1" {
					t.Fatalf("tools: %+v", tools)
				}
			},
		},
		{
			name: "tool_execution_end",
			line: `{"type":"tool_execution_end","toolCallId":"c1","toolName":"bash","result":{"content":[]},"isError":true,"durationMs":12.5}`,
			want: func(t *testing.T, ev Event) {
				end := ev.(*ToolExecutionEndEvent)
				if !end.IsError || end.DurationMs == nil || *end.DurationMs != 12.5 {
					t.Fatalf("unexpected payload: %+v", end)
				}
			},
		},
		{
			name: "queue_update",
			line: `{"type":"queue_update","steering":["a"],"followUp":["b","c"]}`,
			want: func(t *testing.T, ev Event) {
				queue := ev.(*QueueUpdateEvent)
				if len(queue.Steering) != 1 || len(queue.FollowUp) != 2 {
					t.Fatalf("unexpected queues: %+v", queue)
				}
			},
		},
		{
			name: "compaction_end aborted",
			line: `{"type":"compaction_end","reason":"overflow","aborted":true,"willRetry":false}`,
			want: func(t *testing.T, ev Event) {
				end := ev.(*CompactionEndEvent)
				if !end.Aborted || end.Result != nil || end.Reason != "overflow" {
					t.Fatalf("unexpected payload: %+v", end)
				}
			},
		},
		{
			name: "bash_execution_update",
			line: `{"type":"bash_execution_update","id":"req-1","delta":"out\n"}`,
			want: func(t *testing.T, ev Event) {
				update := ev.(*BashExecutionUpdateEvent)
				if update.ID != "req-1" || update.Delta != "out\n" {
					t.Fatalf("unexpected payload: %+v", update)
				}
			},
		},
		{
			name: "extension_ui_request dialog",
			line: `{"type":"extension_ui_request","id":"u1","method":"select","title":"Allow?","options":["Yes","No"],"timeout":10000}`,
			want: func(t *testing.T, ev Event) {
				req := ev.(*ExtensionUIRequestEvent)
				if !req.IsDialog() || len(req.Options) != 2 || req.Timeout != 10000 {
					t.Fatalf("unexpected payload: %+v", req)
				}
			},
		},
		{
			name: "extension_ui_request fire and forget",
			line: `{"type":"extension_ui_request","id":"u2","method":"setStatus","statusKey":"k","statusText":"v"}`,
			want: func(t *testing.T, ev Event) {
				req := ev.(*ExtensionUIRequestEvent)
				if req.IsDialog() || req.StatusKey != "k" {
					t.Fatalf("unexpected payload: %+v", req)
				}
			},
		},
		{
			name: "response",
			line: `{"type":"response","id":"req-1","command":"get_state","success":true,"data":{"isStreaming":false}}`,
			want: func(t *testing.T, ev Event) {
				resp := ev.(*Response)
				if !resp.Success || resp.ID != "req-1" {
					t.Fatalf("unexpected payload: %+v", resp)
				}
				var data struct {
					IsStreaming bool `json:"isStreaming"`
				}
				if err := resp.DecodeData(&data); err != nil || data.IsStreaming {
					t.Fatalf("decode data: %v %+v", err, data)
				}
			},
		},
		{
			name: "unknown type",
			line: `{"type":"future_event","payload":{"a":1}}`,
			want: func(t *testing.T, ev Event) {
				unknown := ev.(*UnknownEvent)
				if unknown.Type != "future_event" || !strings.Contains(string(unknown.Raw), "payload") {
					t.Fatalf("unexpected payload: %+v", unknown)
				}
			},
		},
		{
			name: "known type with wrong payload type",
			line: `{"type":"agent_end","messages":"not-an-array"}`,
			want: func(t *testing.T, ev Event) {
				if _, ok := ev.(*UnknownEvent); !ok {
					t.Fatalf("expected UnknownEvent, got %T", ev)
				}
			},
		},
		{
			name: "unicode line separator preserved inside strings",
			line: "{\"type\":\"message_end\",\"message\":{\"role\":\"assistant\",\"content\":\"a\u2028b\u2029c\"}}",
			want: func(t *testing.T, ev Event) {
				end := ev.(*MessageEndEvent)
				if end.Message.Text() != "a\u2028b\u2029c" {
					t.Fatalf("text: %q", end.Message.Text())
				}
			},
		},
		{
			name: "crlf and surrounding whitespace",
			line: "  {\"type\":\"turn_start\"}  \r",
			want: func(t *testing.T, ev Event) {
				if _, ok := ev.(*TurnStartEvent); !ok {
					t.Fatalf("got %T", ev)
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ev, err := ParseLine([]byte(tc.line))
			if err != nil {
				t.Fatalf("ParseLine: %v", err)
			}
			tc.want(t, ev)
		})
	}
}

func TestParseLineMalformed(t *testing.T) {
	t.Parallel()

	for _, line := range []string{"", "   ", "not json", "[]", "null", `{"no_type":true}`, `{"type":`} {
		if _, err := ParseLine([]byte(line)); !errors.Is(err, ErrMalformedRecord) {
			t.Fatalf("ParseLine(%q) error = %v, want ErrMalformedRecord", line, err)
		}
	}
}

func TestMessageContentVariants(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"role":"user","content":"plain string","timestamp":5}`)
	var message Message
	if err := json.Unmarshal(raw, &message); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if message.Text() != "plain string" || len(message.Content) != 1 {
		t.Fatalf("unexpected message: %+v", message)
	}

	raw = []byte(`{"role":"assistant","content":[{"type":"thinking","thinking":"hmm","redacted":true},{"type":"text","text":"a"},{"type":"text","text":"b"},{"type":"mystery","text":"x"}]}`)
	if err := json.Unmarshal(raw, &message); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if message.Text() != "a\nb" {
		t.Fatalf("text: %q", message.Text())
	}
	if len(message.Content) != 4 || message.Content[0].Thinking != "hmm" || !message.Content[0].Redacted {
		t.Fatalf("unexpected content: %+v", message.Content)
	}

	raw = []byte(`{"role":"assistant","content":null}`)
	if err := json.Unmarshal(raw, &message); err != nil {
		t.Fatalf("null content: %v", err)
	}
	if len(message.Content) != 0 {
		t.Fatalf("expected empty content, got %+v", message.Content)
	}
}

func TestStreamAccumulator(t *testing.T) {
	t.Parallel()

	var acc StreamAccumulator
	events := []AssistantMessageEvent{
		{Type: "start"},
		{Type: "thinking_start", ContentIndex: 0},
		{Type: "thinking_delta", ContentIndex: 0, Delta: "pl"},
		{Type: "thinking_delta", ContentIndex: 0, Delta: "an"},
		{Type: "thinking_end", ContentIndex: 0, Content: "plan"},
		{Type: "text_start", ContentIndex: 1},
		{Type: "text_delta", ContentIndex: 1, Delta: "Hel"},
		{Type: "text_delta", ContentIndex: 1, Delta: "lo"},
		{Type: "toolcall_start", ContentIndex: 2, ToolCallID: "call_1", ToolName: "read"},
		{Type: "toolcall_delta", ContentIndex: 2, Delta: `{"path":`},
		{Type: "toolcall_delta", ContentIndex: 2, Delta: `"a.md"}`},
		{Type: "done"},
	}
	for _, event := range events {
		acc.Apply(event)
	}

	if got := acc.Thinking(); got != "plan" {
		t.Fatalf("thinking = %q", got)
	}
	if got := acc.Text(); got != "Hello" {
		t.Fatalf("text = %q", got)
	}

	blocks := acc.Blocks()
	if len(blocks) != 3 {
		t.Fatalf("blocks = %+v", blocks)
	}
	if blocks[2].Kind != "toolCall" || blocks[2].ToolName != "read" || blocks[2].Args != `{"path":"a.md"}` {
		t.Fatalf("tool block = %+v", blocks[2])
	}

	// Authoritative end payloads must replace accumulated deltas.
	acc.Apply(AssistantMessageEvent{Type: "text_end", ContentIndex: 1, Content: "Hello!"})
	if got := acc.Text(); got != "Hello!" {
		t.Fatalf("text after end = %q", got)
	}

	// toolcall_end replaces the serialized argument buffer.
	acc.Apply(AssistantMessageEvent{
		Type:         "toolcall_end",
		ContentIndex: 2,
		ToolCall:     &ToolCall{ID: "call_1", Name: "read", Arguments: json.RawMessage(`{"path":"b.md"}`)},
	})
	if got := acc.Blocks()[2]; got.Args != `{"path":"b.md"}` || !got.Done {
		t.Fatalf("tool block after end = %+v", got)
	}

	acc.Reset()
	if len(acc.Blocks()) != 0 {
		t.Fatal("reset did not clear blocks")
	}
}

func TestDecodeArgs(t *testing.T) {
	t.Parallel()

	if args := DecodeArgs(json.RawMessage(`{"command":"ls"}`)); args["command"] != "ls" {
		t.Fatalf("args = %+v", args)
	}
	if args := DecodeArgs(json.RawMessage(`not json`)); args != nil {
		t.Fatalf("expected nil for invalid json, got %+v", args)
	}
}
