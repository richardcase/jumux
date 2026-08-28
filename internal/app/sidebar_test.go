package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/richardcase/jumux/internal/agentstate"
	"github.com/richardcase/jumux/internal/sidebar"
)

// sidebarFixture extends the base fixture with global list-panes /
// list-windows responses and an executable stub.
func sidebarFixture(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	f.app.Executable = func() (string, error) { return "/bin/jumux", nil }
	// No sidebar panes yet; two windows across two sessions.
	f.responses["tmux list-panes"] = "%1\t@1\t\n%2\t@2\t"
	f.responses["tmux list-windows"] = "$0\tmain\t@1\tzsh\t\t/home/me\t0\n" +
		"$0\tmain\t@2\tauth\tauth\t/repos/myrepo-auth\t0"
	return f
}

func TestSidebarToggleOn(t *testing.T) {
	f := sidebarFixture(t)
	if err := f.app.Sidebar(); err != nil {
		t.Fatal(err)
	}
	f.assertRan(t,
		"tmux split-window -h -b -d -l 32 -t @1 -c "+f.mainRoot+" -P -F #{pane_id} /bin/jumux sidebar run",
		"tmux split-window -h -b -d -l 32 -t @2",
		"tmux set-option -p -t  @jumux-sidebar 1",
	)
	f.assertNotRan(t, "kill-pane")
	if !strings.Contains(f.out.String(), "sidebar opened in 2 windows") {
		t.Errorf("output: %s", f.out.String())
	}
}

