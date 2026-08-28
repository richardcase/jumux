package app

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/richardcase/jumux/internal/agentstate"
	"github.com/richardcase/jumux/internal/run"
)

func TestHook(t *testing.T) {
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		status   string
		tmuxPane string
		display  string // response to tmux display-message
		wantErr  bool
		want     map[string]agentstate.Status
	}{
		{
			name:     "records status for feature window",
			status:   "working",
			tmuxPane: "%3",
			display:  "@5\tauth",
			want:     map[string]agentstate.Status{"@5": agentstate.Working},
		},
		{
			name:     "records blocked status",
			status:   "blocked",
			tmuxPane: "%3",
			display:  "@5\tauth",
			want:     map[string]agentstate.Status{"@5": agentstate.Blocked},
		},
		{
			name:     "records error status",
			status:   "error",
			tmuxPane: "%3",
			display:  "@5\tauth",
			want:     map[string]agentstate.Status{"@5": agentstate.Error},
		},
		{
			name:    "invalid status errors",
			status:  "napping",
			wantErr: true,
		},
		{
			name:   "no TMUX_PANE is a silent no-op",
			status: "done",
			want:   map[string]agentstate.Status{},
		},
		{
			name:     "non-feature window is a silent no-op",
			status:   "waiting",
			tmuxPane: "%3",
			display:  "@5\t",
			want:     map[string]agentstate.Status{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
				return tt.display, nil
			}}
			a := &App{
				Runner:   fr,
				Errw:     &bytes.Buffer{},
				StateDir: dir,
				Now:      func() time.Time { return now },
				Getwd:    func() (string, error) { return "", errors.New("no repo") },
				Getenv: func(k string) string {
					if k == "TMUX_PANE" {
						return tt.tmuxPane
					}
					return ""
				},
			}
			err := a.Hook(tt.status)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Hook(%q) error = %v, wantErr %v", tt.status, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			got := agentstate.ReadAll(dir, now)
			if len(got) != len(tt.want) {
				t.Fatalf("state = %v, want %v", got, tt.want)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("state[%q] = %q, want %q", k, got[k], v)
				}
			}
			if tt.tmuxPane != "" {
				wantCmd := "tmux display-message -p -t " + tt.tmuxPane
				if !strings.Contains(fr.CommandLines(), wantCmd) {
					t.Errorf("expected command containing %q; ran:\n%s", wantCmd, fr.CommandLines())
				}
			}
		})
	}
}

// newNotifyApp wires an App whose repoContext() resolves successfully
// against a throwaway jj workspace, so maybeNotify can load config.
func newNotifyApp(t *testing.T, notifyToml string) (*App, *run.FakeRunner) {
	t.Helper()
	wsRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wsRoot, ".jj", "repo"), 0o755); err != nil {
		t.Fatal(err)
	}
	global := filepath.Join(t.TempDir(), "config.toml")
	if notifyToml != "" {
		if err := os.WriteFile(global, []byte(notifyToml), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		if name == "jj" {
			return wsRoot, nil
		}
		return "@5\tauth", nil // tmux display-message
	}}
	a := &App{
		Runner:       fr,
		Errw:         &bytes.Buffer{},
		StateDir:     t.TempDir(),
		GlobalConfig: global,
		Now:          func() time.Time { return time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC) },
		Getwd:        func() (string, error) { return wsRoot, nil },
		Getenv:       func(k string) string { return map[string]string{"TMUX_PANE": "%3"}[k] },
	}
	return a, fr
}

func TestHookNotify(t *testing.T) {
	notifierSupported := runtime.GOOS == "darwin" || runtime.GOOS == "linux"

	tests := []struct {
		name             string
		notifyToml       string
		seedStatus       agentstate.Status // pre-existing status for the window, if any
		status           string
		wantConfigLoaded bool // maybeNotify reaches repoContext (jj root runs)
		wantNotify       bool
	}{
		{
			name:             "notifies on transition to waiting",
			status:           "waiting",
			wantConfigLoaded: true,
			wantNotify:       true,
		},
		{
			name:             "notifies on transition to done",
			status:           "done",
			wantConfigLoaded: true,
			wantNotify:       true,
		},
		{
			name:             "notifies on transition to blocked",
			status:           "blocked",
			wantConfigLoaded: true,
			wantNotify:       true,
		},
		{
			name:             "notifies on transition to error",
			status:           "error",
			wantConfigLoaded: true,
			wantNotify:       true,
		},
		{
			name:   "does not notify for working",
			status: "working",
		},
		{
			name:       "does not notify when status is unchanged",
			seedStatus: agentstate.Waiting,
			status:     "waiting",
		},
		{
			name:             "does not notify when config disables it",
			notifyToml:       "notify = false\n",
			status:           "waiting",
			wantConfigLoaded: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, fr := newNotifyApp(t, tt.notifyToml)
			if tt.seedStatus != "" {
				if err := agentstate.Write(a.StateDir, agentstate.Entry{
					WindowID: "@5", PaneID: "%3", Status: tt.seedStatus, UpdatedAt: a.now(),
				}); err != nil {
					t.Fatal(err)
				}
			}
			if err := a.Hook(tt.status); err != nil {
				t.Fatalf("Hook(%q) error = %v", tt.status, err)
			}
			want := 1 // tmux display-message
			if tt.wantConfigLoaded {
				want++ // jj root
			}
			if tt.wantNotify && notifierSupported {
				want++ // osascript / notify-send
			}
			if len(fr.Calls) != want {
				t.Errorf("commands run = %d, want %d; ran:\n%s", len(fr.Calls), want, fr.CommandLines())
			}
		})
	}
}
