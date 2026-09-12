# jumux

[![CI](https://github.com/richardcase/jumux/actions/workflows/ci.yml/badge.svg)](https://github.com/richardcase/jumux/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/richardcase/jumux)](https://goreportcard.com/report/github.com/richardcase/jumux)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

Work on multiple features in parallel with coding agents, pairing a
[jujutsu](https://github.com/jj-vcs/jj) workspace with a tmux window per
feature.

```
jumux add <feature>       create a jj workspace + tmux window, start the agent
                          (-a/--agent overrides the agent; -t/--template applies a preset)
jumux remove [-f] [name]  tear a feature down (defaults to the current one)
jumux merge [-f] [name]   merge a feature into the base bookmark locally, then tear it down
jumux resurrect           recreate tmux windows lost to a tmux crash/restart
jumux sync [name]         re-apply configured files.copy/files.symlink to a workspace
jumux rebase [feature]    rebase a feature's workspace onto its base revision
                          (--onto REV rebases onto REV instead)
jumux list                show feature workspaces and their tmux windows
jumux pr [feature]        push the feature's bookmark and open a GitHub PR
jumux mr [feature]        push the feature's bookmark and open a GitLab MR
jumux sidebar             toggle a live agent sidebar pane on every tmux window
jumux hook <status>       record agent status (working|waiting|done) from hooks
jumux doctor              check the environment and report problems to fix
jumux config show         print the effective config and where each value came from
```

## Quickstart

1. **Prerequisites**: tmux, and a [jujutsu](https://github.com/jj-vcs/jj) repo
   (ideally colocated with git — `jj git init --colocate` in an existing git
   repo, or `jj git init` for a new one).
2. **Install jumux**:
   ```
   brew install richardcase/tap/jumux
   ```
   or, with Go:
   ```
   go install github.com/richardcase/jumux@latest
   ```
3. **Start tmux and run jumux inside the repo**:
   ```
   jumux add my-feature
   ```
   This creates a jj workspace, opens a tmux window for it, and starts the
   agent (see [What `add` does](#what-add-does)). The first run offers to
   install [Claude Code and Codex hooks](#agent-status-icons) for agent
   status tracking.
4. **Check on your features** from any window:
   ```
   jumux list
   jumux sidebar
   ```
5. **Ship the work** once the agent is done:
   ```
   jumux pr my-feature   # or: jumux mr my-feature
   ```
6. **Clean up** the workspace and tmux window:
   ```
   jumux remove my-feature
   ```

## What `add` does

Run inside tmux, anywhere in a jj repo (ideally colocated with git):

1. Creates a jj workspace named `<feature>` under
   `$XDG_DATA_HOME/jumux/workspaces/<repo>/<feature>` (falling back to
   `~/.local/share/jumux/workspaces/<repo>/<feature>`), based on `trunk()`.
2. Opens a tmux window named `<feature>` in that directory. The window's
   name is pinned (`automatic-rename off`) and tagged with the custom option
   `@jumux-feature` so jumux can find it again even if it gets renamed.
3. Types the configured agent command into the window and presses Enter, then
   switches to it. If the agent exits, the window's shell survives for
   inspection.

## What `remove` does

1. Resolves the feature: the argument, else the workspace your cwd is in,
   else the current window's `@jumux-feature` tag. `default` is refused.
2. If the workspace's working-copy commit has changes, asks for confirmation
   (`-f`/`-force` skips). Note that `jj workspace forget` never deletes
   commits — described work stays in the repo either way.
3. Forgets the jj workspace, deletes the directory, and kills the tmux
   window — in that order, so running `remove` from inside the feature's own
   window still completes the cleanup (the final message disappears with the
   window).

Cleanup is idempotent: stale state (directory deleted by hand, window closed,
etc.) is skipped, and it only errors if nothing at all was found.

`jumux remove --all-done` batch-removes every feature whose most recently
recorded agent status (see [Agent status icons](#agent-status-icons)) is
`done`, running the same steps above for each one. If the feature you are
currently in is among them, it is removed last.

## What `merge` does

For merging a feature in locally instead of going through a PR/MR:

1. Resolves the feature the same way `remove` does, including the
   dirty-working-copy confirmation.
2. Rebases the feature onto `base_bookmark` (default `main`, configurable —
   see [Configuration](#configuration)) and moves `base_bookmark` to the
   rebased tip. This is local only — nothing is pushed.
3. If the rebase leaves the feature conflicted, stops here: the base
   bookmark is left untouched and the workspace/directory/window survive so
   you can resolve the conflict and run `merge` again.
4. Otherwise, cleans up exactly like `remove`: forgets the jj workspace,
   deletes the directory, and kills the tmux window.

## What `sidebar` does

Toggles a live agent sidebar, modeled on
[workmux's sidebar](https://workmux.raine.dev/reference/commands/sidebar):
a narrow pane on the left edge of **every window of every tmux session**.
Each row is a window tagged with `@jumux-feature` (any session, any repo),
showing an agent status icon, `repo/feature`, a right-aligned jj working-copy
icon, and a `!` marker when tmux has flagged activity in that window.

| Column | Icon | Meaning |
|---|---|---|
| agent | `⠋` animated spinner (cyan) | agent is working |
| agent | `?` (yellow) | agent is waiting for input |
| agent | `✓` (green) | agent is done |
| agent | `·` (dim) | no agent status recorded |
| jj | `✓` (green) | working copy clean |
| jj | `●` (yellow) | working copy has changes |
| jj | `?` (dim) | jj state unknown |
| stale | `z` (dim) | idle beyond `stale_after_hours` (no jj changes or hook updates) |

When windows from more than one jumux-managed repo are open, rows are
grouped under a header naming each repo; with only one repo open the
header is omitted. `jumux list` likewise adds a REPO column when it
detects a jumux-tagged tmux window belonging to a different repo.

Keys: `j`/`k` (or arrows) move, `g`/`G` jump to first/last, `Enter` switches
to the selected feature's window (across sessions), `q` closes the sidebar
everywhere — same as running `jumux sidebar` again.

`add` splits a sidebar pane into its new window automatically while the
sidebar is open; windows created by plain tmux pick one up on the next
toggle. Panes only poll jj while their window is visible, so idle windows
cost nothing.

## Agent status icons

The agent column is fed by [Claude Code
hooks](https://docs.anthropic.com/en/docs/claude-code/hooks) calling
`jumux hook <status>`. The first `jumux add` offers to install the
hooks into `~/.claude/settings.json`; to set them up by hand, add:

```json
{
  "hooks": {
    "UserPromptSubmit": [{ "hooks": [{ "type": "command", "command": "jumux hook working" }] }],
    "PostToolUse":      [{ "hooks": [{ "type": "command", "command": "jumux hook working" }] }],
    "Notification":     [{ "hooks": [{ "type": "command", "command": "jumux hook waiting" }] }],
    "Stop":             [{ "hooks": [{ "type": "command", "command": "jumux hook done" }] }]
  }
}
```

(`PostToolUse` flips the status back to working after a permission approval;
without it a row would stay "waiting" until the turn ends.)

The same command also works with [Codex CLI
hooks](https://developers.openai.com/codex/hooks). The first `jumux add`
offers to install them into `~/.codex/hooks.json`; to set them up by hand,
add:

```json
{
  "hooks": {
    "UserPromptSubmit":  [{ "hooks": [{ "type": "command", "command": "jumux hook working" }] }],
    "PostToolUse":       [{ "hooks": [{ "type": "command", "command": "jumux hook working" }] }],
    "PermissionRequest": [{ "hooks": [{ "type": "command", "command": "jumux hook blocked" }] }],
    "Stop":              [{ "hooks": [{ "type": "command", "command": "jumux hook done" }] }]
  }
}
```

Codex has no hook event equivalent to Claude Code's idle notification or a
tool-use-failure event, so status never reaches `waiting` or `error` for a
Codex-driven feature.

`jumux hook` resolves the calling pane's window via `$TMUX_PANE` and
writes a small state file under `~/.local/state/jumux/status/` (or
`$XDG_STATE_HOME/jumux/status/`). Outside tmux or outside an jumux
feature window it does nothing, so the hooks are safe to enable globally.
The sidebar prunes state files for closed windows, `remove` deletes the
feature's file, and a `working` entry with no update for 15 minutes is
treated as unknown.

## Configuration

Global `~/.config/jumux/config.toml`, overridden per-key by
`.jumux.toml` at the main repo root:

```toml
agent = "claude"            # command to run; "{feature}" is substituted if present
select_window = true        # switch to the new window after add
base_revision = "trunk()"   # revset new workspaces are based on
base_bookmark = "main"      # bookmark `jumux merge` rebases onto and moves to the merged tip
window_prefix = ""          # prepended to tmux window names
sidebar_width = 32          # sidebar pane width in columns
sidebar_refresh = 2         # sidebar refresh interval in seconds
notify = true               # send a desktop notification on status changes
stale_after_hours = 168     # idle threshold for the stale indicator; 0 disables it
notify_quiet_start = ""     # start of a daily "HH:MM" window to suppress notifications
notify_quiet_end = ""       # end of that window; leave both unset to disable quiet hours
notify_webhook = ""         # if set, also POST {"title","message"} JSON here on status changes
post_create_hooks = []       # shell commands run (in order) after the workspace/window are created, before the agent starts
pre_remove_hooks = []        # shell commands run (in order) before workspace removal; a failure aborts removal
hook_timeout_seconds = 300   # max seconds a single hook command may run; 0 disables the timeout
```

Run `jumux config show` to see the effective merged value of every key,
along with whether it came from the repo file, the global file, or the
built-in default — handy for debugging precedence between the two files.

`notify` gates the OS desktop notification (`osascript` on macOS,
`notify-send` on Linux) sent by `jumux hook` on a status change to
waiting/done/blocked/error.

`notify_quiet_start`/`notify_quiet_end` bound a daily local-time window
(24h `"HH:MM"`) during which notifications (both the desktop notification
and the webhook) are suppressed — handy for muting notifications
overnight. A window where start is after end wraps past midnight, e.g.
`notify_quiet_start = "22:00"`, `notify_quiet_end = "06:00"`. Leave either
unset to disable quiet hours.

`notify_webhook`, if set, additionally POSTs a JSON body
(`{"title": "...", "message": "..."}`) to that URL on the same status
changes — useful for routing notifications somewhere other than the local
desktop (chat webhook, phone push gateway, etc.) when you're away from
the machine. A webhook failure is logged but never fails the hook.

`post_create_hooks`/`pre_remove_hooks` run each command in order via `sh -c`
in the workspace directory, with output streamed live to your terminal and
`JUMUX_EVENT`/`JUMUX_FEATURE`/`JUMUX_WORKSPACE_PATH`/`JUMUX_REPO_ROOT`/
`JUMUX_WINDOW_NAME` set in the environment. A failing (or timed-out) command
stops the remaining commands and aborts the operation: `add` rolls back the
workspace and tmux window it just created, and `remove` never touches the
jj workspace or the directory. `pre_remove_hooks` always run, even with
`-f`/`--force` (force only skips the interactive dirty-workspace prompt).
`pre_remove_hooks` only run when the workspace directory still exists, so
cleaning up a partial state (e.g. a stale jj workspace entry or tmux window
whose directory is already gone) is never blocked by a hook.
`hook_timeout_seconds` bounds each command individually; set it to `0` to
disable the timeout. Note: only the immediate `sh` process is killed on
timeout — a hook that backgrounds work or spawns children may leave those
running.

Example with a starting prompt:

```toml
agent = "claude 'Work on the {feature} feature'"
```

Note on `base_revision`: jj's builtin `trunk()` resolves via *remote*
bookmarks (`main@origin` etc.), which can fail to resolve in a local-only
repo with no remote. If the default `base_revision` fails and you haven't
overridden it, `jumux add` automatically retries with `@-` and prints a
warning; set `base_revision = "main"` (or similar) explicitly to avoid the
fallback and its warning.

Note on colocation: depending on your jj version, secondary workspaces may
not contain a `.git` directory even when the main repo is colocated, so
agents should use `jj` commands inside the workspace.

### Templates

Named templates bundle a `base_revision`/`agent`/window-option preset for a
recurring kind of feature, selectable with `jumux add --template <name>`
(`-t` for short):

```toml
[templates.bugfix]
base_revision = "main"
agent = "claude 'fix the {feature} bug'"
window_prefix = "bug-"
```

Only the fields set in the template override the base config; anything
left unset falls through to the regular `agent`/`base_revision`/etc.
values. `-a`/`--agent` still wins over a template's `agent` if both are
given. A template defined in `.jumux.toml` fully replaces a global
template of the same name (its fields are not merged individually).

`post_create_hooks` follows the same rule: a template that sets it replaces
the base `post_create_hooks` list entirely (not merged/appended). There is
no per-template override for `pre_remove_hooks`, since removal has no
reliable way to know which template a workspace was originally created
with — `pre_remove_hooks` always comes from the regular (non-template)
config.

### Files

`[files]` lists glob patterns (relative to the main repo root) to copy or
symlink into a new workspace on `jumux add`, and to re-apply to an existing
workspace with `jumux sync [name]` — handy for gitignored files a workspace
needs to actually build/run (`.env`, `node_modules`, a virtualenv, build
caches):

```toml
[files]
copy = [".env", "config/local.*"]
symlink = ["node_modules", ".venv"]
```

Patterns are resolved with `filepath.Glob` (so `*`/`?`/`[...]` work, but not
`**`); a pattern matching nothing is skipped silently, since these files are
often optional. A destination that already exists is left untouched and
reported as a warning rather than overwritten, so re-running `jumux sync` is
safe. A copy/symlink failure only prints a warning — it never blocks `jumux
add` from finishing.

## Install

```
brew install richardcase/tap/jumux
```

or, with Go:

```
go install github.com/richardcase/jumux@latest
```

The binary is named `jumux`. If you'd rather type `jjm`, add an alias:

```
alias jjm=jumux
```

## Development tooling

This repo uses [mise](https://mise.jdx.dev/) to pin the Go, golangci-lint,
and goreleaser versions used to build, test, and release it (see
`mise.toml`). After installing mise, run:

```
mise install
```

`make build`, `make test`, `make lint`, and `make vet` all run through
`mise exec`, so they automatically use the pinned tool versions.

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for how to
build, test, and submit changes, and [AGENTS.md](AGENTS.md) for project
layout and conventions. Please note this project follows a
[Code of Conduct](CODE_OF_CONDUCT.md).

## License

jumux is licensed under the [Apache License 2.0](LICENSE).

## Acknowledgements

jumux was inspired by [workmux](https://github.com/nikolaeu/workmux).
