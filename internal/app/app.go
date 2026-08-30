// Package app orchestrates the jumux commands over the jj and tmux
// wrappers.
package app

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/richardcase/jumux/internal/agentstate"
	"github.com/richardcase/jumux/internal/config"
	"github.com/richardcase/jumux/internal/jj"
	"github.com/richardcase/jumux/internal/run"
	"github.com/richardcase/jumux/internal/tmuxctl"
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
	// Executable resolves the path to the running jumux binary, used to
	// spawn sidebar panes.
	Executable func() (string, error)
	// GlobalConfig is the path to the global config file ("" to skip).
	GlobalConfig string
	// StateDir holds per-window agent status files written by `jumux hook`.
	StateDir string
	// ClaudeSettings is the path to the Claude Code settings file where Add
	// offers to install the status hooks ("" to skip the offer).
	ClaudeSettings string
	Now            func() time.Time
	// HTTPClient is used to send webhook notifications (notify_webhook).
	HTTPClient *http.Client
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
		Executable:   os.Executable,
		GlobalConfig: config.GlobalPath(),
		StateDir:     agentstate.Dir(os.Getenv),
		ClaudeSettings: func() string {
			home, err := os.UserHomeDir()
			if err != nil {
				return ""
			}
			return filepath.Join(home, ".claude", "settings.json")
		}(),
		Now:        time.Now,
		HTTPClient: http.DefaultClient,
	}
}

// repoContext is everything the commands need about the surrounding repo.
type repoContext struct {
	MainRoot string
	WsRoot   string // root of the workspace cwd is in
	Config   config.Config
}

// now returns the current time, honoring a test-injected Now.
func (a *App) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

func (a *App) requireTmux() error {
	if a.Getenv("TMUX") == "" {
		return fmt.Errorf("jumux must be run inside a tmux session")
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

// baseDataDir resolves $XDG_DATA_HOME, falling back to ~/.local/share.
func baseDataDir(getenv func(string) string) string {
	base := getenv("XDG_DATA_HOME")
	if base == "" {
		base = filepath.Join(getenv("HOME"), ".local", "share")
	}
	return base
}

// workspacePath returns the workspace directory for a feature:
// $XDG_DATA_HOME/jumux/workspaces/<repo>/<feature>, falling back to
// ~/.local/share/jumux/workspaces/<repo>/<feature>.
func (a *App) workspacePath(mainRoot, feature string) string {
	return filepath.Join(baseDataDir(a.Getenv), "jumux", "workspaces", filepath.Base(mainRoot), feature)
}

// inferFeature determines the current feature: if cwd is inside a
// non-default workspace whose directory name matches a known feature, use
// it; otherwise fall back to the current window's @jumux-feature tag.
func (a *App) inferFeature(ctx *repoContext, names []string) (string, error) {
	if ctx.WsRoot != ctx.MainRoot {
		if feature := filepath.Base(ctx.WsRoot); contains(names, feature) {
			return feature, nil
		}
	}
	if feature, err := tmuxctl.CurrentWindowFeature(a.Runner); err == nil && feature != "" {
		return feature, nil
	}
	return "", fmt.Errorf("cannot infer the current feature; run from a feature workspace/window or pass a name")
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