func TestSidebarToggleOnUsesConfiguredWidth(t *testing.T) {
	f := sidebarFixture(t)
	cfg := filepath.Join(f.mainRoot, ".jumux.toml")
	if err := os.WriteFile(cfg, []byte("sidebar_width = 45\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := f.app.Sidebar(); err != nil {
		t.Fatal(err)
	}
	f.assertRan(t, "tmux split-window -h -b -d -l 45 -t @1")
}

func TestSidebarToggleOff(t *testing.T) {
	f := sidebarFixture(t)
	f.responses["tmux list-panes"] = "%1\t@1\t\n%5\t@1\t1\n%6\t@2\t1"
	if err := f.app.Sidebar(); err != nil {
		t.Fatal(err)
	}
	f.assertRan(t, "tmux kill-pane -t %5", "tmux kill-pane -t %6")
	f.assertNotRan(t, "split-window", "kill-pane -t %1")
	if !strings.Contains(f.out.String(), "sidebar closed (2 panes)") {
		t.Errorf("output: %s", f.out.String())
	}
}

func TestSidebarRequiresTmux(t *testing.T) {
	f := sidebarFixture(t)
	f.app.Getenv = func(string) string { return "" }
	if err := f.app.Sidebar(); err == nil || !strings.Contains(err.Error(), "tmux") {
		t.Fatalf("got %v", err)
	}
}

func TestSidebarWorksOutsideRepo(t *testing.T) {
	f := sidebarFixture(t)
	f.failOn = "jj root"
	if err := f.app.Sidebar(); err != nil {
		t.Fatalf("sidebar should not need a jj repo: %v", err)
	}
	f.assertRan(t, "tmux split-window")
}

func TestSidebarRunQuitClosesAllPanesOwnLast(t *testing.T) {
	f := sidebarFixture(t)
	f.responses["tmux list-panes"] = "%1\t@1\t\n%5\t@1\t1\n%9\t@2\t1"
	base := f.app.Getenv
	f.app.Getenv = func(k string) string {
		if k == "TMUX_PANE" {
			return "%9"
		}
		return base(k)
	}
	orig := runProgram
	defer func() { runProgram = orig }()
	runProgram = func(m tea.Model) (tea.Model, error) {
		sm := m.(sidebar.Model)
		sm.Quitting = true
		return sm, nil
	}
	if err := f.app.SidebarRun(); err != nil {
		t.Fatal(err)
	}
	lines := f.runner.CommandLines()
	first := strings.Index(lines, "kill-pane -t %5")
	own := strings.Index(lines, "kill-pane -t %9")
	if first == -1 || own == -1 || own < first {
		t.Errorf("own pane must be killed last; ran:\n%s", lines)
	}
	if strings.Contains(lines, "kill-pane -t %1") {
		t.Errorf("non-sidebar pane killed:\n%s", lines)
	}
}

func TestSidebarRunNoQuitLeavesPanes(t *testing.T) {
	f := sidebarFixture(t)
	orig := runProgram
	defer func() { runProgram = orig }()
	runProgram = func(m tea.Model) (tea.Model, error) { return m, nil }
	if err := f.app.SidebarRun(); err != nil {
		t.Fatal(err)
	}
	f.assertNotRan(t, "kill-pane")
}

func TestSidebarRunFetchAgentStatusAndPrune(t *testing.T) {
	f := sidebarFixture(t)
	f.app.StateDir = t.TempDir()
	now := time.Now()
	// @2 is the live auth feature window; @9 no longer exists.
	for _, e := range []agentstate.Entry{
		{WindowID: "@2", Status: agentstate.Working, UpdatedAt: now},
		{WindowID: "@9", Status: agentstate.Done, UpdatedAt: now},
	} {
		if err := agentstate.Write(f.app.StateDir, e); err != nil {
			t.Fatal(err)
		}
	}

	orig := runProgram
	defer func() { runProgram = orig }()
	var view string
	runProgram = func(m tea.Model) (tea.Model, error) {
		// Init batches fetchCmd first, then the refresh tick; run just the
		// fetch so the test doesn't sleep for the tick interval.
		batch, ok := m.Init()().(tea.BatchMsg)
		if !ok || len(batch) == 0 {
			t.Fatal("Init should return a batch")
		}
		next, _ := m.Update(batch[0]())
		view = next.View()
		return next, nil
	}
	if err := f.app.SidebarRun(); err != nil {
		t.Fatal(err)
	}
	// The auth row renders the working spinner from the hook state.
	if !strings.Contains(view, "⠋") {
		t.Errorf("view should show the working spinner:\n%s", view)
	}
	got := agentstate.ReadAll(f.app.StateDir, now)
	if len(got) != 1 || got["@2"] != agentstate.Working {
		t.Errorf("dead-window state should be pruned; got %v", got)
	}
}

func TestSidebarRunDConfirmRemovesFeature(t *testing.T) {
	f := sidebarFixture(t)
	f.app.StateDir = t.TempDir()
	if err := agentstate.Write(f.app.StateDir, agentstate.Entry{WindowID: "@2", Status: agentstate.Done, UpdatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	// ListAllWindows (fetch, "-a") and ListWindows (Remove, session-scoped)
	// share the "tmux list-windows" prefix but need different column shapes.
	base := f.runner.Handler
	f.runner.Handler = func(dir, name string, args ...string) (string, error) {
		if name == "tmux" && len(args) > 0 && args[0] == "list-windows" && !strings.Contains(strings.Join(args, " "), "-a") {
			return "@1\tzsh\t\n@2\tauth\tauth", nil
		}
		return base(dir, name, args...)
	}

	orig := runProgram
	defer func() { runProgram = orig }()
	var sm sidebar.Model
	runProgram = func(m tea.Model) (tea.Model, error) {
		batch, ok := m.Init()().(tea.BatchMsg)
		if !ok || len(batch) == 0 {
			t.Fatal("Init should return a batch")
		}
		next, _ := m.Update(batch[0]())
		next, cmd := next.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
		if cmd != nil {
			t.Fatal("d should not have a side effect before confirmation")
		}
		next, cmd = next.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
		if cmd == nil {
			t.Fatal("y should return a Cmd")
		}
		next, _ = next.Update(cmd())
		sm = next.(sidebar.Model)
		return sm, nil
	}
	if err := f.app.SidebarRun(); err != nil {
		t.Fatal(err)
	}
	f.assertRan(t, "tmux kill-window -t @2")
	if got := agentstate.ReadAll(f.app.StateDir, time.Now()); len(got) != 0 {
		t.Errorf("agent state should be removed for @2, got %v", got)
	}
}

func TestAddSplitsSidebarPaneWhenActive(t *testing.T) {
	f := newFixture(t)
	f.app.Executable = func() (string, error) { return "/bin/jumux", nil }
	f.responses["tmux list-panes"] = "%5\t@1\t1"
	if err := f.app.Add("billing", ""); err != nil {
		t.Fatal(err)
	}
	f.assertRan(t, "tmux split-window -h -b -d -l 32 -t @7")
}

func TestAddSkipsSidebarPaneWhenInactive(t *testing.T) {
	f := newFixture(t)
	f.responses["tmux list-panes"] = "%1\t@1\t"
	if err := f.app.Add("billing", ""); err != nil {
		t.Fatal(err)
	}
	f.assertNotRan(t, "split-window")
}

func TestAddSidebarSplitFailureIsNotFatal(t *testing.T) {
	f := newFixture(t)
	f.app.Executable = func() (string, error) { return "/bin/jumux", nil }
	f.responses["tmux list-panes"] = "%5\t@1\t1"
	f.failOn = "tmux split-window"
	if err := f.app.Add("billing", ""); err != nil {
		t.Fatalf("add must succeed despite sidebar failure: %v", err)
	}
	if !strings.Contains(f.err.String(), "sidebar:") {
		t.Errorf("expected warning, got: %s", f.err.String())
	}
}
