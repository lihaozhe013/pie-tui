package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectsGroupsSessionsByCWD(t *testing.T) {
	dir := t.TempDir()
	write := func(group, name, cwd string) {
		path := filepath.Join(dir, group, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		content := `{"type":"session","version":1,"id":"` + name + `","cwd":"` + cwd + `"}` + "\n"
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a", "one.jsonl", "/work/project")
	write("a", "two.jsonl", "/work/project")
	write("b", "three.jsonl", "/work/other")

	projects, err := Projects(dir)
	if err != nil {
		t.Fatalf("projects: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("projects = %+v", projects)
	}
	paths := map[string]int{}
	for _, project := range projects {
		paths[project.Path] = project.Sessions
	}
	if paths["/work/project"] != 2 || paths["/work/other"] != 1 {
		t.Fatalf("counts = %v", paths)
	}
}
