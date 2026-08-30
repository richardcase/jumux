package app

import (
	"fmt"
	"os"

	"github.com/richardcase/jumux/internal/jj"
	"github.com/richardcase/jumux/internal/tmuxctl"
)

// Rename renames a feature in place: the jj workspace, its data
// directory, and its tmux window are all renamed from oldName to newName.
// The working-copy commit is untouched and the agent is not restarted.
func (a *App) Rename(oldName, newName string) error {
	if err := a.requireTmux(); err != nil {
		return err
	}
	ctx, err := a.repoContext()
	if err != nil {
		return err
	}
	if oldName == "default" || newName == "default" {
		return fmt.Errorf("refusing to rename the default workspace")
	}
	if err := validFeatureName(oldName); err != nil {
		return err
	}
	if err := validFeatureName(newName); err != nil {
		return err
	}
	if oldName == newName {
		return fmt.Errorf("old and new feature names are the same: %q", oldName)
	}

	names, err := jj.Workspaces(a.Runner, ctx.MainRoot)
	if err != nil {
		return err
	}
	if !contains(names, oldName) {
		return fmt.Errorf("workspace %q not found", oldName)
	}
	if contains(names, newName) {
		return fmt.Errorf("workspace %q already exists", newName)
	}

	oldPath := a.workspacePath(ctx.MainRoot, oldName)
	newPath := a.workspacePath(ctx.MainRoot, newName)
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("directory %s already exists", newPath)
	}

	windows, err := tmuxctl.ListWindows(a.Runner)
	if err != nil {
		return err
	}
	if w, ok := tmuxctl.FindWindow(windows, newName, ctx.Config.WindowPrefix+newName); ok {
		return fmt.Errorf("tmux window %s (%s) already belongs to feature %q", w.Name, w.ID, newName)
	}
	window, windowFound := tmuxctl.FindWindow(windows, oldName, ctx.Config.WindowPrefix+oldName)

	if err := jj.WorkspaceRename(a.Runner, oldPath, newName); err != nil {
		return err
	}

	if _, err := os.Stat(oldPath); err == nil {
		// Get out of the directory we are about to rename.
		if err := os.Chdir(ctx.MainRoot); err != nil {
			return err
		}
		if err := os.Rename(oldPath, newPath); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(a.Out, "renamed workspace directory %s to %s\n", oldPath, newPath)
	}

	if windowFound {
		newWindowName := ctx.Config.WindowPrefix + newName
		if err := tmuxctl.RenameWindow(a.Runner, window.ID, newWindowName, newName); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(a.Out, "renamed tmux window %s to %s (%s)\n", window.Name, newWindowName, window.ID)
	}

	_, _ = fmt.Fprintf(a.Out, "renamed feature %q to %q\n", oldName, newName)
	return nil
}
