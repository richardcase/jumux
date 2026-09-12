package app

import (
	"os"
	"strings"
	"testing"
)

// The default fixture responses already describe two features: "auth" (has
// a tmux window "@2") and none other. These tests script "billing" too, as
// a feature with a workspace directory but no window.

func TestResurrectRecreatesMissingWindowOnly(t *testing.T) {
	f := newFixture(t)
	f.responses["jj workspace list"] = "default: qq 11 (empty)\nauth: kk 22 stuff\nbilling: mm 33 stuff"
	if err := os.MkdirAll(f.wsPath("auth"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(f.wsPath("billing"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := f.app.Resurrect(); err != nil {
		t.Fatal(err)
	}

	f.assertRan(t,
		"tmux new-window -d -P -F #{window_id} -n billing -c "+f.wsPath("billing"),
		"tmux set-option -w -t @7 automatic-rename off",
		"tmux set-option -w -t @7 @jumux-feature billing",
		"tmux send-keys -t @7 -l claude",
		"tmux send-keys -t @7 Enter",
	)
	// "auth" already has a window (@2 in the default fixture) so nothing
	// should be created for it, and jj/select-window should never run.
	f.assertNotRan(t,
		"-n auth",
		"select-window",
		"jj workspace add",
		"jj workspace forget",
	)
	if !strings.Contains(f.out.String(), `restored feature "billing"`) {
		t.Errorf("expected a restored-billing message, got: %s", f.out.String())
	}
}

func TestResurrectSkipsStaleWorkspace(t *testing.T) {
	f := newFixture(t)
	f.responses["jj workspace list"] = "default: qq 11 (empty)\nghost: kk 22 stuff"
	// No directory created for "ghost": its workspace is gone from disk.

	if err := f.app.Resurrect(); err != nil {
		t.Fatal(err)
	}

	f.assertNotRan(t, "tmux new-window")
	if !strings.Contains(f.err.String(), `skipping "ghost": workspace directory missing`) {
		t.Errorf("expected a skip message for ghost, got: %s", f.err.String())
	}
}

func TestResurrectNothingToRestore(t *testing.T) {
	f := newFixture(t)
	// Default fixture: only "auth", which already has window @2.
	if err := os.MkdirAll(f.wsPath("auth"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := f.app.Resurrect(); err != nil {
		t.Fatal(err)
	}

	f.assertNotRan(t, "tmux new-window")
	if !strings.Contains(f.out.String(), "nothing to restore") {
		t.Errorf("expected 'nothing to restore', got: %s", f.out.String())
	}
}

func TestResurrectContinuesAfterOneFailure(t *testing.T) {
	f := newFixture(t)
	f.responses["jj workspace list"] = "default: qq 11 (empty)\nauth: kk 22 stuff\nbilling: mm 33 stuff\ncart: nn 44 stuff"
	for _, name := range []string{"auth", "billing", "cart"} {
		if err := os.MkdirAll(f.wsPath(name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	f.failOn = "tmux send-keys -t @7 -l claude" // simulate the agent-start step failing

	err := f.app.Resurrect()
	if err == nil {
		t.Fatal("expected an error to be returned when a feature fails to restore")
	}

	// Both "billing" and "cart" attempt window creation (auth already has a
	// window); regardless of which one hits the scripted failure, tmux
	// should still have been asked to kill the half-created window and the
	// other feature should still have been processed.
	f.assertRan(t, "tmux kill-window -t @7")
}

