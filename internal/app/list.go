package app

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/richardcase/jumux/internal/jj"
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
	if a.Getenv("TMUX") != "" {
		if windows, err = tmuxctl.ListWindows(a.Runner); err != nil {
			return err
		}
	}

	w := tabwriter.NewWriter(a.Out, 2, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "FEATURE\tWORKSPACE\tWINDOW\tAGENT\tSTATUS")
	count := 0
	for _, name := range names {
		if name == "default" {
			continue
		}
		count++
		wsPath := workspacePath(ctx.MainRoot, name)
		status := "clean"
		if _, err := os.Stat(wsPath); err != nil {
			wsPath = "-"
			status = "stale"
		} else if dirty, err := jj.IsDirty(a.Runner, wsPath, name); err == nil && dirty {
			status = "dirty"
		}
		windowCol := "-"
		agentCol := "-"
		if win, ok := tmuxctl.FindWindow(windows, name, ctx.Config.WindowPrefix+name); ok {
			windowCol = fmt.Sprintf("%s (%s)", win.Name, win.ID)
			if win.Dead {
				agentCol = "dead (jumux restart " + name + ")"
			}
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", name, wsPath, windowCol, agentCol, status)
	}
	if count == 0 {
		return w.Flush() // header only; keep output predictable
	}
	return w.Flush()
}
