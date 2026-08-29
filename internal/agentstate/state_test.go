package agentstate

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

var now = time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)

func TestValid(t *testing.T) {
	tests := []struct {
		status Status
		want   bool
	}{
		{Working, true},
		{Waiting, true},
		{Done, true},
		{Blocked, true},
		{Error, true},
		{Status("napping"), false},
		{Status(""), false},
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := Valid(tt.status); got != tt.want {
				t.Errorf("Valid(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestDir(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{
			name: "xdg state home",
			env:  map[string]string{"XDG_STATE_HOME": "/xdg/state"},
			want: "/xdg/state/jumux/status",
		},
		{
			name: "fallback to home",
			env:  map[string]string{"HOME": "/home/u"},
			want: "/home/u/.local/state/jumux/status",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getenv := func(k string) string { return tt.env[k] }
			if got := Dir(getenv); got != tt.want {
				t.Errorf("Dir() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWriteReadAll(t *testing.T) {
	tests := []struct {
		name    string
		entries []Entry
		want    map[string]Status
	}{
		{
			name: "round trip",
			entries: []Entry{
				{WindowID: "@1", PaneID: "%3", Status: Working, UpdatedAt: now},
				{WindowID: "@2", PaneID: "%4", Status: Done, UpdatedAt: now},
			},
			want: map[string]Status{"@1": Working, "@2": Done},
		},
		{
			name: "stale working dropped",
			entries: []Entry{
				{WindowID: "@1", Status: Working, UpdatedAt: now.Add(-WorkingTTL - time.Minute)},
			},
			want: map[string]Status{},
		},
		{
			name: "stale waiting kept",
			entries: []Entry{
				{WindowID: "@1", Status: Waiting, UpdatedAt: now.Add(-24 * time.Hour)},
			},
			want: map[string]Status{"@1": Waiting},
		},
		{
			name: "stale blocked and error kept",
			entries: []Entry{
				{WindowID: "@1", Status: Blocked, UpdatedAt: now.Add(-24 * time.Hour)},
				{WindowID: "@2", Status: Error, UpdatedAt: now.Add(-24 * time.Hour)},
			},
			want: map[string]Status{"@1": Blocked, "@2": Error},
		},
		{
			name: "rewrite replaces",
			entries: []Entry{
				{WindowID: "@1", Status: Working, UpdatedAt: now},
				{WindowID: "@1", Status: Done, UpdatedAt: now},
			},
			want: map[string]Status{"@1": Done},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, e := range tt.entries {
				if err := Write(dir, e); err != nil {
					t.Fatalf("Write: %v", err)
				}
			}
			got := ReadAll(dir, now)
			if len(got) != len(tt.want) {
				t.Fatalf("ReadAll() = %v, want %v", got, tt.want)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("ReadAll()[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestReadAllMissingDir(t *testing.T) {
	got := ReadAll(filepath.Join(t.TempDir(), "nope"), now)
	if len(got) != 0 {
		t.Errorf("ReadAll(missing dir) = %v, want empty", got)
	}
}

func TestReadAllSkipsCorruptFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "w1.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Write(dir, Entry{WindowID: "@2", Status: Waiting, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	got := ReadAll(dir, now)
	if len(got) != 1 || got["@2"] != Waiting {
		t.Errorf("ReadAll() = %v, want only @2:waiting", got)
	}
}

func TestRemove(t *testing.T) {
	dir := t.TempDir()
	if err := Write(dir, Entry{WindowID: "@1", Status: Done, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := Remove(dir, "@1"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if got := ReadAll(dir, now); len(got) != 0 {
		t.Errorf("after Remove, ReadAll() = %v, want empty", got)
	}
	if err := Remove(dir, "@1"); err != nil {
		t.Errorf("Remove(missing) = %v, want nil", err)
	}
}

func TestPrune(t *testing.T) {
	dir := t.TempDir()
	for _, e := range []Entry{
		{WindowID: "@1", Status: Working, UpdatedAt: now},
		{WindowID: "@2", Status: Done, UpdatedAt: now},
	} {
		if err := Write(dir, e); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "w9.json"), []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Prune(dir, map[string]bool{"@1": true}); err != nil {
		t.Fatalf("Prune: %v", err)
	}
	got := ReadAll(dir, now)
	if len(got) != 1 || got["@1"] != Working {
		t.Errorf("after Prune, ReadAll() = %v, want only @1:working", got)
	}
}

func TestLastUpdated(t *testing.T) {
	dir := t.TempDir()
	older := now.Add(-48 * time.Hour)
	for _, e := range []Entry{
		{WindowID: "@1", Status: Working, UpdatedAt: now},
		{WindowID: "@2", Status: Waiting, UpdatedAt: older},
	} {
		if err := Write(dir, e); err != nil {
			t.Fatal(err)
		}
	}
	got := LastUpdated(dir)
	if len(got) != 2 || !got["@1"].Equal(now) || !got["@2"].Equal(older) {
		t.Errorf("LastUpdated() = %v, want @1=%v @2=%v", got, now, older)
	}
}

func TestLastUpdatedIncludesStaleWorking(t *testing.T) {
	// Unlike ReadAll, LastUpdated must not drop a stale "working" entry: it
	// is used for activity/staleness detection, not liveness.
	dir := t.TempDir()
	stale := now.Add(-WorkingTTL - time.Hour)
	if err := Write(dir, Entry{WindowID: "@1", Status: Working, UpdatedAt: stale}); err != nil {
		t.Fatal(err)
	}
	got := LastUpdated(dir)
	if !got["@1"].Equal(stale) {
		t.Errorf("LastUpdated()[@1] = %v, want %v", got["@1"], stale)
	}
}

func TestLastUpdatedMissingDir(t *testing.T) {
	got := LastUpdated(filepath.Join(t.TempDir(), "nope"))
	if len(got) != 0 {
		t.Errorf("LastUpdated(missing dir) = %v, want empty", got)
	}
}

func TestPruneMissingDir(t *testing.T) {
	if err := Prune(filepath.Join(t.TempDir(), "nope"), nil); err != nil {
		t.Errorf("Prune(missing dir) = %v, want nil", err)
	}
}
