package session

import "testing"

func TestReadToolSelection(t *testing.T) {
	t.Parallel()

	entries := []LogEntry{
		{Type: "custom", CustomType: ToolSelectionEntryType, Data: []byte(`{"version":1,"tools":["read","grep"],"extensionTools":"all"}`)},
		{Type: "custom", CustomType: ToolSelectionEntryType, Data: []byte(`{"version":1,"tools":["bash","read"],"extensionTools":"selected"}`)},
	}
	selection, ok := ReadToolSelection(entries)
	if !ok {
		t.Fatal("expected a selection")
	}
	if selection.ExtensionTools != "selected" || len(selection.Tools) != 2 || selection.Tools[0] != "bash" {
		t.Fatalf("selection = %+v", selection)
	}

	cleared := append(entries, LogEntry{Type: "custom", CustomType: ToolSelectionEntryType, Data: []byte(`{"version":1,"cleared":true}`)})
	if _, ok := ReadToolSelection(cleared); ok {
		t.Fatal("cleared entry should retract the pin")
	}

	invalid := []LogEntry{{Type: "custom", CustomType: ToolSelectionEntryType, Data: []byte(`{"version":2,"tools":["read"]}`)}}
	if _, ok := ReadToolSelection(invalid); ok {
		t.Fatal("invalid version accepted")
	}
}

func TestReadWorkMode(t *testing.T) {
	t.Parallel()

	entries := []LogEntry{
		{Type: "custom", CustomType: WorkModeEntryType, Data: []byte(`{"version":1,"mode":"plan"}`)},
	}
	if mode := ReadWorkMode(entries); mode != "plan" {
		t.Fatalf("mode = %q", mode)
	}
	entries = append(entries, LogEntry{Type: "custom", CustomType: WorkModeEntryType, Data: []byte(`{"version":1,"mode":null}`)})
	if mode := ReadWorkMode(entries); mode != "" {
		t.Fatalf("cleared mode = %q", mode)
	}
}
