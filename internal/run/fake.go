package run

import (
	"fmt"
	"strings"
	"time"
)

// Call records one invocation made through a FakeRunner.
type Call struct {
	Dir  string
	Name string
	Args []string
}

func (c Call) String() string {
	return c.Name + " " + strings.Join(c.Args, " ")
}

// FakeRunner is a scripted Runner for tests. Handler decides the response;
// if nil, every call succeeds with empty output.
type FakeRunner struct {
	Calls   []Call
	Handler func(dir, name string, args ...string) (string, error)
}

func (f *FakeRunner) Run(dir, name string, args ...string) (string, error) {
	f.Calls = append(f.Calls, Call{Dir: dir, Name: name, Args: args})
	if f.Handler == nil {
		return "", nil
	}
	return f.Handler(dir, name, args...)
}

// CommandLines renders all recorded calls, one per line, for easy assertions.
func (f *FakeRunner) CommandLines() string {
	var b strings.Builder
	for _, c := range f.Calls {
		fmt.Fprintln(&b, c.String())
	}
	return b.String()
}

// HookCall records one invocation made through a FakeHookRunner.
type HookCall struct {
	Dir     string
	Command string
	Env     []string
	Timeout time.Duration
}

// FakeHookRunner is a scripted HookRunner for tests. If Handler is set it
// decides the response; otherwise every call succeeds unless Err is set.
type FakeHookRunner struct {
	Calls   []HookCall
	Err     error
	Handler func(dir, command string, env []string, timeout time.Duration) error
}

func (f *FakeHookRunner) RunHook(dir, command string, env []string, timeout time.Duration) error {
	f.Calls = append(f.Calls, HookCall{Dir: dir, Command: command, Env: env, Timeout: timeout})
	if f.Handler != nil {
		return f.Handler(dir, command, env, timeout)
	}
	return f.Err
}
