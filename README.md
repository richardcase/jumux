# jumux

Work on multiple features in parallel with coding agents, pairing a
[jujutsu](https://github.com/jj-vcs/jj) workspace with a tmux window per
feature.

```
jumux add <feature>       create a jj workspace + tmux window, start the agent
jumux remove [-f] [name]  tear a feature down (defaults to the current one)
jumux list                show feature workspaces and their tmux windows
jumux sidebar             toggle a live agent sidebar pane on every tmux window
jumux hook <status>       record agent status (working|waiting|done) from hooks
```

## What `add` does

Run inside tmux, anywhere in a jj repo (ideally colocated with git):

1. Creates a jj workspace named `<feature>` in a sibling directory
   `../<repo>-<feature>`, based on `trunk()`.
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
window_prefix = ""          # prepended to tmux window names
sidebar_width = 32          # sidebar pane width in columns
sidebar_refresh = 2         # sidebar refresh interval in seconds
```

Example with a starting prompt:

```toml
agent = "claude 'Work on the {feature} feature'"
```

Note on `base_revision`: jj's builtin `trunk()` resolves via *remote*
bookmarks (`main@origin` etc.) and falls back to `root()` in a repo with no
remote — set `base_revision = "main"` (or similar) for local-only repos.

Note on colocation: depending on your jj version, secondary workspaces may
not contain a `.git` directory even when the main repo is colocated, so
agents should use `jj` commands inside the workspace.

## Install

```
go install github.com/richardcase/jumux@latest
```

The binary is named `jumux`. If you'd rather type `jjm`, add an alias:

```
alias jjm=jumux
```
