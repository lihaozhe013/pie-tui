package ui

import (
	"strings"
	"testing"
)

func TestCompletionFiltersByPrefix(t *testing.T) {
	t.Parallel()

	list := &completion{}
	list.SetItems(append(builtinCommands(),
		completionItem{Value: "/fix-tests", Detail: "fix failing tests", Kind: "prompt"},
	))

	list.Refresh("/")
	if !list.visible || len(list.filtered) < len(builtinCommands()) {
		t.Fatalf("expected all builtins, got %+v", list.filtered)
	}

	list.Refresh("/mo")
	if !list.visible || len(list.filtered) != 1 || list.filtered[0].Value != "/model" {
		t.Fatalf("filtered = %+v", list.filtered)
	}

	list.Refresh("/fix")
	if !list.visible || list.filtered[0].Value != "/fix-tests" {
		t.Fatalf("filtered = %+v", list.filtered)
	}

	list.Refresh("/model extra")
	if list.visible {
		t.Fatal("arguments must hide the popup")
	}

	list.Refresh("hello")
	if list.visible {
		t.Fatal("plain text must hide the popup")
	}
}

func TestCompletionMovesAndWraps(t *testing.T) {
	t.Parallel()

	list := &completion{}
	list.SetItems(builtinCommands())
	list.Refresh("/")

	first, _ := list.Selected()
	list.Move(-1)
	last, _ := list.Selected()
	if first.Value == last.Value {
		t.Fatal("expected wrap-around selection")
	}
	if last.Value != list.filtered[len(list.filtered)-1].Value {
		t.Fatalf("last = %q", last.Value)
	}

	list.Move(1)
	back, _ := list.Selected()
	if back.Value != first.Value {
		t.Fatalf("back = %q, want %q", back.Value, first.Value)
	}
}

func TestComposerHistory(t *testing.T) {
	t.Parallel()

	c := newComposer(newStyles(defaultTheme()))
	c.PushHistory("first")
	c.PushHistory("second")
	c.SetText("draft")

	if !c.HistoryPrev() {
		t.Fatal("expected history")
	}
	if c.Value() != "second" {
		t.Fatalf("value = %q", c.Value())
	}
	c.HistoryPrev()
	if c.Value() != "first" {
		t.Fatalf("value = %q", c.Value())
	}
	if c.HistoryPrev() {
		t.Fatal("history must stop at the oldest entry")
	}
	c.HistoryNext()
	c.HistoryNext()
	if c.Value() != "draft" {
		t.Fatalf("value = %q", c.Value())
	}
	if c.HistoryNext() {
		t.Fatal("history must stop at the draft")
	}
}

func TestComposerHeightFollowsNewlines(t *testing.T) {
	t.Parallel()

	c := newComposer(newStyles(defaultTheme()))
	if c.Height() != 1 {
		t.Fatalf("height = %d", c.Height())
	}
	c.SetText("a\nb\nc")
	if c.Height() != 3 {
		t.Fatalf("height = %d", c.Height())
	}
	c.Reset()
	if c.Height() != 1 {
		t.Fatalf("height after reset = %d", c.Height())
	}
}

func TestOverlayPickerFiltersAndSelects(t *testing.T) {
	t.Parallel()

	items := []pickerItem{
		{Value: "anthropic/claude", Label: "anthropic/claude", Detail: "Claude"},
		{Value: "openai/gpt", Label: "openai/gpt", Detail: "GPT"},
	}
	overlay := newPickerOverlay("model", "Switch model", items)
	overlay.Resize(80)

	overlay.filter.SetValue("gpt")
	overlay.refilter()
	if len(overlay.filtered) != 1 {
		t.Fatalf("filtered = %+v", overlay.filtered)
	}
	selected, ok := overlay.selected()
	if !ok || selected.Value != "openai/gpt" {
		t.Fatalf("selected = %+v", selected)
	}

	view := overlay.View(newStyles(defaultTheme()))
	if !strings.Contains(view, "Switch model") {
		t.Fatalf("title missing from overlay: %q", view)
	}
}

func TestComposerIgnoresBlankHistory(t *testing.T) {
	t.Parallel()

	c := newComposer(newStyles(defaultTheme()))
	c.PushHistory("   ")
	if c.HistoryPrev() {
		t.Fatal("blank prompts must not enter history")
	}
}
