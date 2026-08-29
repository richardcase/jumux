package app

import (
	"fmt"

	"github.com/richardcase/jumux/internal/forge"
	"github.com/richardcase/jumux/internal/jj"
)

// createOnHost pushes feature's bookmark and opens a change request for it
// on h. If feature is empty, it is inferred from the current
// workspace/window. It backs both PR and MR; everything host-specific
// comes from h.
func (a *App) createOnHost(h forge.Host, feature string) error {
	ctx, err := a.repoContext()
	if err != nil {
		return err
	}

	if feature == "" {
		names, err := jj.Workspaces(a.Runner, ctx.MainRoot)
		if err != nil {
			return err
		}
		feature, err = a.inferFeature(ctx, names)
		if err != nil {
			return err
		}
	}
	if feature == "default" {
		return fmt.Errorf("refusing to open a %s for the default workspace", h.Noun)
	}
	if err := validFeatureName(feature); err != nil {
		return err
	}

	title, body, err := forge.PreparePush(a.Runner, ctx.WsRoot, feature)
	if err != nil {
		return err
	}

	// The host CLI resolves the git remote, so it must run in the main
	// workspace: that is the one colocated with git. A secondary workspace
	// (../<repo>-<feature>) has a .jj directory but no .git.
	out, err := a.Runner.Run(ctx.MainRoot, h.Bin, h.CreateArgv(feature, title, body)...)
	if err != nil {
		if h.AlreadyExists(err, title, body) {
			_, _ = fmt.Fprintf(a.Out, "pushed %s; a %s already exists\n", feature, h.Noun)
			return nil
		}
		return err
	}
	_, _ = fmt.Fprintln(a.Out, out)
	return nil
}
