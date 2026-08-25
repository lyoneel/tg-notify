package main

import (
	"fmt"
	"strings"
)

// runCompletion prints a shell completion script for the requested
// shell (bash, zsh, or fish) and exits. The scripts complete the CLI's
// known flags; they make no network requests.
func runCompletion(shell string) error {
	switch shell {
	case "bash":
		fmt.Print(bashCompletion)
	case "zsh":
		fmt.Print(zshCompletion)
	case "fish":
		fmt.Print(fishCompletion)
	default:
		return fmt.Errorf("unsupported shell %q: want bash, zsh, or fish", shell)
	}
	return nil
}

// completionFlags lists every flag the CLI accepts.
var completionFlags = []string{
	"-m", "--message",
	"-f", "--file",
	"-u", "--url",
	"-F", "--file-id",
	"-a", "--album",
	"-t", "--type",
	"-c", "--caption",
	"-p", "--parse-mode",
	"-r", "--reply-to",
	"-j", "--json",
	"--no-trim",
	"-w", "--whoami",
	"-T", "--token",
	"-C", "--chat-id",
	"-n", "--no-retry",
	"-R", "--retries",
	"-B", "--base-wait",
	"-d", "--discover-chat-id",
	"-o", "--offset",
	"-S", "--silent",
	"-P", "--proxy",
	"-U", "--base-url",
	"-D", "--dry-run",
	"-A", "--completion",
	"-v", "--version",
	"-h", "--help",
}

// completionFlagWords is the space-joined flag list shared by the bash
// and zsh scripts.
var completionFlagWords = strings.Join(completionFlags, " ")

var bashCompletion = `# bash completion for tg-notify
_tg_notify() {
    local cur prev
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    case "$prev" in
        --type|-t)
            COMPREPLY=( $(compgen -W "photo document audio video voice animation sticker" -- "$cur") )
            return 0
            ;;
        --parse-mode|-p)
            COMPREPLY=( $(compgen -W "MarkdownV2 HTML" -- "$cur") )
            return 0
            ;;
        --completion)
            COMPREPLY=( $(compgen -W "bash zsh fish" -- "$cur") )
            return 0
            ;;
    esac

    COMPREPLY=( $(compgen -W "` + completionFlagWords + `" -- "$cur") )
    return 0
}
complete -F _tg_notify tg-notify
`

var zshCompletion = `#compdef tg-notify
# zsh completion for tg-notify
_tg_notify() {
    local -a flags
    flags=(` + completionFlagWords + `)
    _describe 'flag' flags
}
compdef _tg_notify tg-notify
`

var fishCompletion = fishFlagCompletions()

func fishFlagCompletions() string {
	var b strings.Builder
	for _, f := range completionFlags {
		b.WriteString("complete -c tg-notify -l ")
		b.WriteString(strings.TrimPrefix(f, "-"))
		b.WriteString("\n")
	}
	return b.String()
}
