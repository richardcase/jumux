package app

import (
	"fmt"
	"strings"

	"github.com/richardcase/jumux/internal/forge"
	"github.com/richardcase/jumux/internal/jj"
)

// MR pushes feature's bookmark and opens a GitLab merge request for it.
// If feature is empty, it is inferred from the current workspace/window.
func (a *App) MR(feature string) error {
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

	title, body, err := forge.PreparePush(a.Runner, ctx.WsRoot, feature)
	if err != nil {
		return err
	}

	out, err := a.Runner.Run(ctx.WsRoot, "glab", "mr", "create", "--title", title, "--description", body)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			_, _ = fmt.Fprintf(a.Out, "pushed %s; a merge request already exists\n", feature)
			return nil
		}
		return err
	}
	_, _ = fmt.Fprintln(a.Out, out)
	return nil
}
