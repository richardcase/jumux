package app

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/richardcase/jumux/internal/agentstate"
	"github.com/richardcase/jumux/internal/jj"
	"github.com/richardcase/jumux/internal/run"
	"github.com/richardcase/jumux/internal/tmuxctl"
)

// List prints every non-default jj workspace joined with its tmux window.
func (a *App) List() error {
	ctx, err := a.repoContext()
	if err != nil {
		return err
	}
	names, err := jj.Workspaces(a.Runner, ctx.MainRoot)
	if err != nil {
		return err
	}
	var windows []tmuxctl.Window
	showRepo := false
	if a.Getenv("TMUX") != "" {
		if windows, err = tmuxctl.ListWindows(a.Runner); err != nil {
			return err
		}
		if all, err := tmuxctl.ListAllWindows(a.Runner); err == nil {
			showRepo = otherRepoOpen(a.Runner, all, ctx.MainRoot)
		}
	}
	threshold, staleEnabled := ctx.Config.StaleThreshold()
	now := a.now()
	lastHookUpdate := agentstate.LastUpdated(a.StateDir)
	repo := filepath.Base(ctx.MainRoot)

	w := tabwriter.NewWriter(a.Out, 2, 4, 2, ' ', 0)
	if showRepo {
		_, _ = fmt.Fprintln(w, "REPO\tFEATURE\tWORKSPACE\tWINDOW\tAGENT\tSTATUS\tIDLE")
	} else {
		_, _ = fmt.Fprintln(w, "FEATURE\tWORKSPACE\tWINDOW\tAGENT\tSTATUS\tIDLE")
	}
	count := 0
	for _, name := range names {
		if name == "default" {
			continue
		}
		count++
		wsPath := a.workspacePath(ctx.MainRoot, name)
		status := "clean"
		wsExists := true
		if _, err := os.Stat(wsPath); err != nil {
			wsPath = "-"
			status = "stale"
			wsExists = false
		} else if dirty, err := jj.IsDirty(a.Runner, wsPath, name); err == nil && dirty {
			status = "dirty"
		}
		windowCol := "-"
		agentCol := "-"
		win, hasWindow := tmuxctl.FindWindow(windows, name, ctx.Config.WindowPrefix+name)
		if hasWindow {
			windowCol = fmt.Sprintf("%s (%s)", win.Name, win.ID)
			if win.Dead {
				agentCol = "dead (jumux restart " + name + ")"
			}
		}
		idleCol := "-"
		if staleEnabled {
			if last, ok := lastActivity(a, wsExists, wsPath, name, hasWindow, win.ID, lastHookUpdate); ok {
				idleCol = formatIdle(now.Sub(last), now.Sub(last) > threshold)
			}
		}
		if showRepo {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", repo, name, wsPath, windowCol, agentCol, status, idleCol)
		} else {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", name, wsPath, windowCol, agentCol, status, idleCol)
		}
	}
	if count == 0 {
		return w.Flush() // header only; keep output predictable
	}
	return w.Flush()
}

// otherRepoOpen reports whether any jumux-tagged tmux window (any session)
// resolves to a jj repo other than mainRoot, meaning more than one
// jumux-managed repo is currently open.
func otherRepoOpen(r run.Runner, windows []tmuxctl.GlobalWindow, mainRoot string) bool {
	seen := map[string]bool{} // path -> already resolved
	for _, win := range windows {
		if win.Feature == "" || win.Path == "" || seen[win.Path] {
			continue
		}
		seen[win.Path] = true
		wsRoot, err := jj.Root(r, win.Path)
		if err != nil {
			continue
		}
		otherRoot, err := jj.MainRoot(wsRoot)
		if err != nil {
			continue
		}
		if otherRoot != mainRoot {
			return true
		}
	}
	return false
}

// lastActivity returns the most recent of a feature's last jj change and
// last hook status update, and whether either was available.
func lastActivity(a *App, wsExists bool, wsPath, name string, hasWindow bool, windowID string, hookUpdates map[string]time.Time) (time.Time, bool) {
	var latest time.Time
	found := false
	if wsExists {
		if t, err := jj.LastChangeTime(a.Runner, wsPath, name); err == nil {
			latest = t
			found = true
		}
	}
	if hasWindow {
		if t, ok := hookUpdates[windowID]; ok && (!found || t.After(latest)) {
			latest = t
			found = true
		}
	}
	return latest, found
}

// formatIdle renders a rough "time since last activity" string, marking it
// "(idle)" when past the configured stale threshold.
func formatIdle(d time.Duration, stale bool) string {
	s := humanDuration(d)
	if stale {
		return s + " (idle)"
	}
	return s
}

// humanDuration renders a coarse single-unit duration (minutes, hours, or
// days) for compact display in list/sidebar output.
func humanDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
