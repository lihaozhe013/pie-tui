# Architecture

`pie-tui` is a host shell. It never embeds or reimplements the agent: the whole
conversation, tool execution, session storage, and provider access live in the
`pi` binary. This document describes the boundary, the modules, and the rules the
code must keep to stay a thin client.

## Process boundary

```
┌──────────────────── pie-tui (Go) ────────────────────┐      ┌──────── pi (binary) ────────┐
│ cmd/pi-tui   flags · dependency check · shutdown      │      │                             │
│                                                       │      │  agent loop · tools         │
│ internal/rpc  Client ── stdin  (JSONL commands) ──────┼─────▶│  providers · sessions       │
│                 │   ◀── stdout (JSONL events) ────────┼──────│  extensions                 │
│                 │   ◀── stderr (diagnostics) ─────────┼──────│                             │
│                 ▼                                      │      │                             │
│ internal/ui   Model (Bubble Tea) ── renders ──▶ tty    │      └─────────────────────────────┘
└───────────────────────────────────────────────────────┘
```

Rules that keep the client thin:

- No TypeScript or Node.js dependency. `go.mod` only lists Go modules.
- `pi` is discovered with `exec.LookPath` (`--pi` or `PI_TUI_PI` override).
- stdout is protocol only. Anything the UI needs to log goes to a file (`--debug`)
  or to the child's stderr tail kept for crash reports.
- The client keeps a *projection* of the session, never the source of truth. Every
  authoritative value comes from a response or a `message_end`/`*_end` event.

## Protocol rules

These are the invariants that make the client survive a pi upgrade.

| Rule | Where |
|---|---|
| Split records on LF only; `U+2028`/`U+2029` are legal inside strings | `rpc.readLine` |
| Never use `bufio.Scanner`: it caps a record at 64 KiB | `rpc.readLine` |
| Records above 32 MiB are dropped and the stream resynchronises | `rpc.readLine` |
| Unknown record types degrade to `UnknownEvent`, never to an error | `rpc.ParseLine` |
| Known records with unexpected payloads also degrade to `UnknownEvent` | `rpc.decodeEvent` |
| Responses are correlated by `id`; order is irrelevant | `Client.handleLine` |
| `prompt` success only means "accepted"; `data.disposition` explains how | `ui.applyResponse` |
| A run is over at `agent_settled`, not at `agent_end` | `ui.handleEvent` |
| `*_delta` is for live display; `text_end`/`message_end` are authoritative | `rpc.StreamAccumulator` |
| stderr is diagnostics, never protocol | `Client.readLoop` |
| Closing stdin requests shutdown; SIGTERM and SIGKILL are the escalation | `Client.Close` |

## Modules

### `internal/rpc`

- `events.go` — inbound record types, the permissive parser, and the streaming
  accumulator used to rebuild an assistant message from deltas.
- `client.go` — child process, pipes, correlation, backpressure, shutdown. One
  goroutine reads stdout and owns `cmd.Wait`; commands are written under a mutex;
  the event channel is bounded so the UI always sees backpressure rather than
  losing records.
- `commands.go` — typed builders for every documented command plus the response
  payload shapes the UI consumes.

The reader never renders. A slow consumer blocks the channel, which applies
backpressure to pi through the pipe instead of growing memory.

### `internal/session`

Read-only scanning of pi's session directory (`~/.pi/agent/sessions` by default,
`--session-dir`/`PI_CODING_AGENT_SESSION_DIR` override). It reads the header of
each `.jsonl` file for the ID and working directory, and probes the tail for the
newest `session_info` name. Broken files are skipped so one bad file cannot break
the resume picker.

### `internal/ui`

- `model.go` — the Bubble Tea model: state, message dispatch, layout, and the
  frame composition. The frame is exactly `height` lines; any mismatch would
  scroll the alternate screen and hide the header.
- `update.go` — protocol events to UI state, key routing, slash commands, and the
  command helpers that call pi.
