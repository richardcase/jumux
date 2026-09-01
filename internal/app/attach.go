package app

import (
	"fmt"

	"github.com/richardcase/jumux/internal/jj"
	"github.com/richardcase/jumux/internal/tmuxctl"
)

// Attach switches the current tmux client to a feature's existing window,
// searching across every session, without touching jj or workspace state.
func (a *App) Attach(name string) error {
	if err := a.requireTmux(); err != nil {
		return err
	}
	if err := validFeatureName(name); err != nil {
		return err
	}
	ctx, err := a.repoContext()
	if err != nil {
		return err
	}

	windows, err := tmuxctl.ListAllWindows(a.Runner)
	if err != nil {
		return err
	}
	candidates := tmuxctl.FindGlobalWindows(windows, name, ctx.Config.WindowPrefix+name)
	if len(candidates) == 0 {
		return fmt.Errorf("no tmux window found for feature %q", name)
	}
	window, ok := a.windowInRepo(candidates, ctx.MainRoot)
	if !ok {
		return fmt.Errorf("no tmux window found for feature %q in this repository", name)
	}

	if err := tmuxctl.SwitchToWindow(a.Runner, window.SessionID, window.ID); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(a.Out, "attached to feature %q: window %s (%s)\n", name, window.Name, window.ID)
	return nil
}

// windowInRepo returns the first candidate whose canonical jj main root
// matches mainRoot, skipping candidates whose path no longer resolves to a
// jj workspace (e.g. a deleted worktree) rather than failing outright.
func (a *App) windowInRepo(candidates []tmuxctl.GlobalWindow, mainRoot string) (tmuxctl.GlobalWindow, bool) {
	for _, w := range candidates {
		wsRoot, err := jj.Root(a.Runner, w.Path)
		if err != nil {
			continue
		}
		root, err := jj.MainRoot(wsRoot)
		if err != nil {
			continue
		}
		if root == mainRoot {
			return w, true
		}
	}
	return tmuxctl.GlobalWindow{}, false
}
