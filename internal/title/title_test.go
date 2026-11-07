package title

import (
	"strings"
	"testing"

	"github.com/lihaozhe/pie-tui/internal/modelcatalog"
	"github.com/lihaozhe/pie-tui/internal/rpc"
)

func message(role, text string) rpc.Message {
	return rpc.Message{Role: role, Content: rpc.Content{{Type: "text", Text: text}}}
}

func TestTranscriptDropsToolsAndKeepsEnds(t *testing.T) {
	messages := []rpc.Message{
		message("user", "refactor the settings panel"),
		message("assistant", "I will start with the rail."),
		{Role: "toolResult", Content: rpc.Content{{Type: "text", Text: "huge output"}}},
		{Role: "user", Content: rpc.Content{
			{Type: "text", Text: "look at this"},
			{Type: "image", Data: "base64", MimeType: "image/png"},
		}},
		message("assistant", strings.Repeat("a", 900)),
	}
	got := Transcript(messages)
	if strings.Contains(got, "huge output") {
		t.Fatal("tool output leaked into the transcript")
	}
	if !strings.Contains(got, "User: look at this\n[image]") {
		t.Fatalf("image marker missing: %q", got)
	}
	if !strings.Contains(got, "Assistant: "+strings.Repeat("a", 600)+"…") {
		t.Fatalf("last assistant reply not clipped at 600: %d chars", len(got))
	}
}

func TestTranscriptKeepsHeadAndTail(t *testing.T) {
	messages := []rpc.Message{message("user", "opening goal "+strings.Repeat("x", 500))}
	for i := 0; i < 40; i++ {
		messages = append(messages, message("user", strings.Repeat("m", 700)))
	}
	messages = append(messages, message("assistant", "the outcome"))

	got := Transcript(messages)
	if !strings.HasPrefix(got, "User: opening goal") {
		t.Fatalf("first turn dropped: %.40q", got)
	}
	if !strings.HasSuffix(got, "Assistant: the outcome") {
		t.Fatalf("last reply dropped: %.40q", got[len(got)-40:])
	}
	if !strings.Contains(got, "[…]") {
		t.Fatal("middle not elided")
	}
	if len([]rune(got)) > transcriptChars+1000 {
		t.Fatalf("transcript not bounded: %d", len([]rune(got)))
	}
}

func TestParseAcceptsCommonShapes(t *testing.T) {
	cases := map[string]string{
		`Fix the settings rail`:                                "Fix the settings rail",
		`"Fix the settings rail."`:                             "Fix the settings rail",
		"```json\n{\"title\": \"Fix the settings rail\"}\n```": "Fix the settings rail",
		`Title: Fix the settings rail`:                         "Fix the settings rail",
		`标题：修复设置栏。`:                                            "修复设置栏",
		"First line\nsecond line":                              "First line",
	}
	for raw, want := range cases {
		got, err := Parse(raw)
		if err != nil {
			t.Fatalf("Parse(%q): %v", raw, err)
		}
		if got != want {
			t.Fatalf("Parse(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestParseRejectsEmptyAndCapsLength(t *testing.T) {
	if _, err := Parse("  \n "); err == nil {
		t.Fatal("empty title accepted")
	}
	long := strings.Repeat("标", 120)
	got, err := Parse(long)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len([]rune(got)) != MaxLength {
		t.Fatalf("length = %d", len([]rune(got)))
	}
}

func TestResolveThinkingLevel(t *testing.T) {
	off := modelcatalog.Model{ID: "plain", Reasoning: false}
	if got := ResolveThinkingLevel(&off); got != "off" {
		t.Fatalf("non-reasoning = %q", got)
	}

	low := "low"
	levels := map[string]*string{"off": nil, "low": &low, "high": &low}
	reasoning := modelcatalog.Model{
		ID: "reasoner", Reasoning: true, API: "openai-completions",
		ThinkingLevelMap: levels,
	}
	if got := ResolveThinkingLevel(&reasoning); got != "off" {
		t.Fatalf("reasoning = %q", got)
	}

	minimal := "minimal"
	gemini := modelcatalog.Model{
		ID: "gemini-2.5-pro", Reasoning: true, API: "google-generative-ai",
		ThinkingLevelMap: map[string]*string{"off": nil, "minimal": &minimal, "low": &low},
	}
	if got := ResolveThinkingLevel(&gemini); got != "low" {
		t.Fatalf("gemini = %q", got)
	}
}
