package app

import (
	"fmt"

	"github.com/richardcase/jumux/internal/config"
	"github.com/richardcase/jumux/internal/jj"
)

// Rebase rebases a feature's workspace onto its base revision. If name is
// empty the current feature is inferred, like Remove. If onto is empty the
// configured base_revision is used (falling back to "@-" the same way Add
// does when the default trunk() fails to resolve); otherwise onto is used
// as-is and must resolve on its own. Conflicts left by the rebase are
// reported, not treated as an error: jj represents them as first-class,
// resolvable state.
func (a *App) Rebase(name, onto string) error {
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
		if name, err = a.inferFeature(ctx, names); err != nil {
			return err
		}
	}
	if err := validFeatureName(name); err != nil {
		return err
	}
	if !contains(names, name) {
		return fmt.Errorf("workspace %q not found", name)
	}

	target := onto
	if target == "" {
		target = ctx.Config.BaseRevision
	}
	wsPath := a.workspacePath(ctx.MainRoot, name)
	rev := name + "@"

	if err := jj.Rebase(a.Runner, wsPath, rev, target); err != nil {
		if onto != "" || target != config.DefaultBaseRevision {
			return fmt.Errorf("rebase %q onto %s: %w", name, target, err)
		}
		if fbErr := jj.Rebase(a.Runner, wsPath, rev, fallbackBaseRevision); fbErr != nil {
			return fmt.Errorf("rebase %q onto %s: %w", name, target, err)
		}
		target = fallbackBaseRevision
		_, _ = fmt.Fprintf(a.Errw, "warning: base_revision %q did not resolve (likely no remote bookmark); fell back to %q. Set base_revision explicitly in .jumux.toml to avoid this.\n",
			config.DefaultBaseRevision, fallbackBaseRevision)
	}

	conflicted, err := jj.HasConflict(a.Runner, wsPath, rev)
	if err != nil {
		return err
	}
	if conflicted {
		_, _ = fmt.Fprintf(a.Out, "rebased %q onto %s with conflicts; resolve them inside the workspace\n", name, target)
		return nil
	}
	_, _ = fmt.Fprintf(a.Out, "rebased %q onto %s\n", name, target)
	return nil
}
