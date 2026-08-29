# `jumux pr` / `jumux mr` design

## Context

Closes #15 and #16. Once a feature's work is ready, pushing the jj
change and opening a PR (GitHub) or MR (GitLab) is currently a manual,
multi-step process outside jumux: find the workspace, set/push a
bookmark, switch to a browser or another terminal, and run `gh pr
create` / `glab mr create` by hand. Both issues ask for a jumux
subcommand that does this from within the feature's window/workspace.
The two commands are otherwise identical except for which host CLI
they shell out to, so they're designed and implemented together to
share one core.

## Goals

- `jumux pr [feature]` pushes the feature's jj bookmark and opens a
  GitHub PR via `gh pr create`.
- `jumux mr [feature]` does the same for GitLab via `glab mr create`.
- Title/body are prefilled from the jj change description where
  possible.
- Re-running either command after a PR/MR already exists is a no-op
  success (just pushes updates), not an error.
- `jumux doctor` gains non-fatal checks for `gh`/`glab` presence.

## Non-goals

- No new config fields for remote host type/detection — `gh`/`glab`
  are invoked directly and rely on their own remote/auth detection.
- No support for updating an existing PR/MR's title/body on re-run —
  only the initial creation is prefilled; re-runs just push.
- No interactive prompting for title/body edits; whatever the jj
  description yields is used as-is.

## Design

### `internal/jj` additions

New wrapper functions in `internal/jj/jj.go`, following the existing
`func Name(r run.Runner, dir string, ...) (string, error)` pattern
(each simply shells out via `r.Run`):

- `BookmarkSet(r run.Runner, dir, name, rev string) error` →
  `jj bookmark set <name> -r <rev>`
- `GitPush(r run.Runner, dir, bookmark string) error` →
  `jj git push --bookmark <bookmark>`
- `Description(r run.Runner, dir, rev string) (string, error)` →
  `jj log -r <rev> --no-graph -T description`

### `internal/app`: shared feature resolution

`remove.go`'s `inferFeature` (workspace-dir-name match, falling back
to `tmuxctl.CurrentWindowFeature`) is extracted into a shared `*App`
method so `Remove`, `PR`, and `MR` all use one lookup. Behavior is
unchanged: an explicit feature name short-circuits inference; with no
name, it resolves from the current workspace directory or the tmux
`@jumux-feature` window option, erroring if neither resolves.

### `internal/forge` (new package)

Holds the push+prefill logic shared by both commands, independent of
which host CLI will run afterward:

```go
func PreparePush(r run.Runner, wsRoot, feature string) (title, body string, err error)
```

Steps: `jj.BookmarkSet(r, wsRoot, feature, feature+"@")`, then
`jj.GitPush(r, wsRoot, feature)`, then `jj.Description(r, wsRoot,
feature+"@")` split into first line (title) and remaining lines
(body). If the description is empty, title falls back to the feature
name and body is empty.

### `internal/app/pr.go` / `internal/app/mr.go`

```go
func (a *App) PR(feature string) error
func (a *App) MR(feature string) error
```

Each: resolve `repoContext`, resolve the feature via the shared
inference method, call `forge.PreparePush`, then shell out:

- PR: `a.Runner.Run(ctx.WsRoot, "gh", "pr", "create", "--title", title, "--body", body)`
- MR: `a.Runner.Run(ctx.WsRoot, "glab", "mr", "create", "--title", title, "--description", body)`

**Already-exists handling**: if the command fails and its combined
output contains a host-specific "already exists" marker (`gh`: "a pull
request for branch ... already exists"; `glab`: "already exists" in
its error text), treat it as success — print a message that the
bookmark was pushed and a PR/MR already exists, rather than returning
an error. Any other failure (missing binary, auth failure, network
error) is returned as-is.

### `main.go`

Add `case "pr":` and `case "mr":`, each with a flag set accepting zero
or one positional arg (the optional feature name), calling `a.PR(...)`
/ `a.MR(...)`. Add both to the `usage` const.

### `jumux doctor`

Add two more checks alongside the existing jj/tmux presence checks:
report whether `gh` and `glab` are found on `PATH` (e.g. via `exec
LookPath` or a trivial `--version` run through the Runner). Missing
either is reported as informational, not a doctor failure — a repo
may only ever need one of the two hosts.

### Testing

Table-driven tests using the existing `run.FakeRunner`, following
current conventions (`internal/jj/jj_test.go`, `internal/app`
fixtures):

- `internal/jj/jj_test.go`: cases for `BookmarkSet`, `GitPush`,
  `Description` (including empty-description handling).
- `internal/forge/forge_test.go`: `PreparePush` happy path and error
  propagation from each jj step.
- `internal/app/pr_test.go` / `mr_test.go`: assert exact `gh`/`glab`
  command lines via `FakeRunner.CommandLines()`, cover explicit vs.
  inferred feature, and the "already exists" success path.
- `internal/app/doctor_test.go` (or wherever doctor is tested):
  cover `gh`/`glab` present and absent.

## Verification

- `make all` (build, test, lint) passes.
- Manually, in a real jj+tmux workspace with a GitHub remote: run
  `jumux add`, make a change, `jumux pr`, confirm the bookmark is
  pushed and a PR opens with the change description as title/body;
  re-run `jumux pr` and confirm it succeeds without erroring.
- Repeat against a GitLab-hosted repo with `jumux mr` and `glab`.
- Run `jumux doctor` in an environment missing `gh`/`glab` and confirm
  it reports them as missing without failing the overall check.
