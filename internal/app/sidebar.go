package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/richardcase/agentmux/internal/config"
	"github.com/richardcase/agentmux/internal/jj"
	"github.com/richardcase/agentmux/internal/run"
	"github.com/richardcase/agentmux/internal/sidebar"
	"github.com/richardcase/agentmux/internal/tmuxctl"
)

// runProgram runs the bubbletea program; a var so tests can stub it.
var runProgram = func(m tea.Model) (tea.Model, error) {
	return tea.NewProgram(m, tea.WithAltScreen()).Run()
}

// Sidebar toggles the agent sidebar: if any sidebar panes exist anywhere,
// close them all; otherwise open one on the left edge of every window of
// every session.
func (a *App) Sidebar() error {
	if err := a.requireTmux(); err != nil {
		return err
	}
	cfg, err := a.sidebarConfig()
	if err != nil {
		return err
	}
	panes, err := tmuxctl.ListAllPanes(a.Runner)
	if err != nil {
		return err
	}
	if closed := a.closeSidebarPanes(panes, ""); closed > 0 {
		_, _ = fmt.Fprintf(a.Out, "sidebar closed (%d panes)\n", closed)
		return nil
	}

	windows, err := tmuxctl.ListAllWindows(a.Runner)
	if err != nil {
		return err
	}
	exe, err := a.executablePath()
	if err != nil {
		return err
	}
	cwd, err := a.Getwd()
	if err != nil {
		return err
	}
	opened := 0
	for _, w := range windows {
		if err := openSidebarPane(a.Runner, w.ID, cwd, cfg.SidebarWidthCols(), exe); err != nil {
			return fmt.Errorf("opening sidebar in window %s: %w", w.ID, err)
		}
		opened++
	}
	_, _ = fmt.Fprintf(a.Out, "sidebar opened in %d windows\n", opened)
	return nil
}

// SidebarRun runs the TUI inside a sidebar pane. It is spawned by Sidebar
// via split-window and is not part of the public CLI surface.
func (a *App) SidebarRun() error {
	if err := a.requireTmux(); err != nil {
		return err
	}
	cfg, err := a.sidebarConfig()
	if err != nil {
		return err
	}
	paneID := a.Getenv("TMUX_PANE")

	fetch := func() ([]sidebar.Item, error) {
		// Skip all jj work while this pane's window is not the session's
		// current window; only visible sidebars poll.
		if paneID != "" {
			if active, err := tmuxctl.PaneWindowActive(a.Runner, paneID); err == nil && !active {
				return nil, sidebar.ErrSkip
			}
		}
		windows, err := tmuxctl.ListAllWindows(a.Runner)
		if err != nil {
			return nil, err
		}
		rows := featureStatuses(a.Runner, windows)
		items := make([]sidebar.Item, 0, len(rows))
		for _, row := range rows {
			label := row.Feature
			if row.Repo != "" {
				label = row.Repo + "/" + row.Feature
			}
			items = append(items, sidebar.Item{
				Label:     label,
				Status:    row.Status,
				Activity:  row.Activity,
				SessionID: row.SessionID,
				WindowID:  row.WindowID,
			})
		}
		return items, nil
	}
	jump := func(sessionID, windowID string) error {
		return tmuxctl.SwitchToWindow(a.Runner, sessionID, windowID)
	}

	final, err := runProgram(sidebar.NewModel(fetch, jump, cfg.SidebarRefreshInterval()))
	if err != nil {
		return err
	}
	if m, ok := final.(sidebar.Model); ok && m.Quitting {
		// q means global toggle-off: close every sidebar pane, our own last
		// (killing it terminates this process, so it must be the final act).
		panes, err := tmuxctl.ListAllPanes(a.Runner)
		if err != nil {
			return err
		}
		a.closeSidebarPanes(panes, paneID)
		if paneID != "" {
			_ = tmuxctl.KillPane(a.Runner, paneID)
		}
	}
	return nil
}

// closeSidebarPanes kills every sidebar pane except skipID and returns how
// many sidebar panes were found (including a skipped one).
func (a *App) closeSidebarPanes(panes []tmuxctl.Pane, skipID string) int {
	found := 0
	for _, p := range panes {
		if !p.Sidebar {
			continue
		}
		found++
		if p.ID == skipID {
			continue
		}
		if err := tmuxctl.KillPane(a.Runner, p.ID); err != nil {
			_, _ = fmt.Fprintf(a.Errw, "closing pane %s: %v\n", p.ID, err)
		}
	}
	return found
}

// ensureSidebarPane adds a sidebar pane to a freshly created window when the
// sidebar is currently active. Failures are reported but never fatal.
func (a *App) ensureSidebarPane(cfg config.Config, windowID string) {
	panes, err := tmuxctl.ListAllPanes(a.Runner)
	if err != nil {
		return
	}
	active := false
	for _, p := range panes {
		if p.Sidebar {
			active = true
			break
		}
	}
	if !active {
		return
	}
	exe, err := a.executablePath()
	if err != nil {
		_, _ = fmt.Fprintf(a.Errw, "sidebar: %v\n", err)
		return
	}
	cwd, err := a.Getwd()
	if err != nil {
		_, _ = fmt.Fprintf(a.Errw, "sidebar: %v\n", err)
		return
	}
	if err := openSidebarPane(a.Runner, windowID, cwd, cfg.SidebarWidthCols(), exe); err != nil {
		_, _ = fmt.Fprintf(a.Errw, "sidebar: %v\n", err)
	}
}

// openSidebarPane splits a tagged sidebar pane into the window.
func openSidebarPane(r run.Runner, windowID, dir string, width int, exe string) error {
	paneID, err := tmuxctl.SplitWindowLeft(r, windowID, dir, width, exe, "sidebar", "run")
	if err != nil {
		return err
	}
	return tmuxctl.SetPaneOption(r, paneID, tmuxctl.SidebarOption, "1")
}

// sidebarConfig loads config like repoContext but tolerates running outside
// a jj repo — the sidebar is global, so a repo is optional.
func (a *App) sidebarConfig() (config.Config, error) {
	if cwd, err := a.Getwd(); err == nil {
		if wsRoot, err := jj.Root(a.Runner, cwd); err == nil {
			if mainRoot, err := jj.MainRoot(wsRoot); err == nil {
				return config.Load(a.GlobalConfig, mainRoot)
			}
		}
	}
	return config.Load(a.GlobalConfig, "")
}

func (a *App) executablePath() (string, error) {
	if a.Executable == nil {
		return "", fmt.Errorf("no executable resolver configured")
	}
	return a.Executable()
}
