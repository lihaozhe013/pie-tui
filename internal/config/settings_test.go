package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testPaths(t *testing.T) Paths {
	t.Helper()
	dir := t.TempDir()
	return Paths{Dir: dir}
}

func TestSettingsReadsAndWrites(t *testing.T) {
	t.Parallel()

	paths := testPaths(t)
	writeFile(t, paths.Settings(), `{
  "defaultProvider": "p",
  "defaultModel": "m",
  "defaultThinkingLevel": "high",
  "enabledModels": ["p/m", "p/*"],
  "modelThinkingLevels": {"p/m": "high"},
  "showCacheMissNotices": true,
  "defaultTools": ["+codemode"],
  "untouched": {"nested": 1}
}`)

	settings, err := LoadSettings(paths)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if provider, _ := settings.DefaultProvider(); provider != "p" {
		t.Fatalf("provider = %q", provider)
	}
	if level, _ := settings.DefaultThinkingLevel(); level != "high" {
		t.Fatalf("level = %q", level)
	}
	models, present, err := settings.EnabledModels()
	if err != nil || !present || len(models) != 2 {
		t.Fatalf("enabledModels = %v %v %v", models, present, err)
	}
	levels, err := settings.ModelThinkingLevels()
	if err != nil || levels["p/m"] != "high" {
		t.Fatalf("levels = %v %v", levels, err)
	}
	if notices, _ := settings.ShowCacheMissNotices(); !notices {
		t.Fatal("notices = false")
	}
	tools, present, err := settings.DefaultTools()
	if err != nil || !present || len(tools) != 1 || tools[0] != "+codemode" {
		t.Fatalf("defaultTools = %v %v %v", tools, present, err)
	}

	if err := settings.SetEnabledModels([]string{"p/m"}); err != nil {
		t.Fatalf("SetEnabledModels: %v", err)
	}
	if err := settings.SetModelThinkingLevel("p/m", "low"); err != nil {
		t.Fatalf("SetModelThinkingLevel: %v", err)
	}
	if err := settings.SetShowCacheMissNotices(false); err != nil {
		t.Fatalf("SetShowCacheMissNotices: %v", err)
	}

	reloaded, err := LoadSettings(paths)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	models, _, _ = reloaded.EnabledModels()
	if len(models) != 1 || models[0] != "p/m" {
		t.Fatalf("models = %v", models)
	}
	levels, _ = reloaded.ModelThinkingLevels()
	if levels["p/m"] != "low" {
		t.Fatalf("levels = %v", levels)
	}
	if reloaded.Root().GetPath("untouched", "nested").IntOr(0) != 1 {
		t.Fatalf("unknown key lost: %s", Marshal(reloaded.Root()))
	}

	if err := reloaded.ClearEnabledModels(); err != nil {
		t.Fatalf("ClearEnabledModels: %v", err)
	}
	if _, present, _ := reloaded.EnabledModels(); present {
		t.Fatal("enabledModels still present")
	}
	if err := reloaded.SetModelThinkingLevel("p/m", "extreme"); err == nil {
		t.Fatal("expected an invalid level error")
	}
}

func TestSettingsRejectsWrongTypes(t *testing.T) {
	t.Parallel()

	paths := testPaths(t)
	writeFile(t, paths.Settings(), `{"enabledModels": "p/m"}`)
	settings, err := LoadSettings(paths)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if _, _, err := settings.EnabledModels(); err == nil {
		t.Fatal("expected a type error")
	}
	var typeErr *TypeError
	if _, _, err := settings.EnabledModels(); err == nil || !asType(err, &typeErr) {
		t.Fatalf("error = %v", err)
	}
}

func asType(err error, target **TypeError) bool {
	for err != nil {
		if typed, ok := err.(*TypeError); ok {
			*target = typed
			return true
		}
		unwrapper, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = unwrapper.Unwrap()
	}
	return false
}

func TestPresetRulesAndRoundTrip(t *testing.T) {
	t.Parallel()

	tools, err := NormalizePresetTools([]string{"powershell", "read", "read", "custom_tool"})
	if err != nil {
		t.Fatalf("NormalizePresetTools: %v", err)
	}
	want := []string{"bash", "read", "custom_tool"}
	if strings.Join(tools, ",") != strings.Join(want, ",") {
		t.Fatalf("tools = %v", tools)
	}
	if _, err := NormalizePresetTools(nil); err == nil {
		t.Fatal("expected an empty-list error")
	}
	if _, err := NormalizePresetTools([]string{"bad tool"}); err == nil {
		t.Fatal("expected an invalid-name error")
	}

	presets := []CustomPreset{
		{ID: "p1", Name: "Review", Tools: []string{"bash", "read"}, IncludeExtensions: false},
		{ID: "p2", Name: "Docs", Tools: []string{"read", "grep"}, IncludeExtensions: true},
	}
	if err := ValidatePresets(presets); err != nil {
		t.Fatalf("ValidatePresets: %v", err)
	}
	if err := ValidatePresets([]CustomPreset{
		{ID: "p1", Name: "One", Tools: []string{"read"}},
		{ID: "p2", Name: "one", Tools: []string{"read"}},
	}); err == nil {
		t.Fatal("expected a duplicate-name error")
	}

	paths := testPaths(t)
	file := LoadPresets(paths)
	if err := file.Save(presets); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(paths.Presets())
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}

	again := LoadPresets(paths)
	if again.Invalid() != nil {
		t.Fatalf("invalid: %v", again.Invalid())
	}
	if len(again.Presets()) != 2 || again.Presets()[0].Name != "Review" {
		t.Fatalf("presets = %+v", again.Presets())
	}
	if again.Presets()[0].IncludeExtensions {
		t.Fatal("includeExtensions lost")
	}

	raw, _ := os.ReadFile(paths.Presets())
	if !strings.Contains(string(raw), `"version": 1`) {
		t.Fatalf("file = %s", raw)
	}
}