- `commands.go` — the client-side command table: canonical names, aliases, and
  the alias-to-command index shared by dispatch and completion.
- `activity.go` — the live run status above the composer: phase, running tools,
  retry and compaction labels, the elapsed clock, the token estimate and t/s
  rate of the block being written, and the pulsing marker.
- `viewport.go` — transcript blocks, scrolling, and incremental markdown.
- `input.go` — multi-line composer, prompt history, and slash completion.
- `overlay.go` — pickers and extension dialogs.
- `diff.go` — tool argument/result extraction and a dependency-free line diff.
- `styles.go` — the Lip Gloss palette and the glamour style derived from it.
- `view.go` — small formatting helpers.

### Rendering budget

Streaming assistant text is split at blank lines outside fenced code blocks.
Everything before the tail is a *stable segment*: it is rendered once per width
and cached on the block. Only the tail is re-rendered, and only on a 40 ms tick,
so a fast token stream cannot saturate the CPU. An unterminated fence gets a
synthetic closing marker so code does not flicker between paragraph and block
styling.

## Configuration and the bridge

`pie-tui` and Pie share every file in pi's agent directory
(`PI_CODING_AGENT_DIR`, default `~/.pi/agent`):

- `settings.json`, `models.json`, `auth.json`, `mcp.json`, and `AGENTS.md` are
  pi's own files.
- `pie-model-settings.json`, `pie-tool-presets.json`, and the session log's
  `pie:tool-selection` / `pie:work-mode` entries are Pie's files; the TUI reads
  and writes the same shapes.
- `pie-tui-settings.json` holds terminal-only choices: theme, thinking-block
  visibility, tool expansion, and the last working directory.

`internal/config` owns this layer. Documents keep object member order, JSONC
comments and trailing commas are accepted, writes are private (0600), atomic,
and serialized with a lock file beside the target, and a file that cannot be
parsed is backed up (`.corrupt-<timestamp>`) instead of being silently replaced.

`pi --mode rpc` does not expose tool switching, reload, or a system-prompt
read, so the binary embeds a small extension (`internal/bridge/pie-bridge.ts`),
writes it to `<state-dir>/bridge/`, and passes it with `-e`. The extension
registers `/pie-tools`, `/pie-mode`, `/pie-reload`, and `/pie-publish`; the UI
drives them with ordinary `prompt` commands. Tool selections and work modes are
written as the same session entries Pie uses, so a session moved between the
clients keeps its preset and its Plan mode. If the extension fails to load, the
UI reports `unavailable` in Settings › General and refuses preset changes
rather than silently diverging.

The working directory is the child process's `Dir`, fixed for the life of one
pi process. `/cwd` therefore restarts the child: it closes the old stdin,
starts a fresh pi with the same binary, flags, and bridge extension but no
`--session`, and re-runs the handshake. The new session starts in the chosen
folder; the old transcript is not carried over. A stale `closed` event from the
replaced child is ignored by comparing the client identity. Recent projects are
derived from the `cwd` in session headers rather than a second index, and
`pie-tui-settings.json` keeps the last choice only to seed the browser; the
process's own directory stays the default when no `--cwd` is given.

MCP servers are read from the agent's `mcp.json` and the project's
`.pi/mcp.json` with the same `mcpServers` shape pi uses. The panel edits only
three fields through `internal/config`: `enabled`, `exposure`, and removal; an
add goes through `pi mcp add` so the CLI validates transports, headers, and
OAuth settings exactly as it does for the web panel. Connection state comes from
`pi mcp list --json`, run on demand because it starts every enabled server.
Project files keep pi's trust rule at run time: the terminal writes them only
for the folder the person opened.

Plugins are the `packages` array in `settings.json` and in the project's
`.pi/settings.json`. A string entry is enabled; an object whose four resource
lists are all empty is disabled. Disabling a filtered entry is refused because
the write would drop the filters, exactly as the web panel refuses it. Every
mutation goes through `internal/config` under the lock, so an entry keeps its
position, unknown keys (`autoload`, say) and the rest of the file. Installing,
removing, and updating shell out to `pi install/remove/update`, which writes the
same settings entry pi itself reads.

