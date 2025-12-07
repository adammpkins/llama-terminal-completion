package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/adammpkins/llamaterm/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage LlamaTerm configuration",
	Long: `View and manage LlamaTerm configuration.

Examples:
  lt config show      Show current configuration
  lt config path      Show config file location
  lt config init      Create a config file with defaults`,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println()
		printInfo("LlamaTerm Configuration\n")
		fmt.Println("─────────────────────────")
		fmt.Printf("  Base URL:     %s\n", cfg.BaseURL)
		fmt.Printf("  Model:        %s\n", cfg.Model)
		if cfg.APIKey != "" {
			fmt.Printf("  API Key:      %s...%s\n", cfg.APIKey[:4], cfg.APIKey[len(cfg.APIKey)-4:])
		} else {
			fmt.Printf("  API Key:      (not set)\n")
		}
		fmt.Printf("  Max Tokens:   %d\n", cfg.MaxTokens)
		fmt.Printf("  Temperature:  %.1f\n", cfg.Temperature)
		fmt.Printf("  Streaming:    %t\n", cfg.Stream)
		fmt.Printf("  Confirm Cmds: %t\n", cfg.ConfirmCommands)
		fmt.Printf("  Shell:        %s\n", cfg.Shell)
		fmt.Println()
		return nil
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show config file location",
	Run: func(cmd *cobra.Command, args []string) {
		path := config.GetConfigPath()
		fmt.Println(path)

		// Check if it exists
		if _, err := os.Stat(path); os.IsNotExist(err) {
			printWarning("(file does not exist - run 'lt config init' to create)")
		}
	},
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a config file with defaults",
	RunE: func(cmd *cobra.Command, args []string) error {
		path := config.GetConfigPath()

		// Check if already exists
		if _, err := os.Stat(path); err == nil {
			printWarning("Config file already exists at: %s", path)
			fmt.Print("Overwrite? [y/N]: ")
			var response string
			_, _ = fmt.Scanln(&response)
			if response != "y" && response != "Y" {
				return nil
			}
		}

		// Create directory if needed
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}

		// Write default config
		configContent := `# LlamaTerm Configuration
# https://github.com/adammpkins/llamaterm

# API endpoint (works with any OpenAI-compatible API)
base_url: http://localhost:11434/v1

# Model to use
model: llama3.2

# API key (optional for local endpoints like Ollama)
api_key: ""

# Generation settings
max_tokens: 1024
temperature: 0.7

# Behavior
stream: true
confirm_commands: true
shell: /bin/zsh

# ─────────────────────────────────────────────────────
# Provider Examples:
# ─────────────────────────────────────────────────────
#
# Ollama (default, no API key needed):
#   base_url: http://localhost:11434/v1
#   model: llama3.2
#
# LM Studio:
#   base_url: http://localhost:1234/v1
#   model: local-model
#
# OpenAI:
#   base_url: https://api.openai.com/v1
#   model: gpt-4o-mini
#   api_key: sk-...
#
# Anthropic (via OpenAI-compatible proxy):
#   base_url: https://your-proxy.com/v1
#   model: claude-3-sonnet
#   api_key: ...
`

		if err := os.WriteFile(path, []byte(configContent), 0644); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}

		printSuccess("✓ Created config file: %s", path)
		fmt.Println()
		fmt.Println("Edit this file to configure LlamaTerm for your API provider.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configInitCmd)
}
