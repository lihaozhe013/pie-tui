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

- Header line with the current directory (`~`-collapsed), model, thinking
  level, and session name
- Streaming assistant output with incremental Markdown rendering
- A live blue activity bar above the composer names the current wait, thinking
  block, tool call, retry, or compaction, with a pulsing marker, an `Elapsed`
  clock, a `~N tokens` estimate of the block being written, and a live `t/s`
  rate
- Tool calls rendered as compact lines, with a colourised diff for `edit`/`write`
- Thinking blocks (collapsible), follow-up queueing, steering, and abort
- Model picker, thinking-level cycling, session rename/resume/new, compaction,
  HTML export, and token/context statistics
- A settings panel (`/settings` or `ctrl+p`) with the same tabs as the Pie web
  UI, sharing pi's files: `settings.json`, `models.json`, `AGENTS.md`, and the
  `pie-*` companion files in the agent directory
- Tool presets (`/preset`): the built-in configured, plan, chat-only,
  read-only, default, and full sets, plus custom presets stored in
  `pie-tool-presets.json` and editable from Settings › Tools
- A Models tab that merges pi's catalog with `models.json`: context-window
  limit, startup defaults, enabled-model patterns, per-model thinking levels,
  provider credentials, and custom provider and model editors (including
  compatibility endpoints, costs, and thinking maps)
- Pie's 18 color themes, applied instantly and saved to the terminal-only
  preferences file
- A bundled bridge extension (loaded with `-e`) that adds hot tool switching,
  Plan mode, and reload over RPC, using the same `pie:*` session entries as the
  web UI
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
pie-tui --cwd ~/src/project   # start in another working directory
pie-tui --debug               # protocol log in ~/.local/state/pie-tui/
```

The agent directory is `PI_CODING_AGENT_DIR` when set, otherwise
`~/.pi/agent`; the client and pi always read the same one.

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
| `ctrl+p` | open settings |
| `shift+tab` | cycle the thinking level |
| `ctrl+t` | show/hide thinking blocks |
| `ctrl+o` | expand or collapse tool output and diffs |
| `ctrl+l` | clear the scrollback |
| `tab` | accept a slash-command completion |
| `up` / `down` | prompt history |
| `pgup` / `pgdn`, `ctrl+u` | scroll the transcript |

## Slash commands

Each client-side command also answers to the aliases listed below, so muscle
memory from other CLIs keeps working.

| Command | Aliases | Description |
|---|---|---|
| `/settings` | `/config`, `/prefs` | open the settings panel |
| `/preset` | `/tools` | change the tool preset (`/preset plan`, `/preset configured`, or a custom name) |
| `/model` | `/models` | switch model |
| `/think` | | cycle the thinking level |
| `/new` | | start a new session |
| `/resume` | `/continue`, `/sessions` | resume another session |
| `/compact` | `/compress` | compact the conversation |
| `/name` | `/rename`, `/title` | rename this session |
| `/export` | | export the session as HTML |
| `/stats` | `/status`, `/usage` | show token and context usage |
| `/clear` | `/cls` | clear the scrollback |
| `/help` | `/keys` | show key bindings and commands |
| `/quit` | `/exit`, `/q` | exit pie-tui |

Anything else starting with `/` is forwarded to pi unchanged, so extension
commands, prompt templates, and skills (`/skill:name`) work as they do in the
native TUI. Aliases never take precedence over pi: if pi advertises a command
with the same name, that name is forwarded instead of being treated as an alias.

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
