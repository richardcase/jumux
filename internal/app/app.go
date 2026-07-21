// Package app orchestrates the agentmux commands over the jj and tmux
// wrappers.
package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/richardcase/agentmux/internal/config"
	"github.com/richardcase/agentmux/internal/jj"
	"github.com/richardcase/agentmux/internal/run"
)

var featureNameRe = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9._-]*$`)

// App carries the injectable environment for the commands.
type App struct {
	Runner run.Runner
	Out    io.Writer
	Errw   io.Writer
	In     io.Reader
	Getwd  func() (string, error)
	Getenv func(string) string
	// GlobalConfig is the path to the global config file ("" to skip).
	GlobalConfig string
}

// New returns an App wired to the real environment.
func New() *App {
	return &App{
		Runner:       run.ExecRunner{},
		Out:          os.Stdout,
		Errw:         os.Stderr,
		In:           os.Stdin,
		Getwd:        os.Getwd,
		Getenv:       os.Getenv,
		GlobalConfig: config.GlobalPath(),
	}
}

// repoContext is everything the commands need about the surrounding repo.
type repoContext struct {
	MainRoot string
	WsRoot   string // root of the workspace cwd is in
	Config   config.Config
}

func (a *App) requireTmux() error {
	if a.Getenv("TMUX") == "" {
		return fmt.Errorf("agentmux must be run inside a tmux session")
	}
	return nil
}

func (a *App) repoContext() (*repoContext, error) {
	cwd, err := a.Getwd()
	if err != nil {
		return nil, err
	}
	wsRoot, err := jj.Root(a.Runner, cwd)
	if err != nil {
		return nil, fmt.Errorf("not inside a jj repo: %w", err)
	}
	mainRoot, err := jj.MainRoot(wsRoot)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(a.GlobalConfig, mainRoot)
	if err != nil {
		return nil, err
	}
	return &repoContext{MainRoot: mainRoot, WsRoot: wsRoot, Config: cfg}, nil
}

// workspacePath returns the conventional sibling directory for a feature:
// ../<mainRepoDir>-<feature>.
func workspacePath(mainRoot, feature string) string {
	return filepath.Join(filepath.Dir(mainRoot), filepath.Base(mainRoot)+"-"+feature)
}

func validFeatureName(name string) error {
	if !featureNameRe.MatchString(name) {
		return fmt.Errorf("invalid feature name %q (use letters, digits, '.', '_', '-'; no leading '.')", name)
	}
	return nil
}

func contains(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}
