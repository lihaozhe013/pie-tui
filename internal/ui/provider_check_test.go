package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakePiScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pi")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCheckProviderParsesReady(t *testing.T) {
	m := New(newFakeClient(), Options{PiBin: fakePiScript(t, `echo '{"status":"ready","provider":"demo","authType":"api_key"}'`)})
	msg, ok := m.checkProvider("demo")().(providerCheckMsg)
	if !ok || msg.status != "ready" || msg.kind != "api_key" || msg.err != nil {
		t.Fatalf("msg = %+v", msg)
	}
	if got := providerCheckSummary(msg); got != "demo: ready (api_key)" {
		t.Fatalf("summary = %q", got)
	}
}

func TestCheckProviderParsesNotReadyExit(t *testing.T) {
	script := fakePiScript(t, `echo '{"status":"not_ready","provider":"demo","reason":"credentials_not_configured"}'; exit 1`)
	m := New(newFakeClient(), Options{PiBin: script})
	msg, ok := m.checkProvider("demo")().(providerCheckMsg)
	if !ok || msg.status != "not_ready" || msg.err != nil {
		t.Fatalf("msg = %+v", msg)
	}
	if got := providerCheckSummary(msg); !strings.Contains(got, "credentials_not_configured") {
		t.Fatalf("summary = %q", got)
	}
}
