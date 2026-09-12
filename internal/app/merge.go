package app

import (
	"fmt"
	"os"

	"github.com/richardcase/jumux/internal/jj"
)

// Merge integrates a feature into the configured base bookmark
// (base_bookmark, default "main") by rebasing the feature onto it and
// moving the base bookmark to the rebased tip, then tears down the feature
// the same way Remove does: forgets the jj workspace, deletes its
// directory, and kills its tmux window. If name is empty the current
// feature is inferred.
//
// If the rebase leaves the feature's tip conflicted, Merge stops before
// touching the base bookmark or cleaning up, leaving the workspace,
// directory, and window intact so the conflict can be resolved manually.
func (a *App) Merge(name string, force bool) error {
	if err := a.requireTmux(); err != nil {
		return err
	}
	ctx, err := a.repoContext()
	if err != nil {
		return err
	}
	target, err := a.resolveTarget(ctx, name, force, "merge")
	if err != nil {
		return err
	}

	base := ctx.Config.BaseBookmark
	if err := jj.Rebase(a.Runner, ctx.MainRoot, target.name, base); err != nil {
		return err
	}
	conflict, err := jj.HasConflict(a.Runner, ctx.MainRoot, target.name+"@")
	if err != nil {
		return err
	}
	if conflict {
		return fmt.Errorf("merging %q onto %q produced conflicts; resolve them in the workspace and run merge again", target.name, base)
	}
	if err := jj.BookmarkSet(a.Runner, ctx.MainRoot, base, target.name+"@"); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(a.Out, "merged %q into %q\n", target.name, base)

	// Get out of the directory we are about to delete.
	if err := os.Chdir(ctx.MainRoot); err != nil {
		return err
	}
	return a.cleanupWorkspace(ctx, target)
}
