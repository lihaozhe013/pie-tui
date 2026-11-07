package ui

import (
	"strings"
	"testing"

	"github.com/lihaozhe/pie-tui/internal/rpc"
)

func TestTitleCommandNeedsATranscript(t *testing.T) {
	model, client := newTestModel(t)
	client.respond("get_messages", func(rpc.Command) *rpc.Response {
		return &rpc.Response{Command: "get_messages", Success: true, Data: []byte(`{"messages":[]}`)}
	})

	msg, ok := model.generateTitle()().(titleMsg)
	if !ok {
		t.Fatal("unexpected message type")
	}
	if msg.err == nil || !strings.Contains(msg.err.Error(), "nothing to title") {
		t.Fatalf("err = %v", msg.err)
	}
}

func TestTitleIsItsOwnCommand(t *testing.T) {
	target, ok := builtinIndex["/title"]
	if !ok || target.name != "/title" || target.alias {
		t.Fatalf("/title = %+v, %v", target, ok)
	}
	if target, ok := builtinIndex["/name"]; !ok || target.name != "/name" {
		t.Fatalf("/name = %+v, %v", target, ok)
	}
}
