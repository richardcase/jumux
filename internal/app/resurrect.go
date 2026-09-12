package app

import (
	"fmt"
	"os"

	"github.com/richardcase/jumux/internal/agentstate"
	"github.com/richardcase/jumux/internal/jj"
	"github.com/richardcase/jumux/internal/tmuxctl"
)

// Resurrect recreates tmux windows for every feature workspace that has
// lost its window, e.g. after a tmux server crash or restart wiped every
// jumux-managed window while the jj workspaces on disk survived. It never
// touches jj: a feature is only restored if its workspace directory already
// exists, and skipped (already has a window) or reported (workspace
// missing) otherwise. Safe to re-run: features with a live window are left
// untouched.
func (a *App) Resurrect() error {
	if err := a.requireTmux(); err != nil {
		return err
	}
	ctx, err := a.repoContext()
	if err != nil {
		return err
	}
	names, err := jj.Workspaces(a.Runner, ctx.MainRoot)
	if err != nil {
		return err
	}
	windows, err := tmuxctl.ListWindows(a.Runner)
	if err != nil {
		return err
	}

	restored := 0
	var firstErr error
	for _, name := range names {
		if name == "default" {
			continue
		}
		wsPath := a.workspacePath(ctx.MainRoot, name)
		if _, err := os.Stat(wsPath); err != nil {
			_, _ = fmt.Fprintf(a.Errw, "skipping %q: workspace directory missing\n", name)
			continue
		}
		windowName := ctx.Config.WindowPrefix + name
		if _, ok := tmuxctl.FindWindow(windows, name, windowName); ok {
			continue // already has a window; nothing to do
		}

		windowID, err := tmuxctl.NewWindow(a.Runner, windowName, wsPath)
		if err != nil {
			_, _ = fmt.Fprintf(a.Errw, "restoring %q: %v\n", name, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := tmuxctl.Configure(a.Runner, windowID, name); err != nil {
			_, _ = fmt.Fprintf(a.Errw, "restoring %q: %v\n", name, err)
			_ = tmuxctl.KillWindow(a.Runner, windowID)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := tmuxctl.SendCommand(a.Runner, windowID, ctx.Config.AgentCommand(name, "")); err != nil {
			_, _ = fmt.Fprintf(a.Errw, "restoring %q: %v\n", name, err)
			_ = tmuxctl.KillWindow(a.Runner, windowID)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		a.ensureSidebarPane(ctx.Config, windowID)

		_, _ = fmt.Fprintf(a.Out, "restored feature %q: workspace %s, tmux window %s (%s)\n",
			name, wsPath, windowName, windowID)
		restored++
	}

	if restored == 0 && firstErr == nil {
		_, _ = fmt.Fprintln(a.Out, "nothing to restore")
	}

	// Best-effort: drop agent-status entries left over from the crashed
	// server, now that we know which window IDs are actually live.
	if live, err := tmuxctl.ListWindows(a.Runner); err == nil {
		liveIDs := map[string]bool{}
		for _, w := range live {
			liveIDs[w.ID] = true
		}
		_ = agentstate.Prune(a.StateDir, liveIDs)
	}

	return firstErr
}
