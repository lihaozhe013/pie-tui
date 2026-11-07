package ui

import (
	"testing"
)

func TestComputeDiffTrimsCommonLines(t *testing.T) {
	t.Parallel()

	oldText := "a\nb\nc\nd"
	newText := "a\nb\nC\nd"
	lines := computeDiff(oldText, newText, 1)

	var kinds []byte
	var texts []string
	for _, line := range lines {
		kinds = append(kinds, line.kind)
		texts = append(texts, line.text)
	}

	wantKinds := []byte{' ', '-', '+', ' '}
	if string(kinds) != string(wantKinds) {
		t.Fatalf("kinds = %q (%v), want %q", string(kinds), texts, string(wantKinds))
	}
	if texts[1] != "c" || texts[2] != "C" {
		t.Fatalf("texts = %v", texts)
	}
}

func TestComputeDiffPureAddition(t *testing.T) {
	t.Parallel()

	lines := computeDiff("", "one\ntwo", 0)
	added, removed := 0, 0
	for _, line := range lines {
		switch line.kind {
		case '+':
			added++
		case '-':
			removed++
		}
	}
	if added != 2 || removed != 0 {
		t.Fatalf("added=%d removed=%d", added, removed)
	}
}

func TestToolDiffSupportsEditSchemas(t *testing.T) {
	t.Parallel()

	// Current schema: edits array.
	path, pairs, ok := toolDiff("edit", map[string]any{
		"path": "internal/ui/model.go",
		"edits": []any{
			map[string]any{"oldText": "old line", "newText": "new line"},
		},
	})
	if !ok || path != "internal/ui/model.go" || len(pairs) != 1 {
		t.Fatalf("ok=%v path=%q pairs=%+v", ok, path, pairs)
	}
	if pairs[0].Old != "old line" || pairs[0].New != "new line" {
		t.Fatalf("pairs = %+v", pairs)
	}

	// Legacy schema: top-level oldText/newText.
	_, pairs, ok = toolDiff("edit", map[string]any{
		"path":    "main.go",
		"oldText": "a",
		"newText": "b",
	})
	if !ok || len(pairs) != 1 || pairs[0].New != "b" {
		t.Fatalf("legacy pairs = %+v ok=%v", pairs, ok)
	}

	// write is a pure addition.
	path, pairs, ok = toolDiff("write", map[string]any{"path": "new.txt", "content": "hello\n"})
	if !ok || path != "new.txt" || pairs[0].Old != "" || pairs[0].New != "hello\n" {
		t.Fatalf("write = %+v ok=%v", pairs, ok)
	}

	// Unrelated tools, missing args, and empty payloads produce no diff.
	if _, _, ok := toolDiff("bash", map[string]any{"command": "ls"}); ok {
		t.Fatal("bash must not produce a diff")
	}
	if _, _, ok := toolDiff("edit", map[string]any{"path": "x"}); ok {
		t.Fatal("edit without edits must not produce a diff")
	}
	if _, _, ok := toolDiff("edit", nil); ok {
		t.Fatal("nil args must not produce a diff")
	}
}

func TestDiffBodyTruncates(t *testing.T) {
	t.Parallel()

	pairs := []diffPair{{Old: "", New: "1\n2\n3\n4\n5\n6\n7\n8"}}
	body, truncated := diffBody(pairs, 3)
	if !truncated {
		t.Fatal("expected truncation")
	}
	if lines := len(splitLines(body)); lines != 3 {
		t.Fatalf("lines = %d body = %q", lines, body)
	}
}

func TestToolSummaryPicksUsefulFields(t *testing.T) {
	t.Parallel()

	if got := toolSummary("bash", map[string]any{"command": "go test ./..."}); got != "go test ./..." {
		t.Fatalf("summary = %q", got)
	}
	if got := toolSummary("read", map[string]any{"path": "README.md"}); got != "README.md" {
		t.Fatalf("summary = %q", got)
	}
	long := toolSummary("bash", map[string]any{"command": string(make([]byte, 0)) + repeat("x", 200)})
	if len([]rune(long)) > 80 {
		t.Fatalf("summary not truncated: %d", len([]rune(long)))
	}
	if got := toolSummary("read", map[string]any{}); got != "" {
		t.Fatalf("summary = %q", got)
	}
}

func repeat(value string, count int) string {
	out := make([]byte, 0, count*len(value))
	for i := 0; i < count; i++ {
		out = append(out, value...)
	}
	return string(out)
}
