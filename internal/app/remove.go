package app

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/richardcase/jumux/internal/agentstate"
	"github.com/richardcase/jumux/internal/jj"
	"github.com/richardcase/jumux/internal/tmuxctl"
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
		_, _ = fmt.Fprintf(a.Out, "forgot jj workspace %q\n", name)
	}
	if dirExists {
		if err := os.RemoveAll(wsPath); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(a.Out, "deleted %s\n", wsPath)
	}
	if windowFound {
		if err := agentstate.Remove(a.StateDir, window.ID); err != nil {
			_, _ = fmt.Fprintf(a.Errw, "removing agent state: %v\n", err)
		}
		// Killed last: if it is our own window this ends the process.
		_, _ = fmt.Fprintf(a.Out, "killing tmux window %s (%s)\n", window.Name, window.ID)
		if err := tmuxctl.KillWindow(a.Runner, window.ID); err != nil {
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
