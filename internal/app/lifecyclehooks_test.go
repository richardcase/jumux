package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/richardcase/jumux/internal/run"
)

func TestHookEnvIncludesExpectedVars(t *testing.T) {
	env := hookEnv("post_create", "billing", "/ws/billing", "/repo", "billing-window")
	want := map[string]string{
		"JUMUX_EVENT":          "post_create",
		"JUMUX_FEATURE":        "billing",
		"JUMUX_WORKSPACE_PATH": "/ws/billing",
		"JUMUX_REPO_ROOT":      "/repo",
		"JUMUX_WINDOW_NAME":    "billing-window",
	}
	for k, v := range want {
		if !contains(env, k+"="+v) {
			t.Errorf("env missing %s=%s; got %v", k, v, env)
		}
	}
}

func TestHookEnvOmitsEmptyWindowName(t *testing.T) {
	env := hookEnv("pre_remove", "billing", "/ws/billing", "/repo", "")
	for _, e := range env {
		if strings.HasPrefix(e, "JUMUX_WINDOW_NAME=") {
			t.Errorf("expected no JUMUX_WINDOW_NAME entry, got %v", env)
		}
	}
}

func TestRunHooksNoCommandsIsNoOp(t *testing.T) {
	r := &run.FakeHookRunner{}
	if err := runHooks(r, "/ws", nil, 0, nil); err != nil {
		t.Fatal(err)
	}
	if len(r.Calls) != 0 {
		t.Errorf("expected no calls, got %d", len(r.Calls))
	}
}

func TestRunHooksRunsInOrder(t *testing.T) {
	r := &run.FakeHookRunner{}
	err := runHooks(r, "/ws", []string{"echo one", "echo two"}, 5*time.Second, []string{"A=1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Calls) != 2 || r.Calls[0].Command != "echo one" || r.Calls[1].Command != "echo two" {
		t.Errorf("unexpected calls: %+v", r.Calls)
	}
	if r.Calls[0].Dir != "/ws" || r.Calls[0].Timeout != 5*time.Second || r.Calls[0].Env[0] != "A=1" {
		t.Errorf("unexpected call fields: %+v", r.Calls[0])
	}
}

func TestRunHooksStopsAtFirstFailure(t *testing.T) {
	boom := errors.New("boom")
	calls := 0
	r := &run.FakeHookRunner{Handler: func(dir, command string, env []string, timeout time.Duration) error {
		calls++
		if command == "echo one" {
			return boom
		}
		return nil
	}}
	err := runHooks(r, "/ws", []string{"echo one", "echo two"}, 0, nil)
	if err == nil || !errors.Is(err, boom) {
		t.Fatalf("expected wrapped boom, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected the second command to be skipped, ran %d calls", calls)
	}
}
