package app

import (
	"fmt"
	"strings"

	"github.com/richardcase/jumux/internal/forge"
	"github.com/richardcase/jumux/internal/jj"
)

// PR pushes feature's bookmark and opens a GitHub pull request for it.
// If feature is empty, it is inferred from the current workspace/window.
func (a *App) PR(feature string) error {
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

	out, err := a.Runner.Run(ctx.WsRoot, "gh", "pr", "create", "--title", title, "--body", body)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			_, _ = fmt.Fprintf(a.Out, "pushed %s; a pull request already exists\n", feature)
			return nil
		}
		return err
	}
	_, _ = fmt.Fprintln(a.Out, out)
	return nil
}
