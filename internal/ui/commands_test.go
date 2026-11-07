package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lihaozhe/pie-tui/internal/rpc"
)

func TestBuiltinAliasesResolveToCanonical(t *testing.T) {
	t.Parallel()

	if len(builtinCommandTable) == 0 {
		t.Fatal("empty command table")
	}
	for _, command := range builtinCommandTable {
		if !strings.HasPrefix(command.Name, "/") {
			t.Fatalf("%q must start with a slash", command.Name)
		}
		if command.Detail == "" {
			t.Fatalf("%q needs a description", command.Name)
		}
		target, ok := builtinIndex[command.Name]
		if !ok || target.name != command.Name || target.alias {
			t.Fatalf("canonical %q resolved to %+v (ok=%v)", command.Name, target, ok)
		}
		for _, alias := range command.Aliases {
			target, ok := builtinIndex[alias]
			if !ok {
				t.Fatalf("alias %q is not indexed", alias)
			}
			if target.name != command.Name || !target.alias {
				t.Fatalf("alias %q resolved to %+v", alias, target)
			}
		}
	}
}

func TestBuiltinNamesAreUnique(t *testing.T) {
	t.Parallel()

	owner := make(map[string]string, len(builtinCommandTable)*2)
	for _, command := range builtinCommandTable {
		names := append([]string{command.Name}, command.Aliases...)
		for _, name := range names {
			if other, taken := owner[name]; taken {
				t.Fatalf("%q is claimed by both %q and %q", name, other, command.Name)
			}
			owner[name] = command.Name
		}
	}
}

func TestBuiltinPopupListsEveryAlias(t *testing.T) {
	t.Parallel()

	listed := make(map[string]completionItem)
	for _, item := range builtinCommands() {
		listed[item.Value] = item
	}
	for _, command := range builtinCommandTable {
		if item, ok := listed[command.Name]; !ok || item.Alias != "" {
			t.Fatalf("canonical %q missing from the popup: %+v", command.Name, item)
		}
		for _, alias := range command.Aliases {
			item, ok := listed[alias]
			if !ok || item.Alias != command.Name {
				t.Fatalf("alias %q missing from the popup: %+v", alias, item)
			}
		}
	}
}

func TestModelSlashAliasesDispatchLocally(t *testing.T) {
	t.Parallel()

	aliases := []string{"/models", "/keys", "/cls", "/usage", "/q"}
	for _, alias := range aliases {
		model, client := newTestModel(t)
		model.composer.SetText(alias)

		model, cmd := step(t, model, keyMsg("enter"))
		runCmd(t, cmd)

		if client.countCommand("prompt") != 0 {
			t.Fatalf("%s must not be forwarded to pi", alias)
		}
	}
}

func TestModelSlashAliasQuitReturnsTeaQuit(t *testing.T) {
	t.Parallel()

	model, _ := newTestModel(t)
	model.composer.SetText("/exit")

	_, cmd := step(t, model, keyMsg("enter"))
	if cmd == nil {
		t.Fatal("/exit must quit")
	}
	if msg := cmd(); msg == nil {
		t.Fatal("/exit produced no message")
	} else if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("/exit produced %T", msg)
	}
}

func TestModelSlashAliasYieldsToPiCommand(t *testing.T) {
	t.Parallel()

	model, client := newTestModel(t)
	model.commands = append(model.commands, rpc.CommandInfo{Name: "continue", Description: "my template", Source: "prompt"})
	model.completion.SetItems(model.commandItems())

	model.composer.SetText("/continue now")
	model, cmd := step(t, model, keyMsg("enter"))
	runCmd(t, cmd)

	if _, ok := client.lastCommand("prompt"); !ok {
		t.Fatal("/continue must be forwarded when pi owns the name")
	}
	if model.overlay != nil {
		t.Fatal("the resume picker must not open for a pi command")
	}
	for _, item := range model.completion.items {
		if item.Value == "/continue" && item.Alias == "/resume" {
			t.Fatal("pi-owned name must not be advertised as a built-in alias")
		}
	}
}

func TestModelSlashCanonicalWinsOverPiCommand(t *testing.T) {
	t.Parallel()

	model, client := newTestModel(t)
	model.commands = append(model.commands, rpc.CommandInfo{Name: "help", Source: "extension"})
	model.composer.SetText("/help")

	model, cmd := step(t, model, keyMsg("enter"))
	runCmd(t, cmd)

	if client.countCommand("prompt") != 0 {
		t.Fatal("canonical built-ins stay client-side even when pi uses the name")
	}
	if model.overlay == nil {
		t.Fatal("help overlay did not open")
	}
}

func TestModelUnknownSlashIsForwarded(t *testing.T) {
	t.Parallel()

	model, client := newTestModel(t)
	model.composer.SetText("/fix-tests please")

	model, cmd := step(t, model, keyMsg("enter"))
	runCmd(t, cmd)

	if _, ok := client.lastCommand("prompt"); !ok {
		t.Fatal("unknown commands must be forwarded to pi")
	}
}

func TestCompletionRanksCanonicalBeforeAlias(t *testing.T) {
	t.Parallel()

	list := &completion{}
	list.SetItems(builtinCommands())

	list.Refresh("/")
	first, _ := list.Selected()
	if first.Alias != "" {
		t.Fatalf("popup opened on alias %q", first.Value)
	}

	list.Refresh("/q")
	first, _ = list.Selected()
	if first.Value != "/q" {
		t.Fatalf("exact match must win, got %q", first.Value)
	}
}
