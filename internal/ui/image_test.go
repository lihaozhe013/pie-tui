package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lihaozhe/pie-tui/internal/rpc"
)

func TestSendImageAttachesBase64(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", t.TempDir())
	path := filepath.Join(dir, "shot.png")
	// Minimal PNG header is enough for content sniffing.
	data := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	m := New(newFakeClient(), Options{CWD: dir})
	m.width, m.height = 100, 30
	m.ready = true
	m.layout()

	cmd := m.sendImage(path + " what is this")
	if cmd == nil {
		t.Fatal("no command")
	}
	msg := cmd().(responseMsg)
	if msg.err != nil {
		t.Fatalf("send: %v", msg.err)
	}
	client := m.client.(*fakeClient)
	command, ok := client.lastCommand("prompt")
	if !ok {
		t.Fatal("prompt not sent")
	}
	if command["message"] != "what is this" {
		t.Fatalf("message = %v", command["message"])
	}
	images, ok := command["images"].([]rpc.Image)
	if !ok || len(images) != 1 || images[0].MimeType != "image/png" || images[0].Data == "" {
		t.Fatalf("images = %#v", command["images"])
	}
	if len(m.transcript.Blocks()) == 0 {
		t.Fatal("user block missing")
	}
}

func TestSendImageRejectsMissingAndBusy(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", t.TempDir())
	m := New(newFakeClient(), Options{CWD: t.TempDir()})
	if cmd := m.sendImage("missing.png"); cmd != nil {
		t.Fatal("missing image accepted")
	}
	path := filepath.Join(t.TempDir(), "x.png")
	if err := os.WriteFile(path, []byte("\x89PNG\r\n\x1a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.busy = true
	if cmd := m.sendImage(path); cmd != nil {
		t.Fatal("image accepted while busy")
	}
}
