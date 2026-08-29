package sidebar

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func testItems() []Item {
	return []Item{
		{Label: "myrepo/auth", Feature: "auth", Status: "dirty", Agent: "working", SessionID: "$0", WindowID: "@2"},
		{Label: "myrepo/billing", Feature: "billing", Status: "clean", Agent: "done", SessionID: "$0", WindowID: "@3"},
		{Label: "other/fix", Feature: "fix", Status: "unknown", SessionID: "$1", WindowID: "@7", Activity: true},
	}
}

func newTestModel(items []Item, jump Jump) Model {
	return newTestModelWithRemove(items, jump, nil)
}

func newTestModelWithRemove(items []Item, jump Jump, remove Remove) Model {
	m := NewModel(func() ([]Item, error) { return items, nil }, jump, remove, time.Second)
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

func TestDOnEmptyListDoesNothing(t *testing.T) {
	m := newTestModelWithRemove(nil, nil, func(string) error {
		t.Fatal("remove must not be called")
		return nil
	})
	if _, cmd := update(t, m, key("d")); cmd != nil {
		t.Error("expected no Cmd on empty list")
	}
	if m.confirming {
		t.Error("should not enter confirm state on empty list")
	}
}

func TestDEntersConfirmState(t *testing.T) {
	m := newTestModelWithRemove(testItems(), nil, func(string) error {
		t.Fatal("remove must not be called before confirmation")
		return nil
	})
	m, cmd := update(t, m, key("d"))
	if cmd != nil {
		t.Error("d should not have a side effect before confirmation")
	}
	if !m.confirming || m.pendingRemove == nil || m.pendingRemove.Feature != "auth" {
		t.Errorf("expected confirm state for auth, got confirming=%v pendingRemove=%+v", m.confirming, m.pendingRemove)
	}
}

func TestConfirmYCallsRemove(t *testing.T) {
	var got string
	remove := func(feature string) error {
		got = feature
		return nil
	}
	m := newTestModelWithRemove(testItems(), nil, remove)
	m, _ = update(t, m, key("j")) // billing
	m, _ = update(t, m, key("d"))
	m, cmd := update(t, m, key("y"))
	if m.confirming {
		t.Error("confirming should be cleared after y")
	}
	if cmd == nil {
		t.Fatal("y should return a Cmd")
	}
	if msg, ok := cmd().(removeMsg); !ok || msg.err != nil {
		t.Fatalf("cmd result: %+v", msg)
	}
	if got != "billing" {
		t.Errorf("remove called with %q, want billing", got)
	}
}

func TestConfirmNCancelsWithoutCalling(t *testing.T) {
	remove := func(string) error {
		t.Fatal("remove must not be called")
		return nil
	}
	m := newTestModelWithRemove(testItems(), nil, remove)
	m, _ = update(t, m, key("d"))
	m, cmd := update(t, m, key("n"))
	if cmd != nil {
		t.Error("n should not return a Cmd")
	}
	if m.confirming || m.pendingRemove != nil {
		t.Errorf("confirm state should be cleared: confirming=%v pendingRemove=%+v", m.confirming, m.pendingRemove)
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
	// Icons: spinner (auth working), ✓ (billing done + clean), ● (auth
	// dirty), ? (fix unknown), · (fix agent unknown).
	for _, want := range []string{"jumux", "myrepo/auth", "other/fix", "q quit", "d remove",
		spinnerFrames[0], "✓", "●", "?", "·"} {
		if !strings.Contains(out, want) {
			t.Errorf("view missing %q:\n%s", want, out)
		}
	}
	// Status words are replaced by icons.
	for _, unwanted := range []string{"dirty", "clean", "working", "done"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("view should not contain status word %q:\n%s", unwanted, out)
		}
	}
	for _, line := range strings.Split(out, "\n") {
		if w := len([]rune(stripANSI(line))); w > 40 {
			t.Errorf("line wider than 40 (%d): %q", w, line)
		}
	}
}

func TestViewGroupsByRepoWhenMultiple(t *testing.T) {
	items := []Item{
		{Label: "repoA/auth", Repo: "repoA", Feature: "auth", SessionID: "$0", WindowID: "@1"},
		{Label: "repoB/billing", Repo: "repoB", Feature: "billing", SessionID: "$0", WindowID: "@2"},
	}
	m := newTestModel(items, nil)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 40, Height: 20})
	out := m.View()
	idxA := strings.Index(out, "repoA")
	idxB := strings.Index(out, "repoB")
	idxAuth := strings.Index(out, "repoA/auth")
	idxBilling := strings.Index(out, "repoB/billing")
	if idxA < 0 || idxB < 0 || idxAuth < 0 || idxBilling < 0 {
		t.Fatalf("missing expected content:\n%s", out)
	}
	if idxA >= idxAuth || idxAuth >= idxB || idxB >= idxBilling {
		t.Errorf("expected repo headers directly above each group:\n%s", out)
	}
}

