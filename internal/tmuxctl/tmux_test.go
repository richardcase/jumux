package tmuxctl

import (
	"errors"
	"testing"

	"github.com/richardcase/jumux/internal/run"
)

func TestServerRunning(t *testing.T) {
	fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		return "main: 1 windows", nil
	}}
	if err := ServerRunning(fr); err != nil {
		t.Errorf("got %v, want nil", err)
	}

	frErr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		return "", errors.New("no server running on /tmp/tmux-501/default")
	}}
	if err := ServerRunning(frErr); err == nil {
		t.Error("got nil error, want non-nil when the tmux server isn't running")
	}
}

func TestListAndFindWindow(t *testing.T) {
	fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		return "@1\tzsh\t\n@2\trenamed-by-agent\tauth\n@3\tbilling\t", nil
	}}
	windows, err := ListWindows(fr)
	if err != nil {
		t.Fatal(err)
	}
	if len(windows) != 3 {
		t.Fatalf("got %d windows", len(windows))
	}
	// Feature tag wins over name, surviving a rename.
	if w, ok := FindWindow(windows, "auth", "auth"); !ok || w.ID != "@2" {
		t.Errorf("feature-tag lookup: got %+v, %v", w, ok)
	}
	// Fallback to exact window name when no tag matches.
	if w, ok := FindWindow(windows, "billing", "billing"); !ok || w.ID != "@3" {
		t.Errorf("name fallback: got %+v, %v", w, ok)
	}
	if _, ok := FindWindow(windows, "nope", "nope"); ok {
		t.Error("unexpected match")
	}
}
