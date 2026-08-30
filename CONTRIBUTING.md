# Contributing to jumux

Thanks for considering a contribution! This project is small, so the process
is lightweight.

## Getting set up

jumux uses [mise](https://mise.jdx.dev/) to pin the Go, golangci-lint, and
goreleaser versions used to build, test, and release it. After installing
mise:

```
mise install
```

See [AGENTS.md](AGENTS.md) for a full overview of the project layout and
conventions (`internal/app`, `internal/jj`, `internal/tmuxctl`, etc.).

## Building and testing

Use the Makefile targets:

- `make build` — build the binary to `bin/jumux`
- `make test` — run all tests (`go test ./...`)
- `make lint` — run `golangci-lint run`
- `make fmt` — format code (`gofmt -l -w .`)
- `make vet` — run `go vet ./...`
- `make all` — build + test + lint

Run `make all` before opening a PR — CI runs the same checks on every push
and PR to `main`.

## Tests

- Write table-driven tests alongside the code they test.
- Use the fake command runner in `internal/run/fake.go` instead of executing
  real `jj`/`tmux` commands.

## Commit style

Use conventional-commit-style prefixes where they fit (`feat:`, `fix:`,
`docs:`, `test:`, etc.) — the release changelog filters on these. Keep
commit messages concise with an imperative subject line.

## Opening a pull request

1. Fork the repo and create a branch off `main`.
2. Make your change, keeping logic in `internal/` (`main.go` only parses
   arguments and dispatches).
3. Run `make all` and make sure it passes.
4. Open a PR against `main` describing what changed and why.

## Manual testing

jumux must be run inside tmux, in a jj repo, to exercise most commands
end-to-end — keep that in mind when testing changes to `add`, `remove`,
`sidebar`, etc.

## Code of Conduct

This project follows the [Code of Conduct](CODE_OF_CONDUCT.md). By
participating, you're expected to uphold it.
