# AGENTS.md

Guidance for coding agents working in this repository.

## Project overview

agentmux is a Go CLI for working on multiple features in parallel with coding
agents. It pairs a [jujutsu (jj)](https://github.com/jj-vcs/jj) workspace with
a tmux window per feature. It must be run inside tmux, in a jj repo.

- Module: `github.com/richardcase/agentmux`
- Commands: `add <feature>`, `remove [-f] [name]`, `list`, `sidebar`
  (dispatched in `main.go`; `sidebar run` is the internal per-pane TUI mode)
- Configuration: global `~/.config/agentmux/config.toml`, overridden per-repo
  by `.agentmux.toml` at the repo root (TOML)

## Layout

- `main.go` — entrypoint and CLI argument parsing
- `internal/app/` — command implementations (`add.go`, `remove.go`, `list.go`,
  `sidebar.go`, `status.go`)
- `internal/config/` — TOML config loading and merging
- `internal/jj/` — jujutsu wrapper
- `internal/tmuxctl/` — tmux control
- `internal/sidebar/` — bubbletea sidebar TUI (jj/tmux-agnostic; data in via closures)
- `internal/run/` — command runner abstraction; `fake.go` is a reusable test fake

## Build, test, lint

Use the Makefile targets:

- `make build` — build the binary to `bin/agentmux`
- `make test` — run all tests (`go test ./...`)
- `make lint` — run `golangci-lint run`
- `make fmt` — format code (`gofmt -l -w .`)
- `make vet` — run `go vet ./...`
- `make all` — build + test + lint

CI runs build, test, and lint on every push and PR to `main`. Run `make all`
before considering a change done.

## Conventions

- Keep all logic in `internal/`; `main.go` only parses arguments and dispatches.
- Write table-driven tests alongside the code they test, using the fake command
  runner in `internal/run/fake.go` instead of executing real `jj`/`tmux`.
- Use conventional-commit-style prefixes where they fit (e.g. `docs:`, `test:`);
  the release changelog filters on these.

## Commit and PR rules

- Do **not** add `Co-Authored-By:` trailers for AI agents to commit messages.
- Do **not** add "Generated with Claude Code" or any similar agent attribution
  to commit messages or PR descriptions.
- Keep commit messages concise, with an imperative subject line.
- Do not commit or push unless explicitly asked.
