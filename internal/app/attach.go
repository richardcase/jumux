package app

import (
	"fmt"

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
	window, ok := tmuxctl.FindGlobalWindow(windows, name, ctx.Config.WindowPrefix+name)
	if !ok {
		return fmt.Errorf("no tmux window found for feature %q", name)
	}

	if err := tmuxctl.SwitchToWindow(a.Runner, window.SessionID, window.ID); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(a.Out, "attached to feature %q: window %s (%s)\n", name, window.Name, window.ID)
	return nil
}
