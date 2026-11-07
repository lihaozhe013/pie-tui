package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestSplitMarkdown(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		src       string
		stable    int
		tail      string
		fenceOpen bool
	}{
		{
			name:   "single paragraph",
			src:    "hello world",
			stable: 0,
			tail:   "hello world",
		},
		{
			name:   "two complete paragraphs",
			src:    "first\n\nsecond\n\n",
			stable: 2,
			tail:   "",
		},
		{
			name:   "trailing paragraph is unstable",
			src:    "first\n\nsecond",
			stable: 1,
			tail:   "second",
		},
		{
			name:      "blank lines inside fences do not split",
			src:       "intro\n\n```go\nfunc main() {\n\n}\n```\n\nafter",
			stable:    2,
			tail:      "after",
			fenceOpen: false,
		},
		{
			name:      "unterminated fence keeps the tail open",
			src:       "intro\n\n```go\nfunc main() {",
			stable:    1,
			tail:      "```go\nfunc main() {",
			fenceOpen: true,
		},
		{
			name:   "tilde fences",
			src:    "a\n\n~~~\ncode\n\nmore\n~~~\n\nb",
			stable: 2,
			tail:   "b",
		},
		{
			name:   "list stays in one segment",
			src:    "- one\n- two\n\nnext",
			stable: 1,
			tail:   "next",
		},
		{
			name:   "crlf input",
			src:    "first\r\n\r\nsecond\r\n\r\n",
			stable: 2,
			tail:   "",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			split := splitMarkdown(tc.src)
			if len(split.stable) != tc.stable {
				t.Fatalf("stable = %d (%q), want %d", len(split.stable), split.stable, tc.stable)
			}
			if split.tail != tc.tail {
				t.Fatalf("tail = %q, want %q", split.tail, tc.tail)
			}
			if split.fenceOpen != tc.fenceOpen {
				t.Fatalf("fenceOpen = %v, want %v", split.fenceOpen, tc.fenceOpen)
			}
		})
	}
}

func TestTranscriptRendersBlocks(t *testing.T) {
	t.Parallel()

	st := newStyles(defaultTheme())
	transcript := newTranscript(st)
	transcript.SetSize(60, 20)

	transcript.Add(newBlock(blockUser, "hello there"))
	transcript.Add(newBlock(blockAssistant, "**bold** answer"))

	view := ansi.Strip(transcript.View())
	if !strings.Contains(view, "hello there") {
		t.Fatalf("missing user text: %q", view)
	}
	if !strings.Contains(view, "bold") {
		t.Fatalf("missing assistant text: %q", view)
	}
}

func TestTranscriptReusesStableSegments(t *testing.T) {
	t.Parallel()

	st := newStyles(defaultTheme())
	transcript := newTranscript(st)
	transcript.SetSize(72, 20)

	block := newBlock(blockAssistant, "first paragraph\n\nsecond")
	block.streaming = true
	transcript.Add(block)
	_ = transcript.View()
	if len(block.stable.out) != 1 {
		t.Fatalf("stable renders = %d, want 1", len(block.stable.out))
	}

	// Growing the tail must not re-render the already stable segment.
	block.setText("first paragraph\n\nsecond paragraph grows")
	transcript.MarkDirty()
	_ = transcript.View()
	if len(block.stable.out) != 1 {
		t.Fatalf("stable renders after growth = %d, want 1", len(block.stable.out))
	}

	block.setText("first paragraph\n\nsecond paragraph grows\n\nthird")
	transcript.MarkDirty()
	_ = transcript.View()
	if len(block.stable.out) != 2 {
		t.Fatalf("stable renders after new segment = %d, want 2", len(block.stable.out))
	}

	// Completed blocks reuse the whole cached render.
	block.streaming = false
	block.revision++
	first := transcript.render(block)
	second := transcript.render(block)
	if first != second {
		t.Fatal("cache did not stabilise")
	}
}

func TestTranscriptKeepsOpenFenceVisible(t *testing.T) {
	t.Parallel()

	st := newStyles(defaultTheme())
	transcript := newTranscript(st)
	transcript.SetSize(80, 20)

	block := newBlock(blockAssistant, "```go\nfunc main() {")
	block.streaming = true
	transcript.Add(block)

	view := ansi.Strip(transcript.View())
	if !strings.Contains(view, "func main() {") {
		t.Fatalf("streaming code block not rendered: %q", view)
	}
}

func TestTranscriptStickBottom(t *testing.T) {
	t.Parallel()

	st := newStyles(defaultTheme())
	transcript := newTranscript(st)
	transcript.SetSize(40, 5)

	for i := 0; i < 20; i++ {
		transcript.Add(newBlock(blockNotice, strings.Repeat("line\n", 3)))
	}
	_ = transcript.View()
	if !transcript.vp.AtBottom() {
		t.Fatal("expected the viewport to follow new content")
	}

	transcript.vp.SetYOffset(0)
	transcript.Update(nil)
	if transcript.stickBottom {
		t.Fatal("scrolling up must disable stick-bottom")
	}

	transcript.ScrollToBottom()
	if !transcript.stickBottom {
		t.Fatal("ScrollToBottom must re-enable stick-bottom")
	}
}

func TestTranscriptFollowsOnlyWhenPinned(t *testing.T) {
	t.Parallel()

	st := newStyles(defaultTheme())
	transcript := newTranscript(st)
	transcript.SetSize(40, 5)

	for i := 0; i < 10; i++ {
		transcript.Add(newBlock(blockNotice, "line"))
	}
	_ = transcript.View()

	// Scrolling up must survive new blocks.
	transcript.vp.SetYOffset(1)
	transcript.stickBottom = false
	transcript.Add(newBlock(blockNotice, "late"))
	_ = transcript.View()
	if transcript.vp.YOffset != 1 {
		t.Fatalf("offset moved to %d", transcript.vp.YOffset)
	}
}

func TestRenderToolBlockRespectsExpansion(t *testing.T) {
	t.Parallel()

	st := newStyles(defaultTheme())
	transcript := newTranscript(st)
	transcript.SetSize(80, 20)

	block := newBlock(blockTool, "")
	block.callID = "call_1"
	block.title = "bash"
	block.meta = "go test ./..."
	block.body = "ok  package"
	block.status = statusOK
	block.collapsed = true
	transcript.Add(block)

	collapsed := ansi.Strip(transcript.render(block))
	if strings.Contains(collapsed, "ok  package") {
		t.Fatalf("collapsed tool block leaked output: %q", collapsed)
	}

	block.collapsed = false
	block.invalidate()
	expanded := ansi.Strip(transcript.render(block))
	if !strings.Contains(expanded, "ok  package") {
		t.Fatalf("expanded tool block missing output: %q", expanded)
	}
}

func TestFormatHelpers(t *testing.T) {
	t.Parallel()

	if got := formatTokens(999); got != "999" {
		t.Fatalf("formatTokens(999) = %q", got)
	}
	if got := formatTokens(1500); got != "1.5k" {
		t.Fatalf("formatTokens(1500) = %q", got)
	}
	if got := formatTokens(2_400_000); got != "2.4M" {
		t.Fatalf("formatTokens(2400000) = %q", got)
	}
	if got := truncateLine("abcdef", 4); !strings.HasSuffix(got, "…") {
		t.Fatalf("truncateLine = %q", got)
	}
}
