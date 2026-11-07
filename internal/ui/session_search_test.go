package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSessionSearchOpensResults(t *testing.T) {
	dir := t.TempDir()
	group := filepath.Join(dir, "grp")
	if err := os.MkdirAll(group, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `{"type":"session","version":3,"id":"one","cwd":"/work/proj"}
{"type":"message","message":{"role":"user","content":[{"type":"text","text":"Fix the JSONC parser"}]}}
`
	if err := os.WriteFile(filepath.Join(group, "one.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	m := New(newFakeClient(), Options{SessionDir: dir})
	msg, ok := m.searchSessions("jsonc")().(sessionSearchMsg)
	if !ok || msg.err != nil {
		t.Fatalf("msg = %+v", msg)
	}
	model, _ := m.Update(msg)
	m = model.(*Model)
	if m.overlay == nil || m.overlay.action != "session-search-hit" {
		t.Fatal("results overlay missing")
	}
	if len(m.overlay.items) != 1 || m.overlay.items[0].Value != filepath.Join(group, "one.jsonl") {
		t.Fatalf("items = %+v", m.overlay.items)
	}
}
