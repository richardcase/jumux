package app

import (
	"fmt"

	"github.com/richardcase/jumux/internal/agentstate"
	"github.com/richardcase/jumux/internal/notify"
	"github.com/richardcase/jumux/internal/tmuxctl"
)

// Hook records the agent status for the window containing the calling pane.
// It is invoked from Claude Code hooks, so outside tmux or outside an
// jumux feature window it exits silently — a misconfigured hook must
// never break the agent.
func (a *App) Hook(status string) error {
	s := agentstate.Status(status)
	if !agentstate.Valid(s) {
		return fmt.Errorf("invalid status %q (want working|waiting|done|blocked|error)", status)
	}
	paneID := a.Getenv("TMUX_PANE")
	if paneID == "" {
		return nil
	}
	windowID, feature, err := tmuxctl.PaneWindowInfo(a.Runner, paneID)
	if err != nil || windowID == "" || feature == "" {
		return nil
	}
	now := a.now()
	prev := agentstate.ReadAll(a.StateDir, now)[windowID]
	if err := agentstate.Write(a.StateDir, agentstate.Entry{
		WindowID:  windowID,
		PaneID:    paneID,
		Status:    s,
		UpdatedAt: now,
	}); err != nil {
		_, _ = fmt.Fprintf(a.Errw, "jumux hook: %v\n", err)
	}
	a.maybeNotify(feature, prev, s)
	return nil
}

// maybeNotify sends a desktop notification when the agent has newly moved
// to a status the user cares about (waiting for input, done, blocked, or
// errored). It never fires for a status the window was already in, so
// repeated hook calls with the same status (e.g. multiple PostToolUse
// events) don't spam.
func (a *App) maybeNotify(feature string, prev, next agentstate.Status) {
	switch next {
	case agentstate.Waiting, agentstate.Done, agentstate.Blocked, agentstate.Error:
	default:
		return
	}
	if prev == next {
		return
	}
	notifyEnabled := true
	webhook := ""
	if rc, err := a.repoContext(); err == nil {
		notifyEnabled = rc.Config.NotifyEnabled()
		if rc.Config.InQuietHours(a.now()) {
			return
		}
		webhook = rc.Config.NotifyWebhook
	}
	if !notifyEnabled {
		return
	}
	title, message := "jumux: "+feature, string(next)
	if err := notify.Send(a.Runner, title, message); err != nil {
		_, _ = fmt.Fprintf(a.Errw, "jumux hook: notify: %v\n", err)
	}
	if webhook != "" {
		if err := notify.SendWebhook(a.HTTPClient, webhook, title, message); err != nil {
			_, _ = fmt.Fprintf(a.Errw, "jumux hook: notify: %v\n", err)
		}
	}
}
