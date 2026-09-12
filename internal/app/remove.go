package app

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/richardcase/jumux/internal/agentstate"
	"github.com/richardcase/jumux/internal/config"
	"github.com/richardcase/jumux/internal/jj"
	"github.com/richardcase/jumux/internal/sidebar"
	"github.com/richardcase/jumux/internal/tmuxctl"
)

// workspaceTarget is a feature resolved against the repo's jj workspaces,
// filesystem, and tmux windows, ready for cleanupWorkspace (and, before
// that, whatever a caller like Merge needs to do to the feature itself).
type workspaceTarget struct {
	name        string
	wsPath      string
	inList      bool
	dirExists   bool
	window      tmuxctl.Window
	windowFound bool
}

// resolveTarget resolves name to a workspaceTarget (inferring the current
// feature if name is empty), refusing the default workspace and invalid
// names, and, unless force is set, confirming before proceeding if the
// workspace's working-copy commit is dirty. action names the caller's
// operation ("remove", "merge") for its error/prompt wording.
func (a *App) resolveTarget(ctx *repoContext, name string, force bool, action string) (workspaceTarget, error) {
	names, err := jj.Workspaces(a.Runner, ctx.MainRoot)
	if err != nil {
		return workspaceTarget{}, err
	}

	if name == "" {
		name, err = a.inferFeature(ctx, names)
		if err != nil {
			return workspaceTarget{}, err
		}
	}
	if name == "default" {
		return workspaceTarget{}, fmt.Errorf("refusing to %s the default workspace", action)
	}
	if err := validFeatureName(name); err != nil {
		return workspaceTarget{}, err
	}

	wsPath := a.workspacePath(ctx.MainRoot, name)
	inList := contains(names, name)
	_, statErr := os.Stat(wsPath)
	dirExists := statErr == nil

	windows, err := tmuxctl.ListWindows(a.Runner)
	if err != nil {
		return workspaceTarget{}, err
	}
	window, windowFound := tmuxctl.FindWindow(windows, name, ctx.Config.WindowPrefix+name)

	if !inList && !dirExists && !windowFound {
		return workspaceTarget{}, fmt.Errorf("nothing to %s for feature %q: no workspace, directory, or tmux window found", action, name)
	}

	if inList && dirExists && !force {
		dirty, err := jj.IsDirty(a.Runner, wsPath, name)
		if err != nil {
			return workspaceTarget{}, err
		}
		if dirty && !a.confirm(fmt.Sprintf("workspace %q has changes in its working-copy commit; %s anyway?", name, action)) {
			return workspaceTarget{}, fmt.Errorf("aborted")
		}
	}

	return workspaceTarget{
		name:        name,
		wsPath:      wsPath,
		inList:      inList,
		dirExists:   dirExists,
		window:      window,
		windowFound: windowFound,
	}, nil
}

// cleanupWorkspace forgets t's jj workspace, deletes its directory, and
// kills its tmux window, in that order. The window is killed last so
// cleaning up the feature you are inside still completes the jj and
// filesystem cleanup. Callers must have already left wsPath (e.g. via
// os.Chdir) before calling this.
//
// Pre-remove hooks run first, only once teardown is actually going ahead
// (callers must have already handled any decline-to-confirm before calling
// this), so a hook with real side effects (tearing down a dev environment,
// freeing external resources) never fires for a removal the user cancelled.
// They run regardless of -f/--force (force only bypasses the confirmation
// prompt) and before anything destructive below.
func (a *App) cleanupWorkspace(ctx *repoContext, t workspaceTarget) error {
	windowName := ""
	if t.windowFound {
		windowName = t.window.Name
	}
	if t.dirExists {
		if err := runHooks(a.HookRunner, t.wsPath, ctx.Config.PreRemoveHooks, ctx.Config.HookTimeout(),
			hookEnv("pre_remove", t.name, t.wsPath, ctx.MainRoot, windowName)); err != nil {
			return err
		}
	}
	if t.inList {
		if err := jj.WorkspaceForget(a.Runner, ctx.MainRoot, t.name); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(a.Out, "forgot jj workspace %q\n", t.name)
	}
	if t.dirExists {
		if err := os.RemoveAll(t.wsPath); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(a.Out, "deleted %s\n", t.wsPath)
	}
	if t.windowFound {
		if err := agentstate.Remove(a.StateDir, t.window.ID); err != nil {
			_, _ = fmt.Fprintf(a.Errw, "removing agent state: %v\n", err)
		}
		// Killed last: if it is our own window this ends the process.
		_, _ = fmt.Fprintf(a.Out, "killing tmux window %s (%s)\n", t.window.Name, t.window.ID)
		if err := tmuxctl.KillWindow(a.Runner, t.window.ID); err != nil {
			return err
		}
	}
	return nil
}

// Remove tears down a feature: forgets the jj workspace, deletes its
// directory, and kills its tmux window. If name is empty the current
// feature is inferred (cwd first, then the current window's tag).
// The window is killed last so removing the feature you are inside still
// completes the jj and filesystem cleanup.
func (a *App) Remove(name string, force bool) error {
	if err := a.requireTmux(); err != nil {
		return err
	}
	ctx, err := a.repoContext()
	if err != nil {
		return err
	}
	target, err := a.resolveTarget(ctx, name, force, "remove")
	if err != nil {
		return err
	}

	// Get out of the directory we are about to delete.
	if err := os.Chdir(ctx.MainRoot); err != nil {
		return err
	}
	return a.cleanupWorkspace(ctx, target)
}

