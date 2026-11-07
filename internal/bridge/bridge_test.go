package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureWritesPrivateExtensionAndReplacesOldVersions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	old := filepath.Join(dir, "bridge", "pie-bridge-000000000000.ts")
	if err := os.MkdirAll(filepath.Dir(old), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(old, []byte("old"), 0o600); err != nil {
		t.Fatalf("write old: %v", err)
	}

	path, err := Ensure(dir)
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if !strings.HasSuffix(path, ".ts") || !strings.Contains(filepath.Base(path), Name+"-") {
		t.Fatalf("path = %q", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), "pie-tools") {
		t.Fatalf("content = %q", data)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("old bridge file not removed: %v", err)
	}

	again, err := Ensure(dir)
	if err != nil || again != path {
		t.Fatalf("second Ensure = %q %v", again, err)
	}
}
