package skills

import (
	"strings"
	"testing"
)

func TestSetDisableModelInvocationInsertsAndReplaces(t *testing.T) {
	content := "---\nname: pdf\ndescription: Work with PDFs.\n---\n\n# PDF\n"
	updated, err := SetDisableModelInvocation(content, true)
	if err != nil {
		t.Fatalf("disable: %v", err)
	}
	if !strings.HasPrefix(updated, "---\ndisable-model-invocation: true\nname: pdf") {
		t.Fatalf("insert = %q", updated)
	}
	if got := ParseFrontmatter(updated); got.Values[disableKey] != "true" || got.Values["name"] != "pdf" {
		t.Fatalf("parsed = %+v", got.Values)
	}

	updated, err = SetDisableModelInvocation(updated, false)
	if err != nil {
		t.Fatalf("enable: %v", err)
	}
	if strings.Contains(updated, disableKey) {
		t.Fatalf("key not removed: %q", updated)
	}
	if updated != content {
		t.Fatalf("round trip changed the file:\n%q\n%q", content, updated)
	}
}

func TestSetDisableModelInvocationReplacesFalse(t *testing.T) {
	content := "---\n# comment\ndescription: x\ndisable-model-invocation: false\n---\nbody\n"
	updated, err := SetDisableModelInvocation(content, true)
	if err != nil {
		t.Fatalf("disable: %v", err)
	}
	if strings.Contains(updated, "false") || !strings.Contains(updated, "disable-model-invocation: true") {
		t.Fatalf("updated = %q", updated)
	}
	if !strings.HasSuffix(updated, "---\nbody\n") {
		t.Fatalf("body changed: %q", updated)
	}

	again, err := SetDisableModelInvocation(updated, false)
	if err != nil {
		t.Fatalf("enable: %v", err)
	}
	if strings.Contains(again, disableKey) || !strings.Contains(again, "# comment") {
		t.Fatalf("removal = %q", again)
	}
}

func TestSetDisableModelInvocationWithoutFrontmatter(t *testing.T) {
	content := "# Notes\n\nThe body mentions disable-model-invocation: true.\n"
	updated, err := SetDisableModelInvocation(content, true)
	if err != nil {
		t.Fatalf("disable: %v", err)
	}
	if !strings.HasPrefix(updated, "---\ndisable-model-invocation: true\n---\n# Notes") {
		t.Fatalf("frontmatter not created: %q", updated)
	}
	if !strings.Contains(updated, "The body mentions disable-model-invocation: true.") {
		t.Fatal("body edited")
	}

	restored, err := SetDisableModelInvocation(updated, false)
	if err != nil {
		t.Fatalf("enable: %v", err)
	}
	if _, present := ParseFrontmatter(restored).Values[disableKey]; present {
		t.Fatalf("key still present: %q", restored)
	}
	if !strings.Contains(restored, "The body mentions disable-model-invocation: true.") {
		t.Fatalf("body changed: %q", restored)
	}
}

func TestSetDisableModelInvocationCRLF(t *testing.T) {
	content := "---\r\nname: x\r\ndescription: y\r\n---\r\nbody\r\n"
	updated, err := SetDisableModelInvocation(content, true)
	if err != nil {
		t.Fatalf("disable: %v", err)
	}
	if !strings.Contains(updated, "\r\ndisable-model-invocation: true\r\nname: x") {
		t.Fatalf("crlf insert = %q", updated)
	}
}

func TestParseFrontmatterBlocks(t *testing.T) {
	content := "---\nname: review\ndescription: >\n  Review a change\n  before merging.\ndisable-model-invocation: true\n---\n# Body\n"
	parsed := ParseFrontmatter(content)
	if parsed.Values["description"] != "Review a change before merging." {
		t.Fatalf("description = %q", parsed.Values["description"])
	}
	if parsed.Values[disableKey] != "true" || parsed.Body != "# Body" {
		t.Fatalf("parsed = %+v", parsed)
	}
	if !strings.Contains(content, "before merging.") {
		t.Fatal("fixture")
	}
}
