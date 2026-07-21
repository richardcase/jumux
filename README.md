# agentmux

Work on multiple features in parallel with coding agents, pairing a
[jujutsu](https://github.com/jj-vcs/jj) workspace with a tmux window per
feature.

```
agentmux add <feature>       create a jj workspace + tmux window, start the agent
agentmux remove [-f] [name]  tear a feature down (defaults to the current one)
agentmux list                show feature workspaces and their tmux windows
```

## What `add` does

Run inside tmux, anywhere in a jj repo (ideally colocated with git):

1. Creates a jj workspace named `<feature>` in a sibling directory
   `../<repo>-<feature>`, based on `trunk()`.
2. Opens a tmux window named `<feature>` in that directory. The window's
   name is pinned (`automatic-rename off`) and tagged with the custom option
   `@agentmux-feature` so agentmux can find it again even if it gets renamed.
3. Types the configured agent command into the window and presses Enter, then
   switches to it. If the agent exits, the window's shell survives for
   inspection.

## What `remove` does

1. Resolves the feature: the argument, else the workspace your cwd is in,
   else the current window's `@agentmux-feature` tag. `default` is refused.
2. If the workspace's working-copy commit has changes, asks for confirmation
   (`-f`/`-force` skips). Note that `jj workspace forget` never deletes
   commits — described work stays in the repo either way.
3. Forgets the jj workspace, deletes the directory, and kills the tmux
   window — in that order, so running `remove` from inside the feature's own
   window still completes the cleanup (the final message disappears with the
   window).

Cleanup is idempotent: stale state (directory deleted by hand, window closed,
etc.) is skipped, and it only errors if nothing at all was found.

## Configuration

Global `~/.config/agentmux/config.toml`, overridden per-key by
`.agentmux.toml` at the main repo root:

```toml
agent = "claude"            # command to run; "{feature}" is substituted if present
select_window = true        # switch to the new window after add
base_revision = "trunk()"   # revset new workspaces are based on
window_prefix = ""          # prepended to tmux window names
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
go install github.com/richardcase/agentmux@latest
```
