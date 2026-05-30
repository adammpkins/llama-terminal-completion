package cli

import (
	"fmt"
	"os"

	"github.com/adammpkins/llama-terminal-completion/internal/config"
	"github.com/adammpkins/llama-terminal-completion/internal/roles"
	"github.com/spf13/cobra"
)

var (
	// Version is set at build time
	Version = "dev"

	// Global flags
	cfgFile     string
	baseURL     string
	apiKey      string
	model       string
	noStream    bool
	maxTokens   int
	temperature float64
	roleName    string

	// Global config
	cfg *config.Config

	// activeRole is the role resolved from --role or config default, if any.
	activeRole *roles.Role
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "lt",
	Short: "LlamaTerm - AI assistant in your terminal",
	Long: `LlamaTerm (lt) is a command-line AI assistant that works with any 
OpenAI-compatible API endpoint.

Works out of the box with Ollama, LM Studio, llama.cpp server, 
OpenAI, and more.

Examples:
  lt ask "How do I list files in Linux?"
  lt cmd "find all .go files modified today"
  lt chat`,
	Version: Version,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		cfg, err = config.Load()
		if err != nil {
			return err
		}

		// Override with flags if provided
		if baseURL != "" {
			cfg.BaseURL = baseURL
		}
		if apiKey != "" {
			cfg.APIKey = apiKey
		}
		if model != "" {
			cfg.Model = model
		}
		if cmd.Flags().Changed("no-stream") {
			cfg.Stream = !noStream
		}
		if cmd.Flags().Changed("max-tokens") {
			cfg.MaxTokens = maxTokens
		}
		if cmd.Flags().Changed("temperature") {
			cfg.Temperature = temperature
		}

		// Resolve the active role (--role flag overrides config default).
		if err := resolveRole(cmd); err != nil {
			return err
		}

		// Initialize markdown renderer for terminal output
		initMarkdownRenderer()

		return nil
	},
}

// resolveRole loads the role named by --role (or cfg.DefaultRole) and applies
// its model/temperature overrides unless those were set explicitly via flags.
func resolveRole(cmd *cobra.Command) error {
	name := roleName
	if name == "" {
		name = cfg.DefaultRole
	}
	if name == "" {
		return nil
	}
	r, err := roles.Load(name)
	if err != nil {
		return err
	}
	activeRole = r
	if r.Model != "" && !cmd.Flags().Changed("model") {
		cfg.Model = r.Model
	}
	if r.Temperature != nil && !cmd.Flags().Changed("temperature") {
		cfg.Temperature = *r.Temperature
	}
	return nil
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

// activeRoleSystem returns the system prompt of the role selected via --role
// (or the config default), or "" if none.
func activeRoleSystem() string {
	if activeRole != nil {
		return activeRole.System
	}
	return ""
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.config/lt/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&baseURL, "base-url", "", "API base URL (default: http://localhost:11434/v1)")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "API key")
	rootCmd.PersistentFlags().StringVarP(&model, "model", "m", "", "Model to use (default: llama3.2)")
	rootCmd.PersistentFlags().BoolVar(&noStream, "no-stream", false, "Disable streaming output")
	rootCmd.PersistentFlags().IntVar(&maxTokens, "max-tokens", 0, "Maximum tokens to generate")
	rootCmd.PersistentFlags().Float64Var(&temperature, "temperature", 0, "Temperature for generation")
	rootCmd.PersistentFlags().StringVarP(&roleName, "role", "r", "", "Use a named role (system-prompt preset)")

	// Add subcommands
	rootCmd.AddCommand(askCmd)
	rootCmd.AddCommand(cmdCmd)
	rootCmd.AddCommand(versionCmd)
}

// versionCmd prints detailed version info
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("LlamaTerm %s\n", Version)
		if cfg != nil {
			fmt.Printf("  API: %s\n", cfg.BaseURL)
			fmt.Printf("  Model: %s\n", cfg.Model)
		}
	},
}

// printError prints an error message in red
func printError(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "\033[91mError: "+format+"\033[0m\n", a...)
}

// printSuccess prints a success message in green
func printSuccess(format string, a ...interface{}) {
	fmt.Printf("\033[92m"+format+"\033[0m\n", a...)
}

// printWarning prints a warning message in yellow
func printWarning(format string, a ...interface{}) {
	fmt.Printf("\033[93m"+format+"\033[0m\n", a...)
}

// printInfo prints info in cyan
func printInfo(format string, a ...interface{}) {
	fmt.Printf("\033[96m"+format+"\033[0m", a...)
}
