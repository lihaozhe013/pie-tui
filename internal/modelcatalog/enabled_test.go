package modelcatalog

import (
	"reflect"
	"testing"
)

func testModels() []*Model {
	return []*Model{
		{ProviderID: "p", ID: "a"},
		{ProviderID: "p", ID: "b"},
		{ProviderID: "q", ID: "c"},
	}
}

func TestEnabledSet(t *testing.T) {
	t.Parallel()

	models := testModels()
	enabled, _, _ := EnabledSet(nil, false, models)
	if len(enabled) != 3 {
		t.Fatalf("all = %v", enabled)
	}

	enabled, pins, resolutions := EnabledSet([]string{"p/*", "q/c:high", "missing/*"}, true, models)
	if !enabled["p/a"] || !enabled["p/b"] || !enabled["q/c"] {
		t.Fatalf("enabled = %v", enabled)
	}
	if pins["q/c"] != "high" {
		t.Fatalf("pins = %v", pins)
	}
	if stale := StalePatterns(resolutions); len(stale) != 1 || stale[0] != "missing/*" {
		t.Fatalf("stale = %v", stale)
	}
}

func TestSetEnabledEditsMinimally(t *testing.T) {
	t.Parallel()

	models := testModels()
	patterns := []string{"p/*", "stale/*"}
	next, set := SetEnabled(patterns, true, models, "p/a", false)
	if !set || !reflect.DeepEqual(next, []string{"p/b", "stale/*"}) {
		t.Fatalf("disable = %v", next)
	}
	next, _ = SetEnabled(next, true, models, "p/a", true)
	if !reflect.DeepEqual(next, []string{"p/b", "stale/*", "p/a"}) {
		t.Fatalf("enable = %v", next)
	}

	next, _ = SetEnabled(next, true, models, "p/b", false)
	if !reflect.DeepEqual(next, []string{"stale/*", "p/a"}) {
		t.Fatalf("second disable = %v", next)
	}

	// Turning everything off materializes every other model.
	next, _ = SetEnabled(nil, false, models, "p/a", false)
	if !reflect.DeepEqual(next, []string{"p/b", "q/c"}) {
		t.Fatalf("materialize = %v", next)
	}
}

func TestEnabledPatternsMatchPisRules(t *testing.T) {
	t.Parallel()

	models := []*Model{
		{ProviderID: "zai", ID: "glm-5.3", Name: "GLM-5.3"},
		{ProviderID: "zai", ID: "glm-5.3-highspeed", Name: "GLM-5.3 Highspeed"},
		{ProviderID: "p", ID: "a/b"},
	}
	enabled, _, _ := EnabledSet([]string{"zai/glm-5.3"}, true, models)
	if !enabled["zai/glm-5.3"] || enabled["zai/glm-5.3-highspeed"] {
		t.Fatalf("qualified exact = %v", enabled)
	}
	enabled, _, _ = EnabledSet([]string{"glm-5.3"}, true, models)
	if !enabled["zai/glm-5.3"] || enabled["zai/glm-5.3-highspeed"] {
		t.Fatalf("bare exact = %v", enabled)
	}
	enabled, _, _ = EnabledSet([]string{"p/*"}, true, models)
	if enabled["p/a/b"] {
		t.Fatalf("single star crossed a slash: %v", enabled)
	}
	enabled, _, _ = EnabledSet([]string{"p/**"}, true, models)
	if !enabled["p/a/b"] {
		t.Fatalf("double star missed a nested id: %v", enabled)
	}
	enabled, _, _ = EnabledSet([]string{"highspeed"}, true, models)
	if !enabled["zai/glm-5.3-highspeed"] || enabled["zai/glm-5.3"] {
		t.Fatalf("fuzzy best match = %v", enabled)
	}
}
