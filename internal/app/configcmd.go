package app

import (
	"fmt"
	"os"

	"github.com/richardcase/jumux/internal/config"
)

// ConfigShow prints the effective merged config (global config <- repo
// config <- defaults) along with, for each key, which file it was
// sourced from. It works outside a jj repo too, in which case only the
// global file (and built-in defaults) are considered.
func (a *App) ConfigShow() error {
	mainRoot := ""
	if ctx, err := a.repoContext(); err == nil {
		mainRoot = ctx.MainRoot
	}

	_, _ = fmt.Fprintf(a.Out, "global: %s\n", describePath(a.GlobalConfig))
	if mainRoot != "" {
		_, _ = fmt.Fprintf(a.Out, "repo:   %s\n", describePath(mainRoot+"/"+config.RepoFileName))
	} else {
		_, _ = fmt.Fprintln(a.Out, "repo:   (not in a jj repo; skipped)")
	}
	_, _ = fmt.Fprintln(a.Out)

	_, fields, err := config.Show(a.GlobalConfig, mainRoot)
	if err != nil {
		return err
	}
	config.FormatFields(a.Out, fields)
	return nil
}

// describePath returns path annotated with whether it currently exists, or
// "(unset)" if path is empty.
func describePath(path string) string {
	if path == "" {
		return "(unset)"
	}
	if _, err := os.Stat(path); err != nil {
		return path + " (absent)"
	}
	return path
}
