package run

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

var osReadFile = os.ReadFile
var errBoom = errors.New("boom")

func TestExecHookRunnerRunsCommandInDir(t *testing.T) {
	dir := t.TempDir()
	r := ExecHookRunner{}
	if err := r.RunHook(dir, "pwd > out.txt", nil, 0); err != nil {
		t.Fatal(err)
	}
	got, err := readFile(t, dir+"/out.txt")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(got) != dir {
		t.Errorf("command ran in %q, want %q", strings.TrimSpace(got), dir)
	}
}

func TestExecHookRunnerPassesEnv(t *testing.T) {
	dir := t.TempDir()
	r := ExecHookRunner{}
	err := r.RunHook(dir, "echo $JUMUX_FEATURE > out.txt", []string{"JUMUX_FEATURE=billing"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	got, err := readFile(t, dir+"/out.txt")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(got) != "billing" {
		t.Errorf("got %q", got)
	}
}

func TestExecHookRunnerFailingCommandErrors(t *testing.T) {
	r := ExecHookRunner{}
	err := r.RunHook(t.TempDir(), "exit 3", nil, 0)
	if err == nil {
		t.Fatal("expected an error for a nonzero exit")
	}
}

func TestExecHookRunnerTimeout(t *testing.T) {
	r := ExecHookRunner{}
	err := r.RunHook(t.TempDir(), "sleep 5", nil, 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected a timeout-flavored error, got %v", err)
	}
}

func TestFakeHookRunnerRecordsCalls(t *testing.T) {
	f := &FakeHookRunner{}
	if err := f.RunHook("/ws", "echo hi", []string{"A=1"}, 0); err != nil {
		t.Fatal(err)
	}
	if len(f.Calls) != 1 {
		t.Fatalf("got %d calls, want 1", len(f.Calls))
	}
	c := f.Calls[0]
	if c.Dir != "/ws" || c.Command != "echo hi" || len(c.Env) != 1 || c.Env[0] != "A=1" {
		t.Errorf("unexpected call: %+v", c)
	}
}

func TestFakeHookRunnerErr(t *testing.T) {
	f := &FakeHookRunner{Err: errBoom}
	if err := f.RunHook("/ws", "echo hi", nil, 0); err != errBoom {
		t.Errorf("got %v, want errBoom", err)
	}
}

func readFile(t *testing.T, path string) (string, error) {
	t.Helper()
	b, err := osReadFile(path)
	return string(b), err
}
