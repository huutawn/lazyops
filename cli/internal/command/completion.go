package command

import (
	"context"
	"fmt"
	"strings"
)

func completionCommand() *Command {
	return &Command{
		Name:    "completion",
		Summary: "Generate shell completion scripts.",
		Usage:   "lazyops completion <bash|zsh|fish>",
		Run:     runCompletion,
	}
}

func runCompletion(ctx context.Context, runtime *Runtime, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("shell type is required. next: run `lazyops completion <bash|zsh|fish>`")
	}

	shell := strings.ToLower(strings.TrimSpace(args[0]))
	switch shell {
	case "bash":
		runtime.Output.Print("%s", bashCompletionScript())
	case "zsh":
		runtime.Output.Print("%s", zshCompletionScript())
	case "fish":
		runtime.Output.Print("%s", fishCompletionScript())
	default:
		return fmt.Errorf("unsupported shell %q. next: use `bash`, `zsh`, or `fish`", shell)
	}

	runtime.Output.Info("to enable completion, run: eval \"$(lazyops completion %s)\"", shell)
	return nil
}

func bashCompletionScript() string {
	return `_lazyops_bash_completion() {
    local cur="${COMP_WORDS[COMP_CWORD]}"
    local prev="${COMP_WORDS[COMP_CWORD-1]}"
    local commands="login logout init link doctor status bindings logs traces tunnel completion help"
    local tunnel_subcommands="db tcp stop list"

    if [[ ${COMP_CWORD} -eq 1 ]]; then
        COMPREPLY=( $(compgen -W "${commands}" -- "${cur}") )
        return 0
    fi

    case "${COMP_WORDS[1]}" in
        tunnel)
            if [[ ${COMP_CWORD} -eq 2 ]]; then
                COMPREPLY=( $(compgen -W "${tunnel_subcommands}" -- "${cur}") )
            fi
            ;;
        logs)
            if [[ "${prev}" == "--level" ]]; then
                COMPREPLY=( $(compgen -W "debug info warn error" -- "${cur}") )
            fi
            ;;
    esac
}
complete -F _lazyops_bash_completion lazyops`
}

func zshCompletionScript() string {
	return `#compdef lazyops

_lazyops() {
    local -a commands
    commands=(
        'login:Authenticate with LazyOps'
        'logout:Revoke or clear the local CLI session'
        'init:Initialize the repository into a valid LazyOps deploy contract'
        'link:Connect the local repo to a project'
        'doctor:Validate the deploy contract and runtime health'
        'status:Show the current deployment status'
        'bindings:List and filter deployment bindings'
        'logs:Inspect service logs'
        'traces:Inspect distributed request flow by correlation id'
        'tunnel:Open an optional debug tunnel'
        'completion:Generate shell completion scripts'
        'help:Show help information'
    )

    _describe 'command' commands

    case "${words[1]}" in
        tunnel)
            local -a tunnel_cmds
            tunnel_cmds=(
                'db:Open a debug database tunnel'
                'tcp:Open a debug TCP tunnel'
                'stop:Stop an active debug tunnel session'
                'list:List active debug tunnel sessions'
            )
            _describe 'tunnel command' tunnel_cmds
            ;;
    esac
}

_lazyops`
}

func fishCompletionScript() string {
	return `# fish completion for lazyops

complete -c lazyops -f -n "__fish_use_subcommand" -a login -d "Authenticate with LazyOps"
complete -c lazyops -f -n "__fish_use_subcommand" -a logout -d "Revoke or clear the local CLI session"
complete -c lazyops -f -n "__fish_use_subcommand" -a init -d "Initialize the repository into a valid LazyOps deploy contract"
complete -c lazyops -f -n "__fish_use_subcommand" -a link -d "Connect the local repo to a project"
complete -c lazyops -f -n "__fish_use_subcommand" -a doctor -d "Validate the deploy contract and runtime health"
complete -c lazyops -f -n "__fish_use_subcommand" -a status -d "Show the current deployment status"
complete -c lazyops -f -n "__fish_use_subcommand" -a bindings -d "List and filter deployment bindings"
complete -c lazyops -f -n "__fish_use_subcommand" -a logs -d "Inspect service logs"
complete -c lazyops -f -n "__fish_use_subcommand" -a traces -d "Inspect distributed request flow by correlation id"
complete -c lazyops -f -n "__fish_use_subcommand" -a tunnel -d "Open an optional debug tunnel"
complete -c lazyops -f -n "__fish_use_subcommand" -a completion -d "Generate shell completion scripts"
complete -c lazyops -f -n "__fish_use_subcommand" -a help -d "Show help information"

complete -c lazyops -f -n "__fish_seen_subcommand_from tunnel" -a db -d "Open a debug database tunnel"
complete -c lazyops -f -n "__fish_seen_subcommand_from tunnel" -a tcp -d "Open a debug TCP tunnel"
complete -c lazyops -f -n "__fish_seen_subcommand_from tunnel" -a stop -d "Stop an active debug tunnel session"
complete -c lazyops -f -n "__fish_seen_subcommand_from tunnel" -a list -d "List active debug tunnel sessions"

complete -c lazyops -f -n "__fish_seen_subcommand_from logs" -l level -d "Log level filter" -a "debug info warn error"`
}
