package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/richardcase/jumux/internal/agentstate"
	"github.com/richardcase/jumux/internal/run"
	"github.com/richardcase/jumux/internal/tmuxctl"
)

var statusNow = time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)

// statusFixture builds a main repo plus workspace dirs whose .jj/repo
// pointer files resolve back to it, so jj.MainRoot works on real FS.
func statusFixture(t *testing.T, features ...string) (mainRoot string, wsPaths map[string]string) {
	t.Helper()
	tmp := t.TempDir()
	mainRoot = filepath.Join(tmp, "myrepo")
	if err := os.MkdirAll(filepath.Join(mainRoot, ".jj", "repo"), 0o755); err != nil {
		t.Fatal(err)
	}
	pointer := filepath.Join(mainRoot, ".jj", "repo")
	wsPaths = map[string]string{}
	for _, feat := range features {
		ws := filepath.Join(tmp, "myrepo-"+feat)
		if err := os.MkdirAll(filepath.Join(ws, ".jj"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ws, ".jj", "repo"), []byte(pointer), 0o644); err != nil {
			t.Fatal(err)
		}
		wsPaths[feat] = ws
	}
	return mainRoot, wsPaths
}

func TestFeatureStatuses(t *testing.T) {
	_, ws := statusFixture(t, "auth", "billing")
	fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		cmd := name + " " + strings.Join(args, " ")
		switch {
		case strings.HasPrefix(cmd, "jj root"):
			if dir == "/gone" {
				return "", errors.New("no repo")
			}
			return dir, nil
		case strings.Contains(cmd, "auth@"):
			return "dirty", nil
		case strings.HasPrefix(cmd, "jj log"):
			return "clean", nil
		}
		return "", nil
	}}
	windows := []tmuxctl.GlobalWindow{
		{SessionID: "$1", SessionName: "work", ID: "@9", Feature: "billing", Path: ws["billing"]},
		{SessionID: "$0", SessionName: "main", ID: "@2", Feature: "auth", Path: ws["auth"], Activity: true},
		{SessionID: "$0", SessionName: "main", ID: "@1", Name: "zsh", Path: "/home/me"},
		{SessionID: "$1", SessionName: "work", ID: "@5", Feature: "lost", Path: "/gone"},
	}
	agent := map[string]agentstate.Status{"@2": agentstate.Working, "@9": agentstate.Done}
	rows := featureStatuses(fr, windows, agent, nil, statusNow, 0)
	if len(rows) != 3 {
		t.Fatalf("untagged window should be skipped; got %d rows: %+v", len(rows), rows)
	}
	// Sorted by session name then window ID.
	if rows[0].Feature != "auth" || rows[1].Feature != "lost" || rows[2].Feature != "billing" {
		t.Errorf("sort order wrong: %+v", rows)
	}
	auth := rows[0]
	if auth.Repo != "myrepo" || auth.Status != "dirty" || !auth.Activity ||
		auth.SessionID != "$0" || auth.WindowID != "@2" || auth.AgentStatus != "working" {
		t.Errorf("auth row wrong: %+v", auth)
	}
	if rows[2].Status != "clean" || rows[2].Repo != "myrepo" || rows[2].AgentStatus != "done" {
		t.Errorf("billing row wrong: %+v", rows[2])
	}
	if rows[1].Status != "unknown" || rows[1].Repo != "" || rows[1].AgentStatus != "" {
		t.Errorf("unresolvable row should be unknown: %+v", rows[1])
	}
}

func TestFeatureStatusesStale(t *testing.T) {
	_, ws := statusFixture(t, "auth", "billing", "fresh")
	fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		cmd := name + " " + strings.Join(args, " ")
		switch {
		case strings.HasPrefix(cmd, "jj root"):
			return dir, nil
		case strings.Contains(cmd, "committer.timestamp"):
			switch {
			case strings.Contains(cmd, "auth@"):
				// 10 days old: stale on jj activity alone.
				return fmt.Sprintf("%d", statusNow.Add(-240*time.Hour).Unix()), nil
			case strings.Contains(cmd, "fresh@"):
				return fmt.Sprintf("%d", statusNow.Add(-time.Hour).Unix()), nil
			}
			return "", errors.New("no timestamp")
		case strings.HasPrefix(cmd, "jj log"):
			return "clean", nil
		}
		return "", nil
	}}
	windows := []tmuxctl.GlobalWindow{
		{SessionName: "a", ID: "@1", Feature: "auth", Path: ws["auth"]},
		{SessionName: "a", ID: "@2", Feature: "billing", Path: ws["billing"]},
		{SessionName: "a", ID: "@3", Feature: "fresh", Path: ws["fresh"]},
	}
	// billing's jj timestamp is unavailable, but a recent hook update keeps
	// it fresh.
	hookUpdates := map[string]time.Time{"@2": statusNow.Add(-time.Minute)}
	rows := featureStatuses(fr, windows, nil, hookUpdates, statusNow, 168*time.Hour)
	byFeature := map[string]FeatureStatus{}
	for _, r := range rows {
		byFeature[r.Feature] = r
	}
	if !byFeature["auth"].Stale {
		t.Error("auth should be stale (10d old jj activity, no hook update)")
	}
	if byFeature["billing"].Stale {
		t.Error("billing should not be stale (recent hook update)")
	}
	if byFeature["fresh"].Stale {
		t.Error("fresh should not be stale (1h old jj activity)")
	}
}

func TestFeatureStatusesStaleDisabled(t *testing.T) {
	_, ws := statusFixture(t, "auth")
	fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		return "clean", nil
	}}
	windows := []tmuxctl.GlobalWindow{{SessionName: "a", ID: "@1", Feature: "auth", Path: ws["auth"]}}
	rows := featureStatuses(fr, windows, nil, nil, statusNow, 0)
	if rows[0].Stale {
		t.Error("staleAfter <= 0 should disable stale detection")
	}
}

func TestFeatureStatusesCachesRootPerPath(t *testing.T) {
	_, ws := statusFixture(t, "auth")
	rootCalls := 0
	fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "root" {
			rootCalls++
			return dir, nil
		}
		return "clean", nil
	}}
	windows := []tmuxctl.GlobalWindow{
		{SessionName: "a", ID: "@1", Feature: "auth", Path: ws["auth"]},
		{SessionName: "a", ID: "@2", Feature: "auth", Path: ws["auth"]},
	}
	if rows := featureStatuses(fr, windows, nil, nil, statusNow, 0); len(rows) != 2 {
		t.Fatalf("got %d rows", len(rows))
	}
	if rootCalls != 1 {
		t.Errorf("jj root should run once per path, ran %d times", rootCalls)
	}
}
