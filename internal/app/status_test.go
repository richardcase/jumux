package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/richardcase/jumux/internal/agentstate"
	"github.com/richardcase/jumux/internal/run"
	"github.com/richardcase/jumux/internal/tmuxctl"
)

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
	rows := featureStatuses(fr, windows, agent)
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
	if rows := featureStatuses(fr, windows, nil); len(rows) != 2 {
		t.Fatalf("got %d rows", len(rows))
	}
	if rootCalls != 1 {
		t.Errorf("jj root should run once per path, ran %d times", rootCalls)
	}
}
