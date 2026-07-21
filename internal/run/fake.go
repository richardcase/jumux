package run

import (
	"fmt"
	"strings"
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
