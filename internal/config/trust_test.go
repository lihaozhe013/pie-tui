package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectTrustedInheritsAndOverrides(t *testing.T) {
	dir := t.TempDir()
	agent := filepath.Join(dir, "agent")
	project := filepath.Join(dir, "work", "app")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(project, ".pi"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".pi", "mcp.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(agent, 0o755); err != nil {
		t.Fatal(err)
	}
	if !ProjectRequiresTrust(project) {
		t.Fatal("project resources not detected")
	}
	trusted, err := ProjectTrusted(agent, project)
	if err != nil || trusted {
		t.Fatalf("missing store = %v %v", trusted, err)
	}
	work := filepath.Join(dir, "work")
	data := `{"` + work + `":true,"` + project + `":false}`
	if err := os.WriteFile(filepath.Join(agent, "trust.json"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	trusted, err = ProjectTrusted(agent, project)
	if err != nil || trusted {
		t.Fatalf("exact false = %v %v", trusted, err)
	}
	other := filepath.Join(work, "other")
	if trusted, err = ProjectTrusted(agent, other); err != nil || !trusted {
		t.Fatalf("inherited = %v %v", trusted, err)
	}
}
