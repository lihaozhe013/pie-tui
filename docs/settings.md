# Settings reference

The panel opens with `/settings` or `ctrl+p`. Every tab reads the same files as
the Pie web UI, so a change made in either client is visible to the other after
a reload. Press `/` inside the panel to search every tab and jump to a row.

Shared files are written immediately, with a lock and an atomic replace. The
TUI never drops JSON keys it does not understand.

## General

| Row | File | Notes |
| --- | --- | --- |
| Cache-miss notices | `settings.json` (`showCacheMissNotices`) | pi reads it on start or reload. |
| Session directory | `settings.json` (`sessionDir`) | Blank uses pi's default under the agent directory. |
| Agent directory | read-only | `PI_CODING_AGENT_DIR` or `~/.pi/agent`. |
| Settings file | read-only | The agent `settings.json` in use. |
| Bridge extension | read-only | `ready` when pi advertises `pie-tools`. |
| Thinking blocks | `pie-tui-settings.json` | TUI-only visibility default. |
| Goal controls | `pie-tui-settings.json` | Enabling requires the `pi-better-goal` package. |
| Expand tool output | `pie-tui-settings.json` | Same as `ctrl+o`. |
| Reload pi | session prompt `/pie-reload` | Rereads settings and instructions; needs the bridge. |

## Instructions

Edits the global `AGENTS.md` in the agent directory with a multi-line editor.
`ctrl+s` saves. With the bridge, the save sends `/pie-reload` so pi rereads the
file without a restart; otherwise the change applies to the next pi run.

## Appearance

Themes reproduce Pie's palette names and apply instantly. "Enter sends" picks
between Enter and alt+enter as the submit key; the other becomes newline.
Fonts, sizes, content width, and browser theme tiles have no terminal
equivalent.

## Models

Four sections share `models.json`, `settings.json`, `auth.json`,
`models-store.json`, and `pie-model-settings.json`:

- General: context-window cap (blank stores an explicit no-cap), default
  provider, model, and thinking level.
- Enabled models: patterns and per-model switches; edits expand only the
  pattern that covers the changed model and keep `:level` pins and unmatched
  patterns intact.
- Per-model thinking levels: `modelThinkingLevels` for the session model picker.
- Per-model context windows: writes `providers.<id>.modelOverrides.<model>.contextWindow`
  in `models.json`, keeping other override keys and removing empty objects.
- Providers: credential state, `pi auth check` readiness, API keys, and custom
  providers and models with compatibility endpoints and costs.

The cap also feeds the context meter in the header and `/stats`: a model with a
larger native window is shown as the cap size, as in Pie.

## Tools

- Current applies `defaultTools` through the bridge. Presets are the built-in
  configured, plan, chat-only, read-only, default, and full sets plus custom
  entries from `pie-tool-presets.json`.
- Code mode: automatic or always writes `+codemode` into `defaultTools` with the
  smallest edit; code-mode tools choose `on` or `only`; the inline result budget
  edits `codemode.inlineBudget`.
- PowerShell tool (Windows only) swaps `powershell` for `bash` in the resolved
  tool list.

## Skills

Discovers agent, project, settings, `~/.agents/skills`, and installed-package
skills. The detail panel toggles `disable-model-invocation` in the frontmatter,
opens the file in `$VISUAL`/`$EDITOR`, and shows that changes apply on the next
pi start or reload. Search and install use the same `npx skills` commands as
Pie.

## Plugins

Lists packages from the global `settings.json` and the project's
`.pi/settings.json`. Install, remove, and update shell out to `pi install`,
`pi remove`, and `pi update`. Disabling writes all four resource lists empty and
refuses packages that carry their own filters; enabling removes only those
empty lists, preserving `autoload` and unknown keys. Reload from the detail
panel or General to apply.

## MCP

Reads and writes `mcp.json` and a trusted project `.pi/mcp.json`. Servers show
transport, exposure, enabled state, and env/header names with values masked.
Connection checks use `pi mcp list --json`; add, login, logout, and remove use
the matching `pi mcp` commands so validation stays identical to Pie. Project
servers are read-only without an explicit trust decision for the folder, and a
project package can only be changed once its resources are trusted; the TUI
reads the nearest decision from `trust.json`, inherited from a parent folder.

## Goal

A separate panel opened by `/goal` (or Settings › General). It reports the
installed goal package's status, objective, and token usage and sends
`/goal start|pause|resume|complete|clear`. The package must be installed
manually with `pi install npm:pi-better-goal`; the TUI never bundles or installs
it.

## Terminal-only preferences

`pie-tui-settings.json` holds choices a browser does not have: `theme`,
`enterToSend`, `goalEnabled`, `lastCwd`, `toolPreset`, `showThinking`, and
`expandTools`. Everything shared stays in pi's files.
