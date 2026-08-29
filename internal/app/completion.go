package app

import (
	"fmt"

	"github.com/richardcase/jumux/internal/jj"
)

// bashCompletion is a self-contained bash completion script (no dependency
// on the bash-completion package). It completes subcommands, hook's status
// argument, completion's shell argument, and feature names for the
// commands that take one, shelling out to the hidden
// "jumux __complete-features" command for the dynamic feature list.
const bashCompletion = `_jumux_complete() {
    local cur cmd
    cur="${COMP_WORDS[COMP_CWORD]}"
    cmd="${COMP_WORDS[1]}"

    if [[ $COMP_CWORD -eq 1 ]]; then
        COMPREPLY=( $(compgen -W "add remove restart rename attach pr mr list sidebar hook doctor config completion help" -- "$cur") )
        return 0
    fi

    case "$cmd" in
        hook)
            COMPREPLY=( $(compgen -W "working waiting done blocked error" -- "$cur") )
            ;;
        completion)
            COMPREPLY=( $(compgen -W "bash zsh fish" -- "$cur") )
            ;;
        config)
            COMPREPLY=( $(compgen -W "show" -- "$cur") )
            ;;
        remove|restart|attach|rename|pr|mr)
            COMPREPLY=( $(compgen -W "$(jumux __complete-features 2>/dev/null)" -- "$cur") )
            ;;
        *)
            COMPREPLY=()
            ;;
    esac
    return 0
}
complete -F _jumux_complete jumux
`

// zshCompletion is a native zsh completion function.
const zshCompletion = `#compdef jumux

_jumux() {
    local -a commands
    commands=(add remove restart rename attach pr mr list sidebar hook doctor config completion help)

    if (( CURRENT == 2 )); then
        _describe 'command' commands
        return
    fi

    local cmd=${words[2]}
    case $cmd in
        hook)
            local -a statuses
            statuses=(working waiting done blocked error)
            _describe 'status' statuses
            ;;
        completion)
            local -a shells
            shells=(bash zsh fish)
            _describe 'shell' shells
            ;;
        config)
            local -a subcommands
            subcommands=(show)
            _describe 'subcommand' subcommands
            ;;
        remove|restart|attach|rename|pr|mr)
            local -a features
            features=(${(f)"$(jumux __complete-features 2>/dev/null)"})
            _describe 'feature' features
            ;;
    esac
}

_jumux "$@"
`

// fishCompletion is a fish completion script.
const fishCompletion = `function __jumux_features
    jumux __complete-features 2>/dev/null
end

complete -c jumux -f
complete -c jumux -n "__fish_use_subcommand" -a "add remove restart rename attach pr mr list sidebar hook doctor config completion help"
complete -c jumux -n "__fish_seen_subcommand_from remove restart attach rename pr mr" -a "(__jumux_features)"
complete -c jumux -n "__fish_seen_subcommand_from hook" -a "working waiting done blocked error"
complete -c jumux -n "__fish_seen_subcommand_from completion" -a "bash zsh fish"
complete -c jumux -n "__fish_seen_subcommand_from config" -a "show"
`

// Completion writes a shell completion script for shell ("bash", "zsh", or
// "fish") to a.Out.
func (a *App) Completion(shell string) error {
	var script string
	switch shell {
	case "bash":
		script = bashCompletion
	case "zsh":
		script = zshCompletion
	case "fish":
		script = fishCompletion
	default:
		return fmt.Errorf("unsupported shell %q (want bash, zsh, or fish)", shell)
	}
	_, err := fmt.Fprint(a.Out, script)
	return err
}

// CompletionFeatures prints the repo's non-default jj workspace names, one
// per line, for completion scripts to consume via the hidden
// "jumux __complete-features" command. Any failure to resolve a repo or
// list workspaces (e.g. running outside a jj repo) is swallowed so
// completion degrades to no candidates instead of erroring.
func (a *App) CompletionFeatures() error {
	ctx, err := a.repoContext()
	if err != nil {
		return nil
	}
	names, err := jj.Workspaces(a.Runner, ctx.MainRoot)
	if err != nil {
		return nil
	}
	for _, name := range names {
		if name == "default" {
			continue
		}
		if _, err := fmt.Fprintln(a.Out, name); err != nil {
			return err
		}
	}
	return nil
}