func TestViewNoGroupingForSingleRepo(t *testing.T) {
	// testItems() leaves Repo unset on every item, so grouping must stay off
	// regardless of what the labels look like.
	m := newTestModel(testItems(), nil)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 40, Height: 20})
	out := m.View()
	for _, line := range strings.Split(out, "\n") {
		if trimmed := strings.TrimSpace(stripANSI(line)); trimmed == "myrepo" || trimmed == "other" {
			t.Errorf("unexpected standalone group header line %q:\n%s", trimmed, out)
		}
	}
}

func TestViewShowsStaleMarker(t *testing.T) {
	items := testItems()
	items[1].Stale = true // billing
	m := newTestModel(items, nil)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 40, Height: 20})
	out := m.View()
	lines := strings.Split(out, "\n")
	var billing, auth string
	for _, line := range lines {
		if strings.Contains(line, "billing") {
			billing = line
		}
		if strings.Contains(line, "myrepo/auth") {
			auth = line
		}
	}
	if !strings.Contains(billing, "z") {
		t.Errorf("stale billing row missing marker: %q", billing)
	}
	if strings.Contains(auth, "z") {
		t.Errorf("non-stale auth row should not have marker: %q", auth)
	}
}

func TestViewConfirmingShowsPrompt(t *testing.T) {
	m := newTestModel(testItems(), nil)
	m, _ = update(t, m, key("d"))
	out := m.View()
	if !strings.Contains(out, "remove 'myrepo/auth'? y/n") {
		t.Errorf("view missing confirm prompt:\n%s", out)
	}
}

func TestViewIconWidths(t *testing.T) {
	// The row width math assumes every icon is one column wide.
	glyphs := append([]string{"✓", "●", "?", "·", "!", "▸", "z"}, spinnerFrames...)
	for _, g := range glyphs {
		if w := lipgloss.Width(g); w != 1 {
			t.Errorf("glyph %q has width %d, want 1", g, w)
		}
	}
}

func TestSpinner(t *testing.T) {
	m := newTestModel(testItems(), nil)

	// A fetch with a working item schedules the spinner.
	m, cmd := update(t, m, dataMsg{items: testItems()})
	if cmd == nil {
		t.Fatal("dataMsg with a working item should schedule a spinner tick")
	}
	if !m.spinning {
		t.Error("spinning should be set")
	}

	// Ticks advance the frame and change the rendered spinner glyph.
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 40, Height: 20})
	before := m.View()
	m, cmd = update(t, m, spinnerTickMsg{})
	if m.frame != 1 || cmd == nil {
		t.Errorf("tick should advance frame and reschedule; frame=%d cmd=%v", m.frame, cmd)
	}
	if after := m.View(); after == before {
		t.Error("spinner tick should change the view")
	}

	// With nothing working the spinner stops and no new tick is scheduled.
	idle := []Item{{Label: "myrepo/auth", Status: "clean", Agent: "done"}}
	m, _ = update(t, m, dataMsg{items: idle})
	m, cmd = update(t, m, spinnerTickMsg{})
	if m.spinning || cmd != nil {
		t.Errorf("spinner should stop when nothing is working; spinning=%v cmd=%v", m.spinning, cmd)
	}
}

func TestNoSpinnerWhenNothingWorking(t *testing.T) {
	items := []Item{{Label: "myrepo/auth", Status: "clean", Agent: "done"}}
	m := newTestModel(items, nil)
	if _, cmd := update(t, m, dataMsg{items: items}); cmd != nil {
		t.Error("dataMsg without working items should not schedule a spinner tick")
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
