package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSwitchProjectRestartsAndClears(t *testing.T) {
	root := t.TempDir()
	dirA := filepath.Join(root, "a")
	dirB := filepath.Join(root, "b")
	for _, dir := range []string{dirA, dirB} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PI_CODING_AGENT_DIR", filepath.Join(root, "agent"))

	oldClient := newFakeClient()
	next := newFakeClient()
	m := New(oldClient, Options{
		CWD: dirA,
		Restart: func(dir string) (Client, error) {
			if dir != dirB {
				t.Fatalf("restart dir = %q", dir)
			}
			return next, nil
		},
	})
	m.addNotice("old transcript")

	cmd := m.switchProject(dirB)
	if cmd == nil {
		t.Fatal("switch returned no command")
	}
	msg, ok := cmd().(projectSwitchedMsg)
	if !ok || msg.err != nil {
		t.Fatalf("msg = %+v", msg)
	}
	model, _ := m.Update(msg)
	m = model.(*Model)
	if m.opts.CWD != dirB || m.client != next {
		t.Fatalf("state = %q, %v", m.opts.CWD, m.client == next)
	}
	if m.restarting {
		t.Fatal("restarting flag stuck")
	}
	if !strings.Contains(m.pendingNotice, "new session in") {
		t.Fatalf("notice = %q", m.pendingNotice)
	}
	if m.prefs.LastCWD() != dirB {
		t.Fatalf("lastCwd = %q", m.prefs.LastCWD())
	}

	// The old pump's close must not mark the new client disconnected.
	model, _ = m.Update(closedMsg{client: oldClient})
	m = model.(*Model)
	if m.disconnected {
		t.Fatal("stale close treated as a disconnect")
	}
	model, _ = m.Update(closedMsg{client: next})
	m = model.(*Model)
	if !m.disconnected {
		t.Fatal("current close ignored")
	}
}

func TestSwitchProjectRefusesBadDirectories(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", t.TempDir())
	m := New(newFakeClient(), Options{
		CWD:     t.TempDir(),
		Restart: func(dir string) (Client, error) { return newFakeClient(), nil },
	})
	if cmd := m.switchProject(filepath.Join(m.opts.CWD, "missing")); cmd != nil {
		t.Fatal("missing directory accepted")
	}
	m.busy = true
	if cmd := m.switchProject(t.TempDir()); cmd != nil {
		t.Fatal("switch while busy accepted")
	}
}

func TestDirectoryPickerListsSubdirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "alpha"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".hidden"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PI_CODING_AGENT_DIR", t.TempDir())
	m := New(newFakeClient(), Options{CWD: root})
	m.openDirectoryPicker(root)
	if m.overlay == nil {
		t.Fatal("picker not open")
	}
	values := map[string]bool{}
	for _, item := range m.overlay.items {
		values[item.Value] = true
	}
	if !values[projectUseValue] || !values[filepath.Join(root, "alpha")] {
		t.Fatalf("items = %v", values)
	}
	if values[filepath.Join(root, ".hidden")] {
		t.Fatal("hidden directory listed")
	}
	if m.browseDir != root {
		t.Fatalf("browseDir = %q", m.browseDir)
	}
}

func TestSameDirectoryResolvesSymlinks(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	link := filepath.Join(dir, "link")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if !sameDirectory(real, link) {
		t.Fatal("symlinked directories differ")
	}
	if sameDirectory(real, filepath.Join(dir, "other")) {
		t.Fatal("different directories matched")
	}
}
