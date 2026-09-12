package app

import (
	"fmt"
	"os"

	"github.com/richardcase/jumux/internal/jj"
)

// Path prints the filesystem path of a feature workspace to a.Out.
// If name is empty the current feature is inferred.
func (a *App) Path(name string) error {
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
		_, err = fmt.Fprintln(a.Out, ctx.MainRoot)
		return err
	}
	if err := validFeatureName(name); err != nil {
		return err
	}
	if !contains(names, name) {
		return fmt.Errorf("workspace %q not found", name)
	}

	wsPath := a.workspacePath(ctx.MainRoot, name)
	if _, err := os.Stat(wsPath); err != nil {
		return fmt.Errorf("workspace %q not found at %s", name, wsPath)
	}

	_, err = fmt.Fprintln(a.Out, wsPath)
	return err
}
