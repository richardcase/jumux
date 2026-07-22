package app

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/richardcase/agentmux/internal/agentstate"
	"github.com/richardcase/agentmux/internal/run"
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