Skills are read with pi's own discovery rules: a `SKILL.md` directory is one
skill and is not recursed into, standalone `.md` children are skills, and
symlinks are followed. The roots are the agent directory's `skills/`, the
project's `.pi/skills/`, and the `skills` list in `settings.json`.
`disable-model-invocation` is edited line by line inside the frontmatter so the
rest of the file keeps its formatting, and the write follows symlinks (a global
skill installed by `npx skills` is a link into `~/.agents/skills`). Installing
and updating shell out to `npx skills`, matching the web panel's CLI, rather
than writing skill directories in Go.

Goals are the goal package's, not the client's. `internal/session` reads the
newest `custom` entry whose `customType` is `pi-better-goal` or
`pi-codex-goal`; `set` supplies the status, objective, and usage, and `clear`
retracts it. The `/goal` command opens a panel that only sends `/goal …`
prompts to the installed package, which owns continuations and completion. The
package is never bundled: availability is the `goal` command reported by
`get_commands`, and the terminal-only `goalEnabled` preference refuses to turn
on without it. `pie-tui` writes no goal state of its own, so a goal started in
Pie appears in the header here and the reverse.

Settings › Models merges three sources without a second cache:
`models-store.json` (pi's catalog), `models.json` (custom providers, models,
and model overrides), and `get_available_models` (what the running agent
actually exposes). `internal/modelcatalog` owns the merge and the minimal-edit
rules for `enabledModels`: turning one model off expands only the pattern that
covers it, preserves `:thinking` pins, and never touches patterns that match
nothing. Writes go through `internal/config`, so a provider or model added in
the terminal appears in Pie and the reverse. The context-window cap in
`pie-model-settings.json` is shared, but only Pie's server applies it to a
runtime; the terminal client cannot re-clamp pi's in-process model list.

## Startup and shutdown

1. Resolve `pi` (flag, environment, `PATH`). Missing binaries print install
   instructions and exit with code 2.
2. Start `pi --mode rpc` with the pass-through flags.
3. Handshake: `get_state`, `get_available_models`,
   `get_available_thinking_levels`, `get_commands`, `get_session_stats`, and
   `get_messages` when the session already has history.
4. Render the TUI on the alternate screen and pump events.
5. On exit: close stdin (orderly shutdown), wait 3 s, SIGTERM the process group,
   wait 2 s, then SIGKILL. The child is put in its own process group so a
   `SIGTERM` reaches subprocesses too.

If the child dies, the UI keeps the last stderr lines, shows them in an error
block, and refuses to send more commands.

## Testing strategy

- `internal/rpc/client_test.go` builds `testdata/fakepi` as a real child process
  and covers out-of-order responses, malformed lines, CRLF, 1 MiB records,
  oversized records, child crashes, request timeouts, slow consumers, graceful
  close, and SIGTERM escalation. The test binary itself is not used as the fake:
  modern Go testing captures `os.Stdout`, which delays protocol writes.
- `internal/rpc/e2e_test.go` runs against a real pi when `PI_TUI_E2E_BIN` is set.
  `PI_TUI_E2E_PROMPT=1` adds one model round trip.
- `internal/ui/*_test.go` drives the model with a fake client, so key handling,
  streaming, dialogs, and rendering are tested without a terminal.

```bash
make test
go test -race ./...
PI_TUI_E2E_BIN=$(command -v pi) go test ./internal/rpc -run TestRealPi -v
```

## Known limitations

- No image or file attachments yet; prompts are text only.
- Extension `setWidget` output is folded into the status line instead of a
  dedicated panel.
- The diff view is prefix/suffix based. It is exact for edit tools' replacement
  snippets, but a full Myers diff is not implemented.
- Built-in TUI-only pi commands (`/hotkeys`, ...) do not exist in RPC mode and
  are not offered; the client implements its own `/settings` panel.
