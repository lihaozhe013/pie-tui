package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCodemodePreferenceEditsMinimally(t *testing.T) {
	cases := []struct {
		entries    []string
		present    bool
		preference string
		want       []string
		keep       bool
	}{
		{nil, false, "always", []string{"+codemode"}, true},
		{[]string{"+codemode"}, true, "automatic", nil, false},
		{[]string{"read", "+codemode"}, true, "automatic", []string{"read"}, true},
		{[]string{"+codemode", "+goal"}, true, "automatic", []string{"+goal"}, true},
		{[]string{}, true, "automatic", nil, false},
		{[]string{"read"}, true, "always", []string{"read", "+codemode"}, true},
	}
	for _, test := range cases {
		got, keep, err := WithCodemodePreference(test.entries, test.present, test.preference)
		if err != nil {
			t.Fatalf("%v: %v", test.entries, err)
		}
		if keep != test.keep || !equalStrings(got, test.want) {
			t.Fatalf("%v -> %v %v, want %v %v", test.entries, got, keep, test.want, test.keep)
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSettingsCodemodeFields(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", dir)
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(`{"theme":"dark","codemode":{"mode":"only","inlineBudget":500}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	paths, _ := ResolvePaths()
	settings, err := LoadSettings(paths)
	if err != nil {
		t.Fatal(err)
	}
	if pref, _ := settings.CodemodePreference(); pref != "automatic" {
		t.Fatalf("preference = %q", pref)
	}
	if err := settings.SetCodemodePreference("always"); err != nil {
		t.Fatalf("always: %v", err)
	}
	if pref, _ := settings.CodemodePreference(); pref != "always" {
		t.Fatalf("preference = %q", pref)
	}
	if mode, _ := settings.CodemodeMode(); mode != "only" {
		t.Fatalf("mode = %q", mode)
	}
	if budget, ok, _ := settings.CodemodeInlineBudget(); !ok || budget != 500 {
		t.Fatalf("budget = %d %v", budget, ok)
	}
	if err := settings.SetCodemodeMode("on"); err != nil {
		t.Fatal(err)
	}
	if err := settings.SetCodemodeInlineBudget(nil); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) == "" || contains(data, `"mode"`) || contains(data, `"inlineBudget"`) {
		t.Fatalf("settings = %s", data)
	}
	if !contains(data, `"theme": "dark"`) || !contains(data, `"defaultTools"`) {
		t.Fatalf("unknown keys or preference lost: %s", data)
	}
}

func contains(data []byte, text string) bool {
	return len(data) > 0 && len(text) > 0 && (string(data) != "" && indexOf(string(data), text) >= 0)
}

func indexOf(value, text string) int {
	for i := 0; i+len(text) <= len(value); i++ {
		if value[i:i+len(text)] == text {
			return i
		}
	}
	return -1
}
