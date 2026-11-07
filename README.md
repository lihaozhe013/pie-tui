# pie-tui

A thin terminal client for the [`pi`](https://github.com/earendil-works/pi) coding
agent, written in Go with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

`pie-tui` is a host shell only. It contains no TypeScript or Node.js SDK: it starts
the globally installed `pi` binary in RPC mode and talks to it over JSONL on
stdin/stdout. The agent owns the conversation, tools, sessions, and providers; the
client owns pixels and keys.

```
pie-tui (Go, Bubble Tea)  ⇄  stdin/stdout JSONL  ⇄  pi --mode rpc (host binary)
```

## Features

- Streaming assistant output with incremental Markdown rendering
- Tool calls rendered as compact lines, with a colourised diff for `edit`/`write`
- Thinking blocks (collapsible), follow-up queueing, steering, and abort
- Model picker, thinking-level cycling, session rename/resume/new, compaction,
  HTML export, and token/context statistics
- Extension UI support: confirm/select/input/editor dialogs, notifications,
  status entries, window titles, and editor prefills
- Survives upgrades: unknown protocol records are ignored rather than fatal

## Requirements

- `pi` 1.5.0 or newer on `PATH` (or `--pi /path/to/pi`, or `PI_TUI_PI`)
- Go 1.24 or newer to build

## Install

```bash
git clone <this repo> && cd pie-tui
make build
./bin/pie-tui
```

Install on `PATH`:

```bash
go install ./cmd/pi-tui
```

## Usage

```bash
pie-tui                       # resume is offered from the picker
pie-tui --model anthropic/claude-sonnet-4
pie-tui --no-session          # ephemeral, nothing is written to disk
pie-tui --session ~/.pi/agent/sessions/--Users-me-proj--/2024_x.jsonl
pie-tui --debug               # protocol log in ~/.local/state/pie-tui/
```

Flags are forwarded to pi where they overlap (`--provider`, `--model`, `--name`,
`--session`, `--session-dir`, `--no-session`).

## Key bindings

| Key | Action |
|---|---|
| `enter` | send the prompt (queues a follow-up while a turn is running) |
| `alt+enter`, `ctrl+j` | insert a newline |
| `ctrl+s` | steer the running turn |
| `esc` | interrupt the running turn (queues are restored to the editor) |
| `ctrl+c` | interrupt; press twice to quit |
| `ctrl+d` | quit when the editor is empty |
| `ctrl+k` | pick a model |
| `shift+tab` | cycle the thinking level |
| `ctrl+t` | show/hide thinking blocks |
| `ctrl+o` | expand or collapse tool output and diffs |
| `ctrl+l` | clear the scrollback |
| `tab` | accept a slash-command completion |
| `up` / `down` | prompt history |
| `pgup` / `pgdn`, `ctrl+u` | scroll the transcript |

## Slash commands

Client-side: `/model`, `/think`, `/new`, `/resume`, `/compact`, `/name`, `/export`,
`/stats`, `/clear`, `/help`, `/quit`.

Anything else starting with `/` is forwarded to pi unchanged, so extension
commands, prompt templates, and skills (`/skill:name`) work as they do in the
native TUI.

## Design notes

See [docs/architecture.md](docs/architecture.md) for the process boundary, the
protocol invariants (LF-only framing, permissive parsing, `agent_settled` as the
end of a run), the rendering budget, and the shutdown escalation.

## Development

```bash
make test                     # unit tests, includes a compiled fake pi
make vet
go test -race ./...
PI_TUI_E2E_BIN=$(command -v pi) go test ./internal/rpc -run TestRealPi -v
```

## License

MIT
