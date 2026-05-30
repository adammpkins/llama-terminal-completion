package cli

import (
	"fmt"
	"strings"

	"github.com/adammpkins/llama-terminal-completion/internal/client"
	"github.com/spf13/cobra"
)

// completeCmd turns the current command-line buffer (a partial command or a
// natural-language description) into a single shell command, printed raw to
// stdout. It is the engine behind the shell widget: no confirmation, no
// execution, no markdown — and it stays silent on error so the widget can leave
// the buffer untouched.
var completeCmd = &cobra.Command{
	Use:   "complete [buffer]",
	Short: "Complete a partial command line into a shell command (for the shell widget)",
	Long: `Generate a single shell command from a partial command line or a natural
language description, printing only the command. Designed to be called from the
shell widget (see 'lt widget'), but usable directly:

  lt complete "find big files modified this week"`,
	Args:          cobra.ArbitraryArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runComplete,
}

func init() {
	rootCmd.AddCommand(completeCmd)
}

func runComplete(cmd *cobra.Command, args []string) error {
	buffer := strings.TrimSpace(strings.Join(args, " "))
	if buffer == "" {
		var err error
		if buffer, err = getQuestion(nil); err != nil || strings.TrimSpace(buffer) == "" {
			return nil // nothing to complete; stay silent
		}
	}

	apiClient := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.Model)

	systemPrompt := `You are a shell command completion engine. The user gives a partial command line or a short description. Output a SINGLE shell command that best completes or fulfills it.

Rules:
- Output ONLY the command — no explanations, no markdown, no code fences, no surrounding quotes.
- Keep the user's intent; if they typed a partial command, complete it sensibly.
- Prefer common, portable POSIX/Unix tooling.`

	// Use the configured token budget (with a floor) so reasoning models have
	// room to finish their thinking and still emit the command in content.
	maxTokens := cfg.MaxTokens
	if maxTokens < 512 {
		maxTokens = 512
	}
	resp, err := apiClient.ChatCompletion([]client.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: buffer},
	}, maxTokens, 0.2)
	if err != nil {
		return nil // silent: the widget keeps the original buffer
	}
	if len(resp.Choices) == 0 {
		return nil
	}
	command := cleanCommand(resp.Choices[0].Message.Content)
	if command == "" {
		return nil
	}
	fmt.Print(command)
	return nil
}

// widgetCmd prints a shell integration script that binds a key to call
// `lt complete` and replace the current command line with the result.
var widgetCmd = &cobra.Command{
	Use:   "widget [bash|zsh|fish]",
	Short: "Print a shell keybinding that completes the command line via AI",
	Long: `Print a shell snippet that binds Ctrl-G to complete the current command line
using 'lt complete'. Add it to your shell startup file:

  # zsh (~/.zshrc)
  source <(lt widget zsh)

  # bash (~/.bashrc)
  source <(lt widget bash)

  # fish (~/.config/fish/config.fish)
  lt widget fish | source

Then type a partial command or description and press Ctrl-G.`,
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"bash", "zsh", "fish"},
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "zsh":
			fmt.Print(zshWidget)
		case "bash":
			fmt.Print(bashWidget)
		case "fish":
			fmt.Print(fishWidget)
		default:
			return fmt.Errorf("unsupported shell %q (use bash, zsh, or fish)", args[0])
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(widgetCmd)
}

const zshWidget = `# LlamaTerm shell widget (zsh) — press Ctrl-G to complete the command line
_lt_complete_widget() {
  local buf="$BUFFER"
  [[ -z "$buf" ]] && return
  local out
  out="$(lt complete -- "$buf" 2>/dev/null)"
  if [[ -n "$out" ]]; then
    BUFFER="$out"
    CURSOR=${#BUFFER}
  fi
  zle redisplay
}
zle -N _lt_complete_widget
bindkey '^G' _lt_complete_widget
`

const bashWidget = `# LlamaTerm shell widget (bash) — press Ctrl-G to complete the command line
_lt_complete() {
  local buf="$READLINE_LINE"
  [[ -z "$buf" ]] && return
  local out
  out="$(lt complete -- "$buf" 2>/dev/null)"
  if [[ -n "$out" ]]; then
    READLINE_LINE="$out"
    READLINE_POINT=${#READLINE_LINE}
  fi
}
bind -x '"\C-g": _lt_complete'
`

const fishWidget = `# LlamaTerm shell widget (fish) — press Ctrl-G to complete the command line
function _lt_complete
  set -l buf (commandline)
  test -z "$buf"; and return
  set -l out (lt complete -- "$buf" 2>/dev/null)
  if test -n "$out"
    commandline -r -- $out
  end
end
bind \cg _lt_complete
`
