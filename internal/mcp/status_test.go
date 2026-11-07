package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakePi(t *testing.T, script string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pi")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCheckParsesReports(t *testing.T) {
	bin := fakePi(t, `cat <<'JSON'
{"servers":[{"name":"pw","scope":"global","state":"connected","transport":"stdio: playwright","tools":["click","type"],"resources":true},{"name":"off","scope":"global","enabled":false,"state":"disabled","transport":"http: x"}],"errors":[{"scope":"project","message":"untrusted"}],"note":"project files ignored"}
JSON
`)
	snapshot, err := Check(context.Background(), bin, t.TempDir())
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(snapshot.Servers) != 2 || snapshot.Servers[0].Name != "pw" || len(snapshot.Servers[0].Tools) != 2 {
		t.Fatalf("servers = %+v", snapshot.Servers)
	}
	if len(snapshot.Errors) != 1 || !strings.Contains(snapshot.Errors[0], "untrusted") {
		t.Fatalf("errors = %+v", snapshot.Errors)
	}
	if snapshot.Note == "" {
		t.Fatal("note lost")
	}
}

func TestAddPassesLocalFlag(t *testing.T) {
	bin := fakePi(t, `for arg in "$@"; do printf '%s|' "$arg"; done`)
	output, err := Add(context.Background(), bin, t.TempDir(), []string{"demo", "--", "npx", "server"}, true)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if !strings.Contains(output, "mcp|add|demo|--|npx|server|-l|") {
		t.Fatalf("args = %q", output)
	}
}
