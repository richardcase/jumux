package sidebar

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func testItems() []Item {
	return []Item{
		{Label: "myrepo/auth", Status: "dirty", SessionID: "$0", WindowID: "@2"},
		{Label: "myrepo/billing", Status: "clean", SessionID: "$0", WindowID: "@3"},
		{Label: "other/fix", Status: "unknown", SessionID: "$1", WindowID: "@7", Activity: true},
	}
}

func newTestModel(items []Item, jump Jump) Model {
	m := NewModel(func() ([]Item, error) { return items, nil }, jump, time.Second)
	m.items = items
	return m
}

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func update(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(msg)
	nm, ok := next.(Model)
	if !ok {
		t.Fatalf("Update returned %T", next)
	}
	return nm, cmd
}

func TestCursorMovement(t *testing.T) {
	m := newTestModel(testItems(), nil)
	m, _ = update(t, m, key("j"))
	m, _ = update(t, m, key("down"))
	if m.cursor != 2 {
		t.Errorf("cursor = %d, want 2", m.cursor)
	}
	m, _ = update(t, m, key("j")) // clamped at bottom
	if m.cursor != 2 {
		t.Errorf("cursor should clamp at 2, got %d", m.cursor)
	}
	m, _ = update(t, m, key("k"))
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}
	m, _ = update(t, m, key("g"))
	if m.cursor != 0 {
		t.Errorf("g should go first, got %d", m.cursor)
	}
	m, _ = update(t, m, key("k")) // clamped at top
	if m.cursor != 0 {
		t.Errorf("cursor should clamp at 0, got %d", m.cursor)
	}
	m, _ = update(t, m, key("G"))
	if m.cursor != 2 {
		t.Errorf("G should go last, got %d", m.cursor)
	}
}

func TestRefreshPreservesCursorByLabel(t *testing.T) {
	m := newTestModel(testItems(), nil)
	m, _ = update(t, m, key("j")) // on myrepo/billing
	reordered := []Item{testItems()[2], testItems()[1], testItems()[0]}
	m, _ = update(t, m, dataMsg{items: reordered})
	if m.cursor != 1 || m.items[m.cursor].Label != "myrepo/billing" {
		t.Errorf("cursor should follow label, got %d (%+v)", m.cursor, m.items)
	}
	// Shrinking list clamps the cursor.
	m, _ = update(t, m, key("G"))
	m, _ = update(t, m, dataMsg{items: testItems()[:1]})
	if m.cursor != 0 {
		t.Errorf("cursor should clamp after shrink, got %d", m.cursor)
	}
}

func TestSkipKeepsItems(t *testing.T) {
	m := newTestModel(testItems(), nil)
	m, _ = update(t, m, dataMsg{err: ErrSkip})
	if len(m.items) != 3 || m.err != nil {
		t.Errorf("skip should keep items and not surface an error: %+v", m)
	}
	failure := errors.New("boom")
	m, _ = update(t, m, dataMsg{err: failure})
	if len(m.items) != 3 || m.err == nil {
		t.Errorf("fetch failure should keep items but show error: %+v", m)
	}
}

func TestEnterJumps(t *testing.T) {
	var gotSession, gotWindow string
	jump := func(sessionID, windowID string) error {
		gotSession, gotWindow = sessionID, windowID
		return nil
	}
	m := newTestModel(testItems(), jump)
	m, _ = update(t, m, key("G"))
	_, cmd := update(t, m, key("enter"))
	if cmd == nil {
		t.Fatal("enter should return a Cmd")
	}
	if msg, ok := cmd().(jumpMsg); !ok || msg.err != nil {
		t.Fatalf("cmd result: %+v", msg)
	}
	if gotSession != "$1" || gotWindow != "@7" {
		t.Errorf("jump target: %s %s", gotSession, gotWindow)
	}
}

func TestEnterOnEmptyListDoesNothing(t *testing.T) {
	m := newTestModel(nil, func(string, string) error {
		t.Fatal("jump must not be called")
		return nil
	})
	if _, cmd := update(t, m, key("enter")); cmd != nil {
		t.Error("expected no Cmd on empty list")
	}
}

func TestQuit(t *testing.T) {
	for _, k := range []string{"q", "ctrl+c"} {
		m := newTestModel(testItems(), nil)
		m, cmd := update(t, m, key(k))
		if !m.Quitting {
			t.Errorf("%s should set Quitting", k)
		}
		if cmd == nil {
			t.Fatalf("%s should return tea.Quit", k)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("%s cmd is not tea.Quit", k)
		}
	}
}

func TestViewContents(t *testing.T) {
	m := newTestModel(testItems(), nil)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 40, Height: 20})
	out := m.View()
	for _, want := range []string{"agentmux", "myrepo/auth", "dirty", "other/fix", "q quit"} {
		if !strings.Contains(out, want) {
			t.Errorf("view missing %q:\n%s", want, out)
		}
	}
	for _, line := range strings.Split(out, "\n") {
		if w := len([]rune(stripANSI(line))); w > 40 {
			t.Errorf("line wider than 40 (%d): %q", w, line)
		}
	}
}

func TestViewEmpty(t *testing.T) {
	m := newTestModel(nil, nil)
	if out := m.View(); !strings.Contains(out, "no agents") {
		t.Errorf("empty view: %s", out)
	}
}

// stripANSI removes CSI escape sequences for width assertions.
func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		switch {
		case inEsc:
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
		case r == '\x1b':
			inEsc = true
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
