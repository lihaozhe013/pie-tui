package ui

import "testing"

func TestPaletteOpensWithCtrlPAndKeepsDraft(t *testing.T) {
	model, _ := newTestModel(t)
	model.composer.SetText("half-written prompt")
	model.composer.ta.CursorEnd()

	model, _ = step(t, model, keyMsg("ctrl+p"))
	if model.overlay == nil || model.overlay.kind != overlayPicker || model.overlay.action != "palette" {
		t.Fatalf("palette did not open: %+v", model.overlay)
	}
	if got := model.composer.Value(); got != "half-written prompt" {
		t.Fatalf("draft changed: %q", got)
	}

	values := make(map[string]bool)
	for _, item := range model.overlay.items {
		values[item.Value] = true
	}
	for _, want := range []string{"/settings", "/model", "/fix-tests"} {
		if !values[want] {
			t.Fatalf("palette is missing %q: %+v", want, values)
		}
	}
	if values["/models"] {
		t.Fatal("palette must list canonical commands, not aliases")
	}
}

func TestPaletteWinsOverCompletion(t *testing.T) {
	model, _ := newTestModel(t)
	model.composer.SetText("/mo")
	model.completion.Refresh(model.composer.Value())
	if !model.completion.visible {
		t.Fatal("completion did not open")
	}

	model, _ = step(t, model, keyMsg("ctrl+p"))
	if model.overlay == nil || model.overlay.action != "palette" {
		t.Fatalf("palette did not open: %+v", model.overlay)
	}
	if model.completion.visible {
		t.Fatal("completion stayed visible under the palette")
	}
}

func TestPaletteRunsBuiltinWithoutClearingDraft(t *testing.T) {
	model, _ := newTestModel(t)
	model.composer.SetText("draft text")

	model, _ = step(t, model, keyMsg("ctrl+p"))
	model.overlay.filter.SetValue("/model")
	model.overlay.refilter()
	model.overlay.cursor = 0

	model, cmd := step(t, model, keyMsg("enter"))
	runCmd(t, cmd)

	if model.overlay == nil || model.overlay.action != "model" {
		t.Fatalf("model picker did not open: %+v", model.overlay)
	}
	if got := model.composer.Value(); got != "draft text" {
		t.Fatalf("draft changed: %q", got)
	}
}

func TestPaletteRunsPiCommandWithoutClearingDraft(t *testing.T) {
	model, client := newTestModel(t)
	model.composer.SetText("keep this draft")

	model, _ = step(t, model, keyMsg("ctrl+p"))
	model.overlay.filter.SetValue("fix-tests")
	model.overlay.refilter()
	model.overlay.cursor = 0

	model, cmd := step(t, model, keyMsg("enter"))
	runCmd(t, cmd)

	if model.overlay != nil {
		t.Fatalf("palette stayed open: %+v", model.overlay)
	}
	command, ok := client.lastCommand("prompt")
	if !ok || command["message"] != "/fix-tests" {
		t.Fatalf("prompt = %v (ok=%v)", command, ok)
	}
	if got := model.composer.Value(); got != "keep this draft" {
		t.Fatalf("draft changed: %q", got)
	}
}

func TestPaletteFilterFindsAliases(t *testing.T) {
	model, _ := newTestModel(t)
	model, _ = step(t, model, keyMsg("ctrl+p"))

	model.overlay.filter.SetValue("exit")
	model.overlay.refilter()
	selected, ok := model.overlay.selected()
	if !ok || selected.Value != "/quit" {
		t.Fatalf("selected = %+v (ok=%v)", selected, ok)
	}
}
