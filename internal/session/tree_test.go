package session

import (
	"encoding/json"
	"testing"
)

func TestParseTreeLabelsEntries(t *testing.T) {
	raw := json.RawMessage(`{
	  "leafId": "b",
	  "tree": [{
	    "entry": {"id":"a","parentId":null,"type":"message","message":{"role":"user","content":[{"type":"text","text":"First prompt"}]}},
	    "children": [{
	      "entry": {"id":"b","parentId":"a","type":"message","message":{"role":"assistant","content":[{"type":"text","text":"The answer"}]}},
	      "children": []
	    }, {
	      "entry": {"id":"c","parentId":"a","type":"custom","customType":"pie:tool-selection"},
	      "children": []
	    }]
	  }, {
	    "entry": {"id":"d","parentId":null,"type":"session_info","name":"Named"},
	    "children": []
	  }]
	}`)
	tree, err := ParseTree(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if tree.LeafID != "b" || len(tree.Roots) != 2 {
		t.Fatalf("tree = %+v", tree)
	}
	if tree.Roots[0].Label != "user: First prompt" {
		t.Fatalf("label = %q", tree.Roots[0].Label)
	}
	if len(tree.Roots[0].Children) != 2 || tree.Roots[0].Children[0].Label != "assistant: The answer" {
		t.Fatalf("children = %+v", tree.Roots[0].Children)
	}
	if tree.Roots[0].Children[1].Label != "pie:tool-selection" {
		t.Fatalf("custom label = %q", tree.Roots[0].Children[1].Label)
	}
	if tree.Roots[1].Label != "name: Named" {
		t.Fatalf("name label = %q", tree.Roots[1].Label)
	}
}
