package ui

import (
	"strings"
	"testing"

	"github.com/lihaozhe/pie-tui/internal/rpc"
	"github.com/lihaozhe/pie-tui/internal/session"
)

func TestForkFlowLoadsMessagesAndRestoresPrompt(t *testing.T) {
	client := newFakeClient()
	client.respond("fork", func(rpc.Command) *rpc.Response {
		return &rpc.Response{Command: "fork", Success: true, Data: []byte(`{"text":"do that again","cancelled":false}`)}
	})
	client.respond("get_messages", func(rpc.Command) *rpc.Response {
		return &rpc.Response{Command: "get_messages", Success: true, Data: []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"first"}]}]}`)}
	})
	m := New(client, Options{})
	m.transcript.Add(newBlock(blockUser, "old"))
	m.layout()

	msg := m.forkSession("entry-1")().(forkResultMsg)
	if msg.err != nil || msg.cancelled {
		t.Fatalf("msg = %+v", msg)
	}
	model, _ := m.Update(msg)
	m = model.(*Model)
	if m.composer.Value() != "do that again" {
		t.Fatalf("composer = %q", m.composer.Value())
	}
	found := false
	for _, block := range m.transcript.Blocks() {
		if strings.Contains(block.text+block.body, "first") {
			found = true
		}
	}
	if !found {
		t.Fatalf("forked transcript missing: %+v", m.transcript.Blocks())
	}
}

func TestForkMessagesOpenPicker(t *testing.T) {
	m := New(newFakeClient(), Options{})
	model, _ := m.Update(forkMessagesMsg{messages: []forkMessage{{EntryID: "a", Text: "first prompt"}}})
	m = model.(*Model)
	if m.overlay == nil || m.overlay.action != "fork" || len(m.overlay.items) != 1 {
		t.Fatalf("overlay = %+v", m.overlay)
	}
}

func TestTreePrintsIntoTranscript(t *testing.T) {
	m := New(newFakeClient(), Options{})
	tree := session.Tree{
		LeafID: "b",
		Roots: []session.TreeNode{{
			ID: "a", Type: "message", Label: "user: first",
			Children: []session.TreeNode{{ID: "b", Type: "message", Label: "assistant: answer"}},
		}},
	}
	model, _ := m.Update(treeResultMsg{tree: tree})
	m = model.(*Model)
	if m.tree.LeafID != "b" {
		t.Fatalf("tree = %+v", m.tree)
	}
	found := false
	for _, block := range m.transcript.Blocks() {
		if strings.Contains(block.text, "assistant: answer") && strings.Contains(block.text, "← current") {
			found = true
		}
	}
	if !found {
		t.Fatalf("tree not printed: %+v", m.transcript.Blocks())
	}
}
