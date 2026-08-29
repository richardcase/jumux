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
  add [-a AGENT] [-t TEMPLATE] <feature>
                       create a jj workspace + tmux window and start the
                       agent (-a/--agent overrides the configured agent
                       command for this feature; -t/--template applies a
                       named template's base_revision/agent/window options
                       from config)
  remove [-f] [name]   remove a feature's workspace, directory, and window
                       (name defaults to the current feature)
  remove [-f] --all-done
                       remove every feature whose recorded agent status is
                       "done"
  restart [-f] [name]  restart the configured agent in a feature's tmux
                       window (name defaults to the current feature); asks
                       for confirmation unless the pane looks dead or -f is
                       given
  rename <old> <new>   rename a feature's jj workspace, directory, and tmux
                       window in place, without recreating the working copy
  attach <feature>     switch the tmux client to a feature's existing
                       window, without touching jj or workspace state
  pr [feature]         push feature's bookmark and open a GitHub PR
                       (feature defaults to the current feature)
  mr [feature]         push feature's bookmark and open a GitLab MR
                       (feature defaults to the current feature)
  list                 show feature workspaces and their tmux windows
  sidebar              toggle a live agent sidebar pane on every tmux window
  hook <status>        record agent status
                       (working|waiting|done|blocked|error); wired to
                       Claude Code hooks
  doctor               check the environment (jj, tmux, base_revision,
                       Claude Code hooks) and print a pass/fail checklist
  config show          print the effective merged config and which file
                       (global, repo, or default) each value came from

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
		template := fs.String("template", "", "apply a named template's base_revision/agent/window options")
		fs.StringVar(template, "t", *template, "shorthand for -template")
		_ = fs.Parse(os.Args[2:])
		if fs.NArg() != 1 {
			fmt.Fprintln(os.Stderr, "usage: jumux add [-a AGENT|--agent AGENT] [-t TEMPLATE|--template TEMPLATE] <feature>")
			os.Exit(2)
		}
		err = a.Add(fs.Arg(0), *agent, *template)
	case "remove":
		fs := flag.NewFlagSet("remove", flag.ExitOnError)
		force := fs.Bool("force", false, "skip the dirty working-copy confirmation")
		fs.BoolVar(force, "f", *force, "shorthand for -force")
		allDone := fs.Bool("all-done", false, "remove every feature whose recorded agent status is done")
		_ = fs.Parse(os.Args[2:])
		switch {
		case *allDone:
			if fs.NArg() != 0 {
				fmt.Fprintln(os.Stderr, "usage: jumux remove [-f] --all-done")
				os.Exit(2)
			}
			err = a.RemoveAllDone(*force)
		case fs.NArg() > 1:
			fmt.Fprintln(os.Stderr, "usage: jumux remove [-f] [name]")
			os.Exit(2)
		default:
			err = a.Remove(fs.Arg(0), *force)
		}
	case "restart":
		fs := flag.NewFlagSet("restart", flag.ExitOnError)
		force := fs.Bool("force", false, "skip the alive-pane confirmation")
		fs.BoolVar(force, "f", *force, "shorthand for -force")
		_ = fs.Parse(os.Args[2:])
		if fs.NArg() > 1 {
			fmt.Fprintln(os.Stderr, "usage: jumux restart [-f] [name]")
			os.Exit(2)
		}
		err = a.Restart(fs.Arg(0), *force)
	case "rename":
		fs := flag.NewFlagSet("rename", flag.ExitOnError)
		_ = fs.Parse(os.Args[2:])
		if fs.NArg() != 2 {
			fmt.Fprintln(os.Stderr, "usage: jumux rename <old> <new>")
			os.Exit(2)
		}
		err = a.Rename(fs.Arg(0), fs.Arg(1))
	case "attach":
		fs := flag.NewFlagSet("attach", flag.ExitOnError)
		_ = fs.Parse(os.Args[2:])
		if fs.NArg() != 1 {
			fmt.Fprintln(os.Stderr, "usage: jumux attach <feature>")
			os.Exit(2)
		}
		err = a.Attach(fs.Arg(0))
	case "pr":
		fs := flag.NewFlagSet("pr", flag.ExitOnError)
		_ = fs.Parse(os.Args[2:])
		if fs.NArg() > 1 {
			fmt.Fprintln(os.Stderr, "usage: jumux pr [feature]")
			os.Exit(2)
		}
		feature := ""
		if fs.NArg() == 1 {
			feature = fs.Arg(0)
		}
		err = a.PR(feature)
	case "mr":
		fs := flag.NewFlagSet("mr", flag.ExitOnError)
		_ = fs.Parse(os.Args[2:])
		if fs.NArg() > 1 {
			fmt.Fprintln(os.Stderr, "usage: jumux mr [feature]")
			os.Exit(2)
		}
		feature := ""
		if fs.NArg() == 1 {
			feature = fs.Arg(0)
		}
		err = a.MR(feature)
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
			fmt.Fprintln(os.Stderr, "usage: jumux hook <working|waiting|done|blocked|error>")
			os.Exit(2)
		}
		err = a.Hook(os.Args[2])
	case "doctor":
		err = a.Doctor()
	case "config":
		if len(os.Args) != 3 || os.Args[2] != "show" {
			fmt.Fprintln(os.Stderr, "usage: jumux config show")
			os.Exit(2)
		}
		err = a.ConfigShow()
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
