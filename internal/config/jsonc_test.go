package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestParseJSONCLeniencyAndOrder(t *testing.T) {
	t.Parallel()

	raw := "\xEF\xBB\xBF{\n  // a comment\n  \"zeta\": 1,\n  \"alpha\": \"http://example.com//x\", /* block */\n  \"list\": [1, 2,],\n}\n"
	value, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	keys := value.Keys()
	if len(keys) != 3 || keys[0] != "zeta" || keys[1] != "alpha" || keys[2] != "list" {
		t.Fatalf("keys = %v", keys)
	}
	if got, _ := value.Get("alpha").AsString(); got != "http://example.com//x" {
		t.Fatalf("string = %q", got)
	}
	if value.Get("list").Len() != 2 {
		t.Fatalf("list = %v", value.Get("list").Interface())
	}
}

func TestParseRejectsTrailingValue(t *testing.T) {
	t.Parallel()

	if _, err := Parse([]byte(`{} {}`)); err == nil {
		t.Fatal("expected an error")
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	t.Parallel()

	raw := "{\n  \"a\": [1, true, null, \"x\"],\n  \"b\": {\"nested\": 1.5}\n}\n"
	value, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out := string(Marshal(value))
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("missing trailing newline: %q", out)
	}
	again, err := Parse([]byte(out))
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if again.Keys()[0] != "a" || again.Keys()[1] != "b" {
		t.Fatalf("order lost: %s", out)
	}
	left, _ := json.Marshal(value.Interface())
	right, _ := json.Marshal(again.Interface())
	if string(left) != string(right) {
		t.Fatalf("round trip changed data:\n%s\n%s", left, right)
	}
}

func TestMarshalEscapesWithoutHTMLEntities(t *testing.T) {
	t.Parallel()

	root := NewObject()
	root.SetString("url", "https://example.com/?a=1&b=<2>")
	if got := string(Marshal(root)); strings.Contains(got, `\u003c`) || strings.Contains(got, `\u0026`) {
		t.Fatalf("HTML escaping leaked: %s", got)
	}
}

func TestFromAnyIsDeterministic(t *testing.T) {
	t.Parallel()

	value, err := FromAny(map[string]any{"b": true, "a": []any{1, "two"}})
	if err != nil {
		t.Fatalf("FromAny: %v", err)
	}
	if value.Keys()[0] != "a" || value.Keys()[1] != "b" {
		t.Fatalf("keys = %v", value.Keys())
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestLoadMissingReadsAsEmptyObject(t *testing.T) {
	t.Parallel()

	doc, err := LoadObject(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatalf("LoadObject: %v", err)
	}
	if doc.Existed() || doc.Root().Len() != 0 {
		t.Fatalf("doc = %+v", doc)
	}
}

func TestLoadCorruptKeepsFileAndBacksItUp(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "settings.json")
	writeFile(t, path, "{ not json")

	_, err := LoadObject(path)
	if err == nil {
		t.Fatal("expected an error")
	}
	var syntax *SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("error = %T %v", err, err)
	}
	if syntax.Backup == "" {
		t.Fatal("expected a backup")
	}
	if _, statErr := os.Stat(syntax.Backup); statErr != nil {
		t.Fatalf("backup missing: %v", statErr)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil || string(data) != "{ not json" {
		t.Fatalf("original changed: %q %v", data, readErr)
	}
}

func TestUpdateCreatesPrivateFileAndPreservesUnknownKeys(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", "settings.json")
	writeFile(t, path, `{"keep": {"deep": true}, "enabledModels": ["a"]}`)

	if err := Update(path, func(root *Value) error {
		root.SetString("defaultModel", "m")
		return nil
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("mode = %o", mode)
	}
	doc, err := LoadObject(path)
	if err != nil {
		t.Fatalf("LoadObject: %v", err)
	}
	if doc.Root().GetPath("keep", "deep").BoolOr(false) != true {
		t.Fatalf("unknown key lost: %s", Marshal(doc.Root()))
	}
	if got, _ := doc.Root().Get("defaultModel").AsString(); got != "m" {
		t.Fatalf("defaultModel = %q", got)
	}
}

func TestUpdateRefusesCorruptFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "settings.json")
	writeFile(t, path, "{ broken")

	err := Update(path, func(root *Value) error {
		root.SetString("x", "y")
		return nil
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "{ broken" {
		t.Fatalf("file was replaced: %q", data)
	}
}

func TestUpdateSerializesConcurrentWriters(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "settings.json")
	const writers = 12
	var wg sync.WaitGroup
	errs := make(chan error, writers)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- Update(path, func(root *Value) error {
				current, _ := root.Get("count").AsInt()
				root.SetNumber("count", float64(current+1))
				return nil
			})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
	}

	doc, err := LoadObject(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := doc.Root().Get("count").IntOr(0); got != writers {
		t.Fatalf("count = %d, want %d", got, writers)
	}
}

func TestUpdateFollowsSymlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "dotfiles-settings.json")
	writeFile(t, target, `{"a":1}`)
	link := filepath.Join(dir, "settings.json")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if err := Update(link, func(root *Value) error {
		root.SetBool("b", true)
		return nil
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	info, err := os.Lstat(link)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("link replaced: %v %v", info, err)
	}
	doc, err := LoadObject(target)
	if err != nil {
		t.Fatalf("target: %v", err)
	}
	if !doc.Root().Get("b").BoolOr(false) {
		t.Fatalf("target not updated: %s", Marshal(doc.Root()))
	}
}