// RemoveTarget tears down an explicit target's feature, the same way Remove
// does, but never re-resolves the repo or window from the acting process's
// own cwd/tmux session: it acts only on target.MainRoot and target.WindowID.
// This is what the sidebar uses, since its rows span every repo and tmux
// session and re-resolving from ambient state could hit the wrong one.
func (a *App) RemoveTarget(target sidebar.Target, force bool) error {
	if err := a.requireTmux(); err != nil {
		return err
	}
	name := target.Feature
	if name == "default" {
		return fmt.Errorf("refusing to remove the default workspace")
	}
	if err := validFeatureName(name); err != nil {
		return err
	}

	names, err := jj.Workspaces(a.Runner, target.MainRoot)
	if err != nil {
		return err
	}
	cfg, err := config.Load(a.GlobalConfig, target.MainRoot)
	if err != nil {
		return err
	}
	wsPath := a.workspacePath(target.MainRoot, name)
	inList := contains(names, name)
	_, statErr := os.Stat(wsPath)
	dirExists := statErr == nil
	windowFound := target.WindowID != ""

	if !inList && !dirExists && !windowFound {
		return fmt.Errorf("nothing to remove for feature %q: no workspace, directory, or tmux window found", name)
	}

	if inList && dirExists && !force {
		dirty, err := jj.IsDirty(a.Runner, wsPath, name)
		if err != nil {
			return err
		}
		if dirty && !a.confirm(fmt.Sprintf("workspace %q has changes in its working-copy commit; remove anyway?", name)) {
			return fmt.Errorf("aborted")
		}
	}

	// Pre-remove hooks run only once removal is actually going ahead (after
	// any decline-to-confirm has already returned) — see the matching
	// comment in Remove. Unlike Remove, RemoveTarget only has an opaque
	// tmux window ID (target.WindowID, e.g. "@7"), never an actual window
	// name, so passing it as windowName would put a structurally different
	// kind of value into JUMUX_WINDOW_NAME depending on which path removed
	// the workspace. Pass "" so hookEnv omits the var entirely rather than
	// lie about it.
	if dirExists {
		if err := runHooks(a.HookRunner, wsPath, cfg.PreRemoveHooks, cfg.HookTimeout(),
			hookEnv("pre_remove", name, wsPath, target.MainRoot, "")); err != nil {
			return err
		}
	}

	if inList {
		if err := jj.WorkspaceForget(a.Runner, target.MainRoot, name); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(a.Out, "forgot jj workspace %q\n", name)
	}
	if dirExists {
		if err := os.RemoveAll(wsPath); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(a.Out, "deleted %s\n", wsPath)
	}
	if windowFound {
		if err := agentstate.Remove(a.StateDir, target.WindowID); err != nil {
			_, _ = fmt.Fprintf(a.Errw, "removing agent state: %v\n", err)
		}
		// Killed last: if it is our own window this ends the process.
		_, _ = fmt.Fprintf(a.Out, "killing tmux window %s\n", target.WindowID)
		if err := tmuxctl.KillWindow(a.Runner, target.WindowID); err != nil {
			return err
		}
	}
	return nil
}

// RemoveAllDone removes every feature whose most recently recorded agent
// status is "done", reusing Remove's single-feature logic for each one
// (so the usual dirty-working-copy confirmation and force behavior still
// apply per feature). If the current feature is among them it is removed
// last, since killing its own tmux window ends this process.
func (a *App) RemoveAllDone(force bool) error {
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
	agent := agentstate.ReadAll(a.StateDir, a.now())

	var done []string
	for _, name := range names {
		if name == "default" {
			continue
		}
		win, ok := tmuxctl.FindWindow(windows, name, ctx.Config.WindowPrefix+name)
		if !ok {
			continue
		}
		if agent[win.ID] == agentstate.Done {
			done = append(done, name)
		}
	}
	if len(done) == 0 {
		_, _ = fmt.Fprintln(a.Out, "no done features to remove")
		return nil
	}
	if current, err := a.inferFeature(ctx, names); err == nil {
		moveToEnd(done, current)
	}

	var firstErr error
	for _, name := range done {
		_, _ = fmt.Fprintf(a.Out, "removing %q (done)\n", name)
		if err := a.Remove(name, force); err != nil {
			_, _ = fmt.Fprintf(a.Errw, "removing %q: %v\n", name, err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// moveToEnd moves name to the end of names, if present, preserving the
// relative order of everything else.
func moveToEnd(names []string, name string) {
	for i, n := range names {
		if n != name {
			continue
		}
		copy(names[i:], names[i+1:])
		names[len(names)-1] = name
		return
	}
}

func (a *App) confirm(prompt string) bool {
	_, _ = fmt.Fprintf(a.Errw, "%s [y/N] ", prompt)
	line, err := bufio.NewReader(a.In).ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}
