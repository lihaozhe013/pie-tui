package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lihaozhe/pie-tui/internal/skills"
)

func TestSkillsTabListsAndToggles(t *testing.T) {
	root := t.TempDir()
	agent := filepath.Join(root, "agent")
	project := filepath.Join(root, "project")
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("PI_CODING_AGENT_DIR", agent)
	path := filepath.Join(agent, "skills", "alpha", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("---\nname: alpha\ndescription: Test skill\n---\n# Alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := New(newFakeClient(), Options{CWD: project})
	tab := m.skillsSettingsTab()
	var skillRow *settingsRow
	for i := range tab.Rows {
		if tab.Rows[i].ID == "skill-alpha" {
			skillRow = &tab.Rows[i]
		}
	}
	if skillRow == nil {
		t.Fatalf("skill row missing: %+v", tab.Rows)
	}
	if !strings.Contains(skillRow.Current(), "enabled") {
		t.Fatalf("status = %q", skillRow.Current())
	}

	skillRow.Run(m, nil)
	detail := m.settingsPanel
	if detail == nil {
		t.Fatal("detail panel not opened")
	}
	var toggle *settingsRow
	for i := range detail.tabs[0].Rows {
		if detail.tabs[0].Rows[i].ID == "skill-invocation" {
			toggle = &detail.tabs[0].Rows[i]
		}
	}
	if toggle == nil || toggle.SetBool == nil {
		t.Fatal("toggle missing")
	}
	if err := toggle.SetBool(false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	content, _ := os.ReadFile(path)
	if !strings.Contains(string(content), "disable-model-invocation: true") {
		t.Fatalf("file = %s", content)
	}
	list, _ := skills.Load(agent, project, nil)
	if len(list) != 1 || !list[0].Disabled {
		t.Fatalf("list = %+v", list)
	}
}
