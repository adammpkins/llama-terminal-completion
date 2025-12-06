package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adammpkins/llamaterm/internal/client"
	"github.com/spf13/cobra"
)

var explainCmd = &cobra.Command{
	Use:   "explain <file> [question]",
	Short: "Explain code or file contents",
	Long: `Analyze and explain the contents of a file.

Optionally, ask a specific question about the file.

Examples:
  lt explain main.go
  lt explain config.yaml "What does this configure?"
  lt explain error.log "What went wrong?"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runExplain,
}

func init() {
	rootCmd.AddCommand(explainCmd)
}

func runExplain(cmd *cobra.Command, args []string) error {
	filePath := args[0]
	var question string
	if len(args) > 1 {
		question = strings.Join(args[1:], " ")
	}

	// Read the file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Detect file type for context
	ext := filepath.Ext(filePath)
	fileType := detectFileType(ext)

	// Build the prompt
	var userPrompt string
	if question != "" {
		userPrompt = fmt.Sprintf("Here is a %s file named `%s`:\n\n```%s\n%s\n```\n\nQuestion: %s",
			fileType, filepath.Base(filePath), strings.TrimPrefix(ext, "."), string(content), question)
	} else {
		userPrompt = fmt.Sprintf("Here is a %s file named `%s`:\n\n```%s\n%s\n```\n\nPlease explain what this file does, its key components, and any notable patterns or issues.",
			fileType, filepath.Base(filePath), strings.TrimPrefix(ext, "."), string(content))
	}

	// Create API client
	apiClient := client.NewClient(cfg.BaseURL, cfg.APIKey, cfg.Model)

	messages := []client.ChatMessage{
		{
			Role:    "system",
			Content: "You are an expert code reviewer and technical writer. Explain code and files clearly and concisely. Point out important details, potential issues, and best practices when relevant.",
		},
		{
			Role:    "user",
			Content: userPrompt,
		},
	}

	// Get response
	fmt.Println()
	printInfo("📄 Analyzing %s...\n\n", filepath.Base(filePath))

	if cfg.Stream {
		err = apiClient.ChatCompletionStream(messages, cfg.MaxTokens, cfg.Temperature, func(content string) {
			fmt.Print(content)
		})
		fmt.Println()
	} else {
		resp, err := apiClient.ChatCompletion(messages, cfg.MaxTokens, cfg.Temperature)
		if err != nil {
			return err
		}
		if len(resp.Choices) > 0 {
			fmt.Println(resp.Choices[0].Message.Content)
		}
	}

	return err
}

func detectFileType(ext string) string {
	types := map[string]string{
		".go":    "Go source",
		".py":    "Python",
		".js":    "JavaScript",
		".ts":    "TypeScript",
		".jsx":   "React JSX",
		".tsx":   "React TSX",
		".rs":    "Rust",
		".c":     "C",
		".cpp":   "C++",
		".h":     "C/C++ header",
		".java":  "Java",
		".rb":    "Ruby",
		".php":   "PHP",
		".swift": "Swift",
		".kt":    "Kotlin",
		".sh":    "Shell script",
		".bash":  "Bash script",
		".zsh":   "Zsh script",
		".yaml":  "YAML configuration",
		".yml":   "YAML configuration",
		".json":  "JSON",
		".toml":  "TOML configuration",
		".xml":   "XML",
		".html":  "HTML",
		".css":   "CSS",
		".scss":  "SCSS",
		".sql":   "SQL",
		".md":    "Markdown",
		".txt":   "text",
		".log":   "log",
		".env":   "environment",
		".conf":  "configuration",
		".ini":   "INI configuration",
	}

	if t, ok := types[ext]; ok {
		return t
	}
	return "text"
}
