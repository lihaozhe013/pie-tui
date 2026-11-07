# Project Instructions

## Testing

- Do not run the automated test suite (`go test`, `make test`) as part of
  normal work. It is not required for changes to be considered done.
- All testing is manual: build the binary (`make build`) and drive the TUI in
  a real terminal (or a `tmux` session) to exercise the change.
- When a change only needs a quick sanity check, `go build ./...` is enough.
