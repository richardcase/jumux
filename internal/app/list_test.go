package app

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/richardcase/jumux/internal/agentstate"
)

func TestListShowsLiveAgentStatus(t *testing.T) {
	f := newFixture(t)
	if err := os.MkdirAll(f.wsPath("auth"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.app.StateDir = t.TempDir()
	if err := agentstate.Write(f.app.StateDir, agentstate.Entry{WindowID: "@2", Status: agentstate.Working, UpdatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := f.app.List(false); err != nil {
		t.Fatal(err)
	}
	out := f.out.String()
	if !strings.Contains(out, "working") {
		t.Errorf("expected working status in output: %s", out)
	}
}

func TestListFallsBackWhenNoAgentEntry(t *testing.T) {
	f := newFixture(t)
	if err := os.MkdirAll(f.wsPath("auth"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.app.StateDir = t.TempDir()
	if err := f.app.List(false); err != nil {
		t.Fatal(err)
	}
	out := f.out.String()
	line := ""
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "auth") {
			line = l
		}
	}
	if line == "" || !strings.Contains(line, "-") {
		t.Errorf("expected '-' agent column with no recorded status: %q", line)
	}
}

func TestListDeadPaneTakesPrecedenceOverAgentStatus(t *testing.T) {
	f := newFixture(t)
	if err := os.MkdirAll(f.wsPath("auth"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.responses["tmux list-windows"] = "@2\tauth\tauth\t1"
	f.app.StateDir = t.TempDir()
	if err := agentstate.Write(f.app.StateDir, agentstate.Entry{WindowID: "@2", Status: agentstate.Working, UpdatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := f.app.List(false); err != nil {
		t.Fatal(err)
	}
	out := f.out.String()
	if !strings.Contains(out, "dead") || strings.Contains(out, "working") {
		t.Errorf("expected dead-agent hint, not live status, for a dead pane: %s", out)
	}
}

func TestListJSONOutput(t *testing.T) {
	f := newFixture(t)
	if err := os.MkdirAll(f.wsPath("auth"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.app.StateDir = t.TempDir()
	if err := agentstate.Write(f.app.StateDir, agentstate.Entry{WindowID: "@2", Status: agentstate.Blocked, UpdatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := f.app.List(true); err != nil {
		t.Fatal(err)
	}
	var rows []listRow
	if err := json.Unmarshal(f.out.Bytes(), &rows); err != nil {
		t.Fatalf("output did not parse as JSON: %v\n%s", err, f.out.String())
	}
	var auth *listRow
	for i := range rows {
		if rows[i].Feature == "auth" {
			auth = &rows[i]
		}
	}
	if auth == nil {
		t.Fatalf("expected an auth row: %+v", rows)
	}
	if auth.Agent != "blocked" {
		t.Errorf("expected raw status %q, got %q", "blocked", auth.Agent)
	}
	if auth.Status != "clean" {
		t.Errorf("expected status clean, got %q", auth.Status)
	}
}

func TestListJSONEmpty(t *testing.T) {
	f := newFixture(t)
	f.responses["jj workspace list"] = "default: qq 11 (empty)"
	if err := f.app.List(true); err != nil {
		t.Fatal(err)
	}
	var rows []listRow
	if err := json.Unmarshal(f.out.Bytes(), &rows); err != nil {
		t.Fatalf("output did not parse as JSON: %v\n%s", err, f.out.String())
	}
	if len(rows) != 0 {
		t.Errorf("expected no rows for an empty workspace list, got %+v", rows)
	}
}
