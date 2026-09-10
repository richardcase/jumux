package app

import (
	"fmt"
	"os"

	"github.com/richardcase/jumux/internal/filesync"
	"github.com/richardcase/jumux/internal/jj"
)

// Sync re-applies the configured files.copy/files.symlink operations to an
// existing workspace. If name is empty the current feature is inferred, the
// same way Remove does. It's idempotent: destinations that already exist
// from a previous Add or Sync are left untouched (with a warning).
func (a *App) Sync(name string) error {
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

	wsPath := a.workspacePath(ctx.MainRoot, name)
	if _, err := os.Stat(wsPath); err != nil {
		return fmt.Errorf("workspace %q not found at %s", name, wsPath)
	}

	filesync.Apply(ctx.MainRoot, wsPath, ctx.Config.Files.Copy, ctx.Config.Files.Symlink, a.Errw)
	_, _ = fmt.Fprintf(a.Out, "synced files into %s\n", wsPath)
	return nil
}
