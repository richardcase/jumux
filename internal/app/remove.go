package app

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/richardcase/agentmux/internal/jj"
	"github.com/richardcase/agentmux/internal/tmuxctl"
)

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
	if name == "default" {
		return fmt.Errorf("refusing to remove the default workspace")
	}
	if err := validFeatureName(name); err != nil {
		return err
	}

	wsPath := workspacePath(ctx.MainRoot, name)
	inList := contains(names, name)
	_, statErr := os.Stat(wsPath)
	dirExists := statErr == nil

	windows, err := tmuxctl.ListWindows(a.Runner)
	if err != nil {
		return err
	}
	window, windowFound := tmuxctl.FindWindow(windows, name, ctx.Config.WindowPrefix+name)

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

	// Get out of the directory we are about to delete.
	if err := os.Chdir(ctx.MainRoot); err != nil {
		return err
	}

	if inList {
		if err := jj.WorkspaceForget(a.Runner, ctx.MainRoot, name); err != nil {
			return err
		}
		fmt.Fprintf(a.Out, "forgot jj workspace %q\n", name)
	}
	if dirExists {
		if err := os.RemoveAll(wsPath); err != nil {
			return err
		}
		fmt.Fprintf(a.Out, "deleted %s\n", wsPath)
	}
	if windowFound {
		// Killed last: if it is our own window this ends the process.
		fmt.Fprintf(a.Out, "killing tmux window %s (%s)\n", window.Name, window.ID)
		if err := tmuxctl.KillWindow(a.Runner, window.ID); err != nil {
			return err
		}
	}
	return nil
}

// inferFeature determines the current feature: if cwd is inside a
// non-default workspace whose name matches the sibling-dir convention, use
// it; otherwise fall back to the current window's @agentmux-feature tag.
func (a *App) inferFeature(ctx *repoContext, names []string) (string, error) {
	if ctx.WsRoot != ctx.MainRoot {
		base := filepath.Base(ctx.WsRoot)
		prefix := filepath.Base(ctx.MainRoot) + "-"
		if feature := strings.TrimPrefix(base, prefix); feature != base && contains(names, feature) {
			return feature, nil
		}
	}
	if feature, err := tmuxctl.CurrentWindowFeature(a.Runner); err == nil && feature != "" {
		return feature, nil
	}
	return "", fmt.Errorf("cannot infer the current feature; run from a feature workspace/window or pass a name")
}

func (a *App) confirm(prompt string) bool {
	fmt.Fprintf(a.Errw, "%s [y/N] ", prompt)
	line, err := bufio.NewReader(a.In).ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}
