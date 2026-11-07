package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeSession(t *testing.T, dir, project, file, header, extra string) string {
	t.Helper()
	path := filepath.Join(dir, project)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	target := filepath.Join(path, file)
	if err := os.WriteFile(target, []byte(header+extra), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return target
}

func TestListReadsHeadersAndSorts(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	older := writeSession(t, dir, "--Users-me-proj--", "2024_old.jsonl",
		`{"type":"session","version":3,"id":"aaaaaaaa-1111","timestamp":"2024-12-03T14:00:00.000Z","cwd":"/Users/me/proj"}`+"\n",
		`{"type":"message","id":"m1","parentId":null,"timestamp":"2024-12-03T14:00:01.000Z","message":{"role":"user","content":"hi","timestamp":1}}`+"\n")
	newer := writeSession(t, dir, "--Users-me-other--", "2025_new.jsonl",
		`{"type":"session","version":3,"id":"bbbbbbbb-2222","timestamp":"2025-01-03T14:00:00.000Z","cwd":"/Users/me/other"}`+"\n",
		`{"type":"session_info","id":"k1","parentId":null,"timestamp":"2025-01-03T14:01:00.000Z","name":"Refactor auth"}`+"\n")

	past := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(older, past, past); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	entries, err := List(dir, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %+v", entries)
	}
	if entries[0].Path != newer {
		t.Fatalf("expected newest first, got %+v", entries)
	}
	if entries[0].Name != "Refactor auth" || entries[0].Label() != "Refactor auth" {
		t.Fatalf("name = %q label = %q", entries[0].Name, entries[0].Label())
	}
	if entries[1].Label() != "session aaaaaaaa" {
		t.Fatalf("label = %q", entries[1].Label())
	}
	if entries[1].CWD != "/Users/me/proj" {
		t.Fatalf("cwd = %q", entries[1].CWD)
	}
}

func TestListSkipsBrokenFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeSession(t, dir, "p", "broken.jsonl", "not json\n", "")
	writeSession(t, dir, "p", "wrong.jsonl", `{"type":"message","id":"x"}`+"\n", "")
	writeSession(t, dir, "p", "ok.jsonl",
		`{"type":"session","version":3,"id":"cccccccc-3333","cwd":"/w"}`+"\n", "")
	writeSession(t, dir, "p", "ignored.txt", `{"type":"session","id":"d"}`, "")

	entries, err := List(dir, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != "cccccccc-3333" {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestListForCWDFilters(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeSession(t, dir, "p", "a.jsonl", `{"type":"session","id":"a1","cwd":"/work/one"}`+"\n", "")
	writeSession(t, dir, "p", "b.jsonl", `{"type":"session","id":"b2","cwd":"/work/two"}`+"\n", "")

	entries, err := ListForCWD(dir, "/work/one/", 10)
	if err != nil {
		t.Fatalf("ListForCWD: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != "a1" {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestListMissingDirectory(t *testing.T) {
	t.Parallel()

	entries, err := List(filepath.Join(t.TempDir(), "absent"), 5)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if entries != nil {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestDirPrecedence(t *testing.T) {
	agentDir := t.TempDir()
	workDir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", agentDir)
	t.Setenv(DefaultDirEnv, "/from/env")
	t.Chdir(workDir)

	if dir, err := Dir(""); err != nil || dir != "/from/env" {
		t.Fatalf("env dir = %q err = %v", dir, err)
	}
	if dir, err := Dir("/explicit"); err != nil || dir != "/explicit" {
		t.Fatalf("explicit dir = %q err = %v", dir, err)
	}

	t.Setenv(DefaultDirEnv, "")
	if dir, err := Dir(""); err != nil || dir != filepath.Join(agentDir, "sessions") {
		t.Fatalf("agent dir = %q err = %v", dir, err)
	}

	settingsPath := filepath.Join(agentDir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte(`{"sessionDir": "local-sessions"}`), 0o600); err != nil {
		t.Fatalf("write settings: %v", err)
	}
	if dir, err := Dir(""); err != nil || dir != filepath.Join(workDir, "local-sessions") {
		t.Fatalf("settings dir = %q err = %v", dir, err)
	}
}

func TestReadNameHandlesClearAndLongFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeSession(t, dir, "p", "n.jsonl",
		`{"type":"session","id":"eeeeeeee-4444","cwd":"/w"}`+"\n",
		`{"type":"session_info","id":"k1","name":"first"}`+"\n"+
			`{"type":"session_info","id":"k2","name":"second"}`+"\n"+
			`{"type":"message","id":"m1"}`+"\n")

	entry, err := readEntry(path)
	if err != nil {
		t.Fatalf("readEntry: %v", err)
	}
	if entry.Name != "second" {
		t.Fatalf("name = %q", entry.Name)
	}
}
