package notify

import (
	"errors"
	"testing"

	"github.com/richardcase/agentmux/internal/run"
)

func TestSend(t *testing.T) {
	tests := []struct {
		name     string
		goos     string
		wantCmd  string // "" means no command should run
		wantArgs []string
	}{
		{
			name:     "darwin uses osascript",
			goos:     "darwin",
			wantCmd:  "osascript",
			wantArgs: []string{"-e", `display notification "hi there" with title "agentmux: auth"`},
		},
		{
			name:     "linux uses notify-send",
			goos:     "linux",
			wantCmd:  "notify-send",
			wantArgs: []string{"agentmux: auth", "hi there"},
		},
		{
			name: "other platforms are a no-op",
			goos: "windows",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fr := &run.FakeRunner{}
			err := send(fr, tt.goos, "agentmux: auth", "hi there")
			if err != nil {
				t.Fatalf("send() error = %v", err)
			}
			if tt.wantCmd == "" {
				if len(fr.Calls) != 0 {
					t.Fatalf("expected no command, ran:\n%s", fr.CommandLines())
				}
				return
			}
			if len(fr.Calls) != 1 {
				t.Fatalf("expected 1 command, ran:\n%s", fr.CommandLines())
			}
			got := fr.Calls[0]
			if got.Name != tt.wantCmd {
				t.Errorf("command = %q, want %q", got.Name, tt.wantCmd)
			}
			if len(got.Args) != len(tt.wantArgs) {
				t.Fatalf("args = %v, want %v", got.Args, tt.wantArgs)
			}
			for i, a := range tt.wantArgs {
				if got.Args[i] != a {
					t.Errorf("args[%d] = %q, want %q", i, got.Args[i], a)
				}
			}
		})
	}
}

func TestSendPropagatesRunnerError(t *testing.T) {
	wantErr := errors.New("boom")
	fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		return "", wantErr
	}}
	if err := send(fr, "darwin", "title", "msg"); !errors.Is(err, wantErr) {
		t.Fatalf("send() error = %v, want %v", err, wantErr)
	}
}
