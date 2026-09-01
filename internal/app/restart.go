package app

import (
	"fmt"

	"github.com/richardcase/jumux/internal/config"
	"github.com/richardcase/jumux/internal/jj"
	"github.com/richardcase/jumux/internal/sidebar"
	"github.com/richardcase/jumux/internal/tmuxctl"
)

// Restart restarts the configured agent command in place in a feature's
// tmux window: it respawns the window's pane (killing any surviving
// process first) and re-sends the agent command. name defaults to the
// current feature, same as Remove. If the pane does not look dead, it
// asks for confirmation unless force is set.
func (a *App) Restart(name string, force bool) error {
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
	if name == "" {
		name, err = a.inferFeature(ctx, names)
		if err != nil {
			return err
		}
	}
	if err := validFeatureName(name); err != nil {
		return err
	}

	windows, err := tmuxctl.ListWindows(a.Runner)
	if err != nil {
		return err
	}
	window, ok := tmuxctl.FindWindow(windows, name, ctx.Config.WindowPrefix+name)
	if !ok {
		return fmt.Errorf("no tmux window found for feature %q", name)
	}

	if !window.Dead && !force {
		if !a.confirm(fmt.Sprintf("window %s (%s) does not look dead; restart the agent anyway?", window.Name, window.ID)) {
			return fmt.Errorf("aborted")
		}
	}

	if err := tmuxctl.RespawnPane(a.Runner, window.ID); err != nil {
		return err
	}
	if err := tmuxctl.SendCommand(a.Runner, window.ID, ctx.Config.AgentCommand(name, "")); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(a.Out, "restarted agent for feature %q in window %s (%s)\n", name, window.Name, window.ID)
	return nil
}

// RestartTarget restarts an explicit target's agent, the same way Restart
// does, but never re-resolves the window from the acting process's own
// tmux session: it acts only on target.WindowID directly. This is what the
// sidebar uses, since its rows span every tmux session and a session-scoped
// window search could miss the right window or match the wrong one.
func (a *App) RestartTarget(target sidebar.Target, force bool) error {
	if err := a.requireTmux(); err != nil {
		return err
	}
	if err := validFeatureName(target.Feature); err != nil {
		return err
	}
	if target.WindowID == "" {
		return fmt.Errorf("no tmux window found for feature %q", target.Feature)
	}

	if !force {
		if !a.confirm(fmt.Sprintf("window %s does not look dead; restart the agent anyway?", target.WindowID)) {
			return fmt.Errorf("aborted")
		}
	}

	if err := tmuxctl.RespawnPane(a.Runner, target.WindowID); err != nil {
		return err
	}
	cfg, err := config.Load(a.GlobalConfig, target.MainRoot)
	if err != nil {
		return err
	}
	if err := tmuxctl.SendCommand(a.Runner, target.WindowID, cfg.AgentCommand(target.Feature, "")); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(a.Out, "restarted agent for feature %q in window %s\n", target.Feature, target.WindowID)
	return nil
}
