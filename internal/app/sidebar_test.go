package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/richardcase/agentmux/internal/sidebar"
)

// sidebarFixture extends the base fixture with global list-panes /
// list-windows responses and an executable stub.
func sidebarFixture(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	f.app.Executable = func() (string, error) { return "/bin/agentmux", nil }
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
		"tmux split-window -h -b -d -l 32 -t @1 -c "+f.mainRoot+" -P -F #{pane_id} /bin/agentmux sidebar run",
		"tmux split-window -h -b -d -l 32 -t @2",
		"tmux set-option -p -t  @agentmux-sidebar 1",
	)
	f.assertNotRan(t, "kill-pane")
	if !strings.Contains(f.out.String(), "sidebar opened in 2 windows") {
		t.Errorf("output: %s", f.out.String())
	}
}

func TestSidebarToggleOnUsesConfiguredWidth(t *testing.T) {
	f := sidebarFixture(t)
	cfg := filepath.Join(f.mainRoot, ".agentmux.toml")
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

func TestAddSplitsSidebarPaneWhenActive(t *testing.T) {
	f := newFixture(t)
	f.app.Executable = func() (string, error) { return "/bin/agentmux", nil }
	f.responses["tmux list-panes"] = "%5\t@1\t1"
	if err := f.app.Add("billing"); err != nil {
		t.Fatal(err)
	}
	f.assertRan(t, "tmux split-window -h -b -d -l 32 -t @7")
}

func TestAddSkipsSidebarPaneWhenInactive(t *testing.T) {
	f := newFixture(t)
	f.responses["tmux list-panes"] = "%1\t@1\t"
	if err := f.app.Add("billing"); err != nil {
		t.Fatal(err)
	}
	f.assertNotRan(t, "split-window")
}

func TestAddSidebarSplitFailureIsNotFatal(t *testing.T) {
	f := newFixture(t)
	f.app.Executable = func() (string, error) { return "/bin/agentmux", nil }
	f.responses["tmux list-panes"] = "%5\t@1\t1"
	f.failOn = "tmux split-window"
	if err := f.app.Add("billing"); err != nil {
		t.Fatalf("add must succeed despite sidebar failure: %v", err)
	}
	if !strings.Contains(f.err.String(), "sidebar:") {
		t.Errorf("expected warning, got: %s", f.err.String())
	}
}
