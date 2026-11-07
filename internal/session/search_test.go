package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSearchMatchesNameAndTranscript(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, lines ...string) string {
		path := filepath.Join(dir, "grp", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		content := ""
		for _, line := range lines {
			content += line + "\n"
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	header := func(id, cwd string) string {
		return `{"type":"session","version":3,"id":"` + id + `","cwd":"` + cwd + `"}`
	}
	write("one.jsonl",
		header("one", "/work/alpha"),
		`{"type":"message","message":{"role":"user","content":[{"type":"text","text":"Refactor the settings panel"}]}}`,
		`{"type":"session_info","name":"UI cleanup"}`)
	write("two.jsonl",
		header("two", "/work/beta"),
		`{"type":"message","message":{"role":"assistant","content":[{"type":"text","text":"The parser handles JSONC now."}]}}`)

	hits, err := Search(dir, "jsonc", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 1 || hits[0].ID != "two" || hits[0].Matches < 1 {
		t.Fatalf("hits = %+v", hits)
	}
	if hits[0].Snippet == "" || !containsFold(hits[0].Snippet, "jsonc") {
		t.Fatalf("snippet = %q", hits[0].Snippet)
	}

	hits, _ = Search(dir, "ui cleanup", 10)
	if len(hits) != 1 || hits[0].ID != "one" {
		t.Fatalf("name hits = %+v", hits)
	}

	hits, _ = Search(dir, "/work/beta", 10)
	if len(hits) != 1 || hits[0].ID != "two" {
		t.Fatalf("cwd hits = %+v", hits)
	}
}

func containsFold(value, query string) bool {
	return len(value) >= len(query) && (value == query || len(value) > 0) && indexFold(value, query) >= 0
}

func indexFold(value, query string) int {
	for i := 0; i+len(query) <= len(value); i++ {
		if equalFold(value[i:i+len(query)], query) {
			return i
		}
	}
	return -1
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
