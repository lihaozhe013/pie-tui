package skills

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func writeSkill(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func skillDoc(name, description string) string {
	return "---\nname: " + name + "\ndescription: " + description + "\n---\n# " + name + "\n"
}

func TestLoadDiscoversPiLayouts(t *testing.T) {
	root := t.TempDir()
	agent := filepath.Join(root, "agent")
	cwd := filepath.Join(root, "project")
	t.Setenv("HOME", filepath.Join(root, "home"))

	writeSkill(t, filepath.Join(agent, "skills", "alpha", "SKILL.md"), skillDoc("alpha", "First"))
	writeSkill(t, filepath.Join(agent, "skills", "beta.md"), skillDoc("beta", "Second"))
	writeSkill(t, filepath.Join(agent, "skills", "nested", "gamma", "SKILL.md"), skillDoc("gamma", "Third"))
	writeSkill(t, filepath.Join(agent, "skills", "missing.md"), "---\nname: missing\n---\n")
	writeSkill(t, filepath.Join(agent, "skills", "bundle", "SKILL.md"), skillDoc("bundle", "Bundle root"))
	writeSkill(t, filepath.Join(agent, "skills", "bundle", "inner", "SKILL.md"), skillDoc("inner", "Must not load"))
	writeSkill(t, filepath.Join(cwd, ".pi", "skills", "proj", "SKILL.md"), skillDoc("proj", "Project"))
	writeSkill(t, filepath.Join(root, "extra", "extra.md"), skillDoc("extra", "Extra path"))

	// A symlinked copy of alpha must not appear twice.
	if err := os.Symlink(filepath.Join(agent, "skills", "alpha"), filepath.Join(agent, "skills", "alpha-link")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	list, diagnostics := Load(agent, cwd, []string{filepath.Join(root, "extra"), filepath.Join(root, "missing-path")})
	names := map[string]Skill{}
	for _, skill := range list {
		names[skill.Name] = skill
	}
	for _, want := range []string{"alpha", "beta", "gamma", "bundle", "proj", "extra"} {
		if _, ok := names[want]; !ok {
			t.Fatalf("skill %q missing from %v", want, names)
		}
	}
	for _, unwanted := range []string{"missing", "inner"} {
		if _, ok := names[unwanted]; ok {
			t.Fatalf("skill %q should not load", unwanted)
		}
	}
	if names["proj"].Scope != ScopeProject || names["alpha"].Scope != ScopeGlobal || names["extra"].Scope != ScopePath {
		t.Fatalf("scopes = %+v", names)
	}
	if len(list) != 6 {
		t.Fatalf("skills = %d: %+v", len(list), names)
	}
	if len(diagnostics) == 0 {
		t.Fatal("missing path diagnostic not reported")
	}

	packageDir := filepath.Join(agent, "npm", "node_modules", "pkg")
	writeSkill(t, filepath.Join(packageDir, "skills", "pack", "SKILL.md"), skillDoc("pack", "From a package"))
	if err := os.WriteFile(filepath.Join(packageDir, "package.json"), []byte(`{"name":"pkg","pi":{"skills":["./skills"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	list, _ = Load(agent, cwd, nil)
	foundPackage := false
	for _, skill := range list {
		if skill.Name == "pack" && skill.Scope == ScopePackage {
			foundPackage = true
		}
	}
	if !foundPackage {
		t.Fatalf("package skill missing: %+v", list)
	}

	if err := SetDisabled(names["alpha"].FilePath, true); err != nil {
		t.Fatalf("disable: %v", err)
	}
	list, _ = Load(agent, cwd, nil)
	if list[len(list)-1].Name != "alpha" || !list[len(list)-1].Disabled {
		t.Fatalf("disabled skill not ordered last: %+v", list)
	}
}

func TestSearchReadsSkillssh(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/search" || r.URL.Query().Get("q") != "react" {
			t.Fatalf("request = %s", r.URL)
		}
		_, _ = w.Write([]byte(`{"skills":[{"id":"owner/repo/react","source":"owner/repo","name":"react","installs":1200},{"id":"x/y/z","source":"x/y","name":"z","installs":1}]}`))
	}))
	defer server.Close()

	results, err := Search(context.Background(), server.URL, "react", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 2 || results[0].Package != "owner/repo@react" || results[0].Installs != "1.2K installs" {
		t.Fatalf("results = %+v", results)
	}
	if results[0].URL != server.URL+"/owner/repo/react" {
		t.Fatalf("url = %q", results[0].URL)
	}
}
