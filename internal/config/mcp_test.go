package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPSettersKeepUnknownFields(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", dir)
	path := filepath.Join(dir, "mcp.json")
	content := `{
  "mcpServers": {
    "pw": {"command": "playwright-mcp", "args": ["--x"], "timeout": 90, "exposure": "codemode"},
    "web": {"url": "https://example.com/mcp"}
  },
  "other": true
}
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	paths, _ := ResolvePaths()
	file, err := LoadMCP(paths)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := file.SetServerEnabled("pw", false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if err := file.SetServerExposure("pw", "direct"); err != nil {
		t.Fatalf("exposure: %v", err)
	}
	if err := file.SetServerExposure("web", "auto"); err != nil {
		t.Fatalf("exposure auto: %v", err)
	}
	if err := file.SetServerExposure("pw", "bogus"); err == nil {
		t.Fatal("invalid exposure accepted")
	}
	if err := file.RemoveServer("web"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if err := file.RemoveServer("web"); err == nil {
		t.Fatal("missing server removed")
	}

	data, _ := os.ReadFile(path)
	text := string(data)
	if !strings.Contains(text, `"other": true`) || !strings.Contains(text, `"timeout": 90`) || !strings.Contains(text, `"--x"`) {
		t.Fatalf("unknown fields dropped:\n%s", text)
	}
	if !strings.Contains(text, `"enabled": false`) || !strings.Contains(text, `"exposure": "direct"`) {
		t.Fatalf("edits missing:\n%s", text)
	}
	if strings.Contains(text, `"web"`) {
		t.Fatalf("removed server still present:\n%s", text)
	}

	file, _ = LoadMCP(paths)
	server, ok := file.Find("pw")
	if !ok || server.Enabled {
		t.Fatalf("server = %+v, %v", server, ok)
	}
	if err := file.SetServerEnabled("pw", true); err != nil {
		t.Fatal(err)
	}
	file, _ = LoadMCP(paths)
	server, _ = file.Find("pw")
	if !server.Enabled {
		t.Fatal("server not re-enabled")
	}
}
