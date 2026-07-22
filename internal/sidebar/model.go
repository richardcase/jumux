// Package sidebar implements the live agent sidebar TUI. It knows nothing
// about jj or tmux: data comes in through the Fetch closure and jumps go
// out through the Jump closure, so the model is testable in isolation.
package sidebar

import (
	"errors"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Item is one row in the sidebar.
type Item struct {
	Label     string // "repo/feature"
	Status    string // "clean" | "dirty" | "unknown"
	Agent     string // "working" | "waiting" | "done" | "" (unknown)
	Activity  bool
	SessionID string
	WindowID  string
}

// Fetch loads the current rows. It may return ErrSkip to indicate the pane
// is not visible and the previous rows should be kept as-is.
type Fetch func() ([]Item, error)

// Jump switches the tmux client to an item's window.
type Jump func(sessionID, windowID string) error

// ErrSkip tells the model a refresh was intentionally skipped.
var ErrSkip = errors.New("refresh skipped")

// Model is the bubbletea model for the sidebar.
type Model struct {
	fetch    Fetch
	jump     Jump
	interval time.Duration

	items  []Item
	cursor int
	width  int
	height int
	err    error // last fetch/jump error, shown in the footer

	frame    int  // current spinner frame for "working" rows
	spinning bool // a spinner tick is already scheduled

	// Quitting is set when the user asked to close the sidebar; the caller
	// tears down all sidebar panes.
	Quitting bool
}

// NewModel returns a sidebar model refreshing via fetch every interval.
func NewModel(fetch Fetch, jump Jump, interval time.Duration) Model {
	return Model{fetch: fetch, jump: jump, interval: interval, width: 32, height: 24}
}

type tickMsg struct{}

type spinnerTickMsg struct{}

// spinnerInterval paces the "working" spinner animation; spinner ticks only
// redraw, they never re-fetch.
const spinnerInterval = 250 * time.Millisecond

func spinnerTickCmd() tea.Cmd {
	return tea.Tick(spinnerInterval, func(time.Time) tea.Msg { return spinnerTickMsg{} })
}

func anyWorking(items []Item) bool {
	for _, it := range items {
		if it.Agent == "working" {
			return true
		}
	}
	return false
}

type dataMsg struct {
	items []Item
	err   error
}

type jumpMsg struct{ err error }

func fetchCmd(fetch Fetch) tea.Cmd {
	return func() tea.Msg {
		items, err := fetch()
		return dataMsg{items: items, err: err}
	}
}

func tickCmd(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(fetchCmd(m.fetch), tickCmd(m.interval))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		return m, tea.Batch(fetchCmd(m.fetch), tickCmd(m.interval))
	case dataMsg:
		if errors.Is(msg.err, ErrSkip) {
			return m, nil
		}
		m.err = msg.err
		if msg.err == nil {
			m.setItems(msg.items)
		}
		if !m.spinning && anyWorking(m.items) {
			m.spinning = true
			return m, spinnerTickCmd()
		}
		return m, nil
	case spinnerTickMsg:
		if !anyWorking(m.items) {
			m.spinning = false
			return m, nil
		}
		m.frame++
		return m, spinnerTickCmd()
	case jumpMsg:
		m.err = msg.err
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "g":
		m.cursor = 0
	case "G":
		if len(m.items) > 0 {
			m.cursor = len(m.items) - 1
		}
	case "enter":
		if m.cursor < len(m.items) {
			item := m.items[m.cursor]
			jump := m.jump
			return m, func() tea.Msg {
				return jumpMsg{err: jump(item.SessionID, item.WindowID)}
			}
		}
	case "q", "ctrl+c":
		m.Quitting = true
		return m, tea.Quit
	}
	return m, nil
}

// setItems replaces the rows, keeping the cursor on the same label when
// possible and clamped in range otherwise.
func (m *Model) setItems(items []Item) {
	var current string
	if m.cursor < len(m.items) {
		current = m.items[m.cursor].Label
	}
	m.items = items
	for i, it := range items {
		if it.Label == current && current != "" {
			m.cursor = i
			return
		}
	}
	if m.cursor >= len(items) {
		m.cursor = max(0, len(items)-1)
	}
}
