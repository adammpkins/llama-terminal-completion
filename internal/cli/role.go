package cli

import (
	"fmt"
	"strings"

	"github.com/adammpkins/llama-terminal-completion/internal/roles"
	"github.com/spf13/cobra"
)

var roleCmd = &cobra.Command{
	Use:   "role",
	Short: "Manage roles (system-prompt presets)",
	Long: `Roles are named system-prompt presets. Select one for any command with
the global --role/-r flag, e.g. "lt -r code-reviewer ask 'review this' < diff".

Built-in roles (shell, code-reviewer, commit-message) can be overridden by
creating a same-named file under the roles directory.`,
}

var roleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available roles",
	RunE: func(cmd *cobra.Command, args []string) error {
		rs, err := roles.List()
		if err != nil {
			return err
		}
		if len(rs) == 0 {
			fmt.Println("No roles found.")
			return nil
		}
		for _, r := range rs {
			summary := strings.SplitN(r.System, "\n", 2)[0]
			if len(summary) > 70 {
				summary = summary[:67] + "..."
			}
			fmt.Printf("%-16s %s\n", r.Name, summary)
		}
		return nil
	},
}

var roleShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show a role's full definition",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := roles.Load(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Name:        %s\n", r.Name)
		if r.Model != "" {
			fmt.Printf("Model:       %s\n", r.Model)
		}
		if r.Temperature != nil {
			fmt.Printf("Temperature: %g\n", *r.Temperature)
		}
		if len(r.Tools) > 0 {
			fmt.Printf("Tools:       %s\n", strings.Join(r.Tools, ", "))
		}
		fmt.Printf("System:\n%s\n", r.System)
		return nil
	},
}

var roleAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Create a role from a system prompt",
	Long: `Create a role. Provide the system prompt as an argument or via stdin.

Examples:
  lt role add pirate "You answer all questions as a salty pirate."
  echo "You are a SQL expert." | lt role add sql`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		var system string
		if len(args) == 2 {
			system = args[1]
		} else {
			s, err := getQuestion(nil) // reads stdin
			if err != nil {
				return err
			}
			system = s
		}
		if strings.TrimSpace(system) == "" {
			return fmt.Errorf("a system prompt is required (as an argument or via stdin)")
		}
		r := &roles.Role{Name: name, System: system}
		if err := roles.Save(r); err != nil {
			return err
		}
		printSuccess("✓ Saved role %q", name)
		return nil
	},
}

func init() {
	roleCmd.AddCommand(roleListCmd, roleShowCmd, roleAddCmd)
	rootCmd.AddCommand(roleCmd)
}
