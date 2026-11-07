# pie-tui

A thin terminal client for [`pi`](https://github.com/earendil-works/pi) written in Go.

`pie-tui` is a host shell only. It contains no TypeScript or Node.js SDK: it starts
the globally installed `pi` binary in RPC mode and talks to it over JSONL on
stdin/stdout, so the UI process stays small and cannot drift from the agent's own
behaviour.

```
pie-tui (Go, Bubble Tea)  ⇄  stdin/stdout JSONL  ⇄  pi --mode rpc (host binary)
```

## Status

Under active construction. See [docs/architecture.md](docs/architecture.md) for the
design and the milestone list.

## Requirements

- Go 1.24 or newer (build only)
- `pi` 1.5.0 or newer available on `PATH`

## Build and run

```bash
make build
./bin/pie-tui
```

Point the client at a specific binary when `pi` is not on `PATH`:

```bash
./bin/pie-tui --pi /path/to/pi
```

## License

MIT
