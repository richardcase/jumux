package tmuxctl

import (
	"strings"
	"testing"

	"github.com/richardcase/jumux/internal/run"
)

func TestListAllPanes(t *testing.T) {
	fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		return "%1\t@1\t\n%2\t@1\t1\n%3", nil
	}}
	panes, err := ListAllPanes(fr)
	if err != nil {
		t.Fatal(err)
	}
	if len(panes) != 3 {
		t.Fatalf("got %d panes", len(panes))
	}
	if panes[0].Sidebar || !panes[1].Sidebar || panes[2].Sidebar {
		t.Errorf("sidebar flags wrong: %+v", panes)
	}
	if panes[1].WindowID != "@1" || panes[2].WindowID != "" {
		t.Errorf("window IDs wrong: %+v", panes)
	}
	want := "tmux list-panes -a -F #{pane_id}\t#{window_id}\t#{@jumux-sidebar}"
	if got := fr.Calls[0].String(); got != want {
		t.Errorf("command:\n got %q\nwant %q", got, want)
	}
}

func TestListAllWindows(t *testing.T) {
	fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		return "$0\tmain\t@1\tam-auth\tauth\t/repos/myrepo-auth\t1\t1\n" +
			"$1\tother\t@5\tzsh\t\t/home/me\t0\t0\n" +
			"$1\tother\t@6", nil
	}}
	windows, err := ListAllWindows(fr)
	if err != nil {
		t.Fatal(err)
	}
	if len(windows) != 3 {
		t.Fatalf("got %d windows", len(windows))
	}
	w := windows[0]
	if w.SessionID != "$0" || w.SessionName != "main" || w.ID != "@1" ||
		w.Name != "am-auth" || w.Feature != "auth" || w.Path != "/repos/myrepo-auth" || !w.Activity || !w.Dead {
		t.Errorf("first window wrong: %+v", w)
	}
	if windows[1].Feature != "" || windows[1].Activity || windows[1].Dead {
		t.Errorf("second window wrong: %+v", windows[1])
	}
	if windows[2].ID != "@6" || windows[2].Name != "" {
		t.Errorf("ragged line wrong: %+v", windows[2])
	}
}

func TestSplitWindowLeft(t *testing.T) {
	fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
		return "%9", nil
	}}
	id, err := SplitWindowLeft(fr, "@3", "/repo", 32, "/bin/jumux", "sidebar", "run")
	if err != nil {
		t.Fatal(err)
	}
	if id != "%9" {
		t.Errorf("pane id: %q", id)
	}
	want := "tmux split-window -h -b -d -l 32 -t @3 -c /repo -P -F #{pane_id} /bin/jumux sidebar run"
	if got := fr.Calls[0].String(); got != want {
		t.Errorf("command:\n got %q\nwant %q", got, want)
	}
}

func TestPaneCommands(t *testing.T) {
	fr := &run.FakeRunner{}
	if err := SetPaneOption(fr, "%2", SidebarOption, "1"); err != nil {
		t.Fatal(err)
	}
	if err := KillPane(fr, "%2"); err != nil {
		t.Fatal(err)
	}
	if err := SwitchToWindow(fr, "$1", "@4"); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(fr.CommandLines())
	want := "tmux set-option -p -t %2 @jumux-sidebar 1\n" +
		"tmux kill-pane -t %2\n" +
		"tmux select-window -t @4\n" +
		"tmux switch-client -t $1"
	if got != want {
		t.Errorf("commands:\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPaneWindowInfo(t *testing.T) {
	tests := []struct {
		name        string
		out         string
		wantWindow  string
		wantFeature string
	}{
		{name: "with feature", out: "@5\tauth\n", wantWindow: "@5", wantFeature: "auth"},
		{name: "no feature", out: "@5\t\n", wantWindow: "@5", wantFeature: ""},
		{name: "ragged", out: "@5", wantWindow: "@5", wantFeature: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
				return tt.out, nil
			}}
			windowID, feature, err := PaneWindowInfo(fr, "%7")
			if err != nil {
				t.Fatal(err)
			}
			if windowID != tt.wantWindow || feature != tt.wantFeature {
				t.Errorf("got (%q, %q), want (%q, %q)", windowID, feature, tt.wantWindow, tt.wantFeature)
			}
			wantCmd := "tmux display-message -p -t %7 #{window_id}\t#{@jumux-feature}"
			if gotCmd := fr.Calls[0].String(); gotCmd != wantCmd {
				t.Errorf("command %q", gotCmd)
			}
		})
	}
}

func TestPaneWindowActive(t *testing.T) {
	for out, want := range map[string]bool{"1": true, "0": false} {
		fr := &run.FakeRunner{Handler: func(dir, name string, args ...string) (string, error) {
			return out, nil
		}}
		got, err := PaneWindowActive(fr, "%7")
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("output %q: got %v", out, got)
		}
		wantCmd := "tmux display-message -p -t %7 #{window_active}"
		if gotCmd := fr.Calls[0].String(); gotCmd != wantCmd {
			t.Errorf("command %q", gotCmd)
		}
	}
}
