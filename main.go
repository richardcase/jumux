// agentmux eases working with multiple coding agents by pairing jujutsu
// workspaces with tmux windows.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/richardcase/agentmux/internal/app"
)

const usage = `Usage: agentmux <command> [args]

Commands:
  add <feature>        create a jj workspace + tmux window and start the agent
  remove [-f] [name]   remove a feature's workspace, directory, and window
                       (name defaults to the current feature)
  list                 show feature workspaces and their tmux windows

Config: ~/.config/agentmux/config.toml, overridden per-repo by .agentmux.toml
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
		_ = fs.Parse(os.Args[2:])
		if fs.NArg() != 1 {
			fmt.Fprintln(os.Stderr, "usage: agentmux add <feature>")
			os.Exit(2)
		}
		err = a.Add(fs.Arg(0))
	case "remove":
		fs := flag.NewFlagSet("remove", flag.ExitOnError)
		force := fs.Bool("force", false, "skip the dirty working-copy confirmation")
		fs.BoolVar(force, "f", *force, "shorthand for -force")
		_ = fs.Parse(os.Args[2:])
		if fs.NArg() > 1 {
			fmt.Fprintln(os.Stderr, "usage: agentmux remove [-f] [name]")
			os.Exit(2)
		}
		err = a.Remove(fs.Arg(0), *force)
	case "list":
		err = a.List()
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "agentmux: unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "agentmux: %v\n", err)
		os.Exit(1)
	}
}
