package app

import (
	"fmt"

	"github.com/richardcase/agentmux/internal/agentstate"
	"github.com/richardcase/agentmux/internal/tmuxctl"
)

// Hook records the agent status for the window containing the calling pane.
// It is invoked from Claude Code hooks, so outside tmux or outside an
// agentmux feature window it exits silently — a misconfigured hook must
// never break the agent.
func (a *App) Hook(status string) error {
	s := agentstate.Status(status)
	if !agentstate.Valid(s) {
		return fmt.Errorf("invalid status %q (want working|waiting|done)", status)
	}
	paneID := a.Getenv("TMUX_PANE")
	if paneID == "" {
		return nil
	}
	windowID, feature, err := tmuxctl.PaneWindowInfo(a.Runner, paneID)
	if err != nil || windowID == "" || feature == "" {
		return nil
	}
	if err := agentstate.Write(a.StateDir, agentstate.Entry{
		WindowID:  windowID,
		PaneID:    paneID,
		Status:    s,
		UpdatedAt: a.now(),
	}); err != nil {
		_, _ = fmt.Fprintf(a.Errw, "agentmux hook: %v\n", err)
	}
	return nil
}
