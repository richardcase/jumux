package app

import (
	"fmt"
	"os"

	"github.com/richardcase/agentmux/internal/jj"
	"github.com/richardcase/agentmux/internal/tmuxctl"
)

// Add creates a jj workspace for feature, opens a tmux window in it, and
// starts the configured agent.
func (a *App) Add(feature string) error {
	if err := validFeatureName(feature); err != nil {
		return err
	}
	if err := a.requireTmux(); err != nil {
		return err
	}
	ctx, err := a.repoContext()
	if err != nil {
		return err
	}
	a.ensureClaudeHooks()
	if !jj.IsColocated(ctx.MainRoot) {
		_, _ = fmt.Fprintf(a.Errw, "warning: %s is not a colocated jj repo; git tooling will not work in the workspace\n", ctx.MainRoot)
	}

	wsPath := workspacePath(ctx.MainRoot, feature)
	windowName := ctx.Config.WindowPrefix + feature

	// Pre-flight checks for friendly errors.
	names, err := jj.Workspaces(a.Runner, ctx.MainRoot)
	if err != nil {
		return err
	}
	if contains(names, feature) {
		return fmt.Errorf("workspace %q already exists (agentmux remove %s first)", feature, feature)
	}
	if _, err := os.Stat(wsPath); err == nil {
		return fmt.Errorf("directory %s already exists", wsPath)
	}
	windows, err := tmuxctl.ListWindows(a.Runner)
	if err != nil {
		return err
	}
	if w, ok := tmuxctl.FindWindow(windows, feature, windowName); ok {
		return fmt.Errorf("tmux window %s (%s) already belongs to feature %q", w.Name, w.ID, feature)
	}

	if err := jj.WorkspaceAdd(a.Runner, ctx.MainRoot, feature, wsPath, ctx.Config.BaseRevision); err != nil {
		return err
	}
	rollbackWorkspace := func() {
		if ferr := jj.WorkspaceForget(a.Runner, ctx.MainRoot, feature); ferr != nil {
			_, _ = fmt.Fprintf(a.Errw, "rollback: %v\n", ferr)
		}
		if rerr := os.RemoveAll(wsPath); rerr != nil {
			_, _ = fmt.Fprintf(a.Errw, "rollback: %v\n", rerr)
		}
	}

	windowID, err := tmuxctl.NewWindow(a.Runner, windowName, wsPath)
	if err != nil {
		rollbackWorkspace()
		return err
	}
	rollbackAll := func() {
		if kerr := tmuxctl.KillWindow(a.Runner, windowID); kerr != nil {
			_, _ = fmt.Fprintf(a.Errw, "rollback: %v\n", kerr)
		}
		rollbackWorkspace()
	}

	if err := tmuxctl.Configure(a.Runner, windowID, feature); err != nil {
		rollbackAll()
		return err
	}
	if err := tmuxctl.SendCommand(a.Runner, windowID, ctx.Config.AgentCommand(feature)); err != nil {
		rollbackAll()
		return err
	}
	a.ensureSidebarPane(ctx.Config, windowID)
	if ctx.Config.SelectWindowEnabled() {
		if err := tmuxctl.SelectWindow(a.Runner, windowID); err != nil {
			return err
		}
	}

	_, _ = fmt.Fprintf(a.Out, "added feature %q: workspace %s, tmux window %s (%s)\n",
		feature, wsPath, windowName, windowID)
	return nil
}