func TestPresetFileRecoversCorruptContent(t *testing.T) {
	t.Parallel()

	paths := testPaths(t)
	writeFile(t, paths.Presets(), "{ nope")
	file := LoadPresets(paths)
	if file.Invalid() == nil {
		t.Fatal("expected invalid")
	}
	if len(file.Presets()) != 0 {
		t.Fatalf("presets = %+v", file.Presets())
	}
	if err := file.Save([]CustomPreset{{ID: "p1", Name: "One", Tools: []string{"read"}}}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	entries, err := filepath.Glob(paths.Presets() + ".corrupt-*")
	if err != nil || len(entries) == 0 {
		t.Fatalf("backup missing: %v %v", entries, err)
	}
	if len(LoadPresets(paths).Presets()) != 1 {
		t.Fatalf("preset not saved")
	}
}

func TestPresetSelectionLookup(t *testing.T) {
	t.Parallel()

	custom := []CustomPreset{{ID: "p1", Name: "Mine", Tools: []string{"read", "grep"}}}
	if tools, _, ok := LookupSelection("custom:p1", custom); !ok || len(tools) != 2 {
		t.Fatalf("custom = %v %v", tools, ok)
	}
	if tools, _, ok := LookupSelection("custom:gone", custom); ok {
		t.Fatalf("missing custom resolved: %v", tools)
	}
	if tools, _, ok := LookupSelection("read-only", custom); !ok || len(tools) != 4 {
		t.Fatalf("read-only = %v %v", tools, ok)
	}
	if tools, _, ok := LookupSelection("configured", custom); !ok || tools != nil {
		t.Fatalf("configured = %v %v", tools, ok)
	}
	if got := PresetForToolNames([]string{"read", "grep", "find", "ls"}, "", custom); got != "read-only" {
		t.Fatalf("inferred = %q", got)
	}
	if got := PresetForToolNames([]string{"read"}, "plan", custom); got != "plan" {
		t.Fatalf("plan = %q", got)
	}
	if got := PresetForToolNames([]string{"read", "grep"}, "", custom); got != "custom:p1" {
		t.Fatalf("custom = %q", got)
	}
}

func TestModelSettingsContextWindow(t *testing.T) {
	t.Parallel()

	paths := testPaths(t)
	settings, err := LoadModelSettings(paths)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if limit, capped := settings.ContextWindowLimit(); limit != DefaultContextWindowLimit || !capped {
		t.Fatalf("default = %d %v", limit, capped)
	}

	if err := settings.SetContextWindowLimit(nil); err != nil {
		t.Fatalf("set null: %v", err)
	}
	settings, _ = LoadModelSettings(paths)
	if _, capped := settings.ContextWindowLimit(); capped {
		t.Fatal("explicit null should mean no cap")
	}

	limit := 200_000
	if err := settings.SetContextWindowLimit(&limit); err != nil {
		t.Fatalf("set limit: %v", err)
	}
	settings, _ = LoadModelSettings(paths)
	if got, capped := settings.ContextWindowLimit(); got != limit || !capped {
		t.Fatalf("limit = %d %v", got, capped)
	}

	writeFile(t, paths.ModelSettings(), `{"contextWindowLimit": "not a number", "other": true}`)
	settings, _ = LoadModelSettings(paths)
	if got, capped := settings.ContextWindowLimit(); got != DefaultContextWindowLimit || !capped {
		t.Fatalf("invalid = %d %v", got, capped)
	}
	if err := settings.SetContextWindowLimit(&limit); err != nil {
		t.Fatalf("set: %v", err)
	}
	doc, _ := LoadObject(paths.ModelSettings())
	if !doc.Root().Get("other").BoolOr(false) {
		t.Fatal("other key lost")
	}
}

func TestTUIConfigDefaultsAndWrites(t *testing.T) {
	t.Parallel()

	paths := testPaths(t)
	cfg, err := LoadTUIConfig(paths)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Theme() != "auto" || !cfg.EnterToSend() {
		t.Fatalf("defaults: theme=%q enter=%v", cfg.Theme(), cfg.EnterToSend())
	}
	if err := cfg.SetTheme("dusk"); err != nil {
		t.Fatalf("theme: %v", err)
	}
	if err := cfg.SetEnterToSend(false); err != nil {
		t.Fatalf("enter: %v", err)
	}
	if err := cfg.SetGoalEnabled(true); err != nil {
		t.Fatalf("goal: %v", err)
	}
	if err := cfg.SetLastCWD("/tmp/project"); err != nil {
		t.Fatalf("cwd: %v", err)
	}

	again, err := LoadTUIConfig(paths)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if again.Theme() != "dusk" || again.EnterToSend() ||
		!again.GoalEnabled() || again.LastCWD() != "/tmp/project" {
		t.Fatalf("reloaded: %+v", again)
	}
}

func TestPresetFileMissingIsEmptyAndValid(t *testing.T) {
	t.Parallel()

	file := LoadPresets(testPaths(t))
	if file.Invalid() != nil {
		t.Fatalf("missing file reported invalid: %v", file.Invalid())
	}
	if len(file.Presets()) != 0 {
		t.Fatalf("presets = %+v", file.Presets())
	}
}
