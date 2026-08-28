// jumux eases working with multiple coding agents by pairing jujutsu
// workspaces with tmux windows.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/richardcase/jumux/internal/app"
)

const usage = `Usage: jumux <command> [args]

Commands:
  add [-a AGENT] <feature>
                       create a jj workspace + tmux window and start the
                       agent (-a/--agent overrides the configured agent
                       command for this feature)
  remove [-f] [name]   remove a feature's workspace, directory, and window
                       (name defaults to the current feature)
  list                 show feature workspaces and their tmux windows
  sidebar              toggle a live agent sidebar pane on every tmux window
  hook <status>        record agent status (working|waiting|done); wired to
                       Claude Code hooks
  doctor               check the environment (jj, tmux, base_revision,
                       Claude Code hooks) and print a pass/fail checklist

Config: ~/.config/jumux/config.toml, overridden per-repo by .jumux.toml
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	a := app.New()
	var err error

	switch os.Args[1] {
	case "add":
		fs := flag.NewFlagSet("add", flag.ExitOnError)
		agent := fs.String("agent", "", "override the configured agent command for this feature")
		fs.StringVar(agent, "a", *agent, "shorthand for -agent")
		_ = fs.Parse(os.Args[2:])
		if fs.NArg() != 1 {
			fmt.Fprintln(os.Stderr, "usage: jumux add [-a AGENT|--agent AGENT] <feature>")
			os.Exit(2)
		}
		err = a.Add(fs.Arg(0), *agent)
	case "remove":
		fs := flag.NewFlagSet("remove", flag.ExitOnError)
		force := fs.Bool("force", false, "skip the dirty working-copy confirmation")
		fs.BoolVar(force, "f", *force, "shorthand for -force")
		_ = fs.Parse(os.Args[2:])
		if fs.NArg() > 1 {
			fmt.Fprintln(os.Stderr, "usage: jumux remove [-f] [name]")
			os.Exit(2)
		}
		err = a.Remove(fs.Arg(0), *force)
	case "list":
		err = a.List()
	case "sidebar":
		fs := flag.NewFlagSet("sidebar", flag.ExitOnError)
		_ = fs.Parse(os.Args[2:])
		switch {
		case fs.NArg() == 0:
			err = a.Sidebar()
		case fs.NArg() == 1 && fs.Arg(0) == "run":
			// Internal mode: runs the TUI inside a spawned sidebar pane.
			err = a.SidebarRun()
		default:
			fmt.Fprintln(os.Stderr, "usage: jumux sidebar")
			os.Exit(2)
		}
	case "hook":
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "usage: jumux hook <working|waiting|done>")
			os.Exit(2)
		}
		err = a.Hook(os.Args[2])
	case "doctor":
		err = a.Doctor()
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "jumux: unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "jumux: %v\n", err)
		os.Exit(1)
	}
}
