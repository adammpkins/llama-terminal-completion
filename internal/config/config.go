package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds all configuration for LlamaTerm
type Config struct {
	// API Configuration
	BaseURL     string  `mapstructure:"base_url"`
	APIKey      string  `mapstructure:"api_key"`
	Model       string  `mapstructure:"model"`
	MaxTokens   int     `mapstructure:"max_tokens"`
	Temperature float64 `mapstructure:"temperature"`

	// Behavior
	Stream          bool   `mapstructure:"stream"`
	ConfirmCommands bool   `mapstructure:"confirm_commands"`
	Shell           string `mapstructure:"shell"`
	DefaultRole     string `mapstructure:"default_role"`

	// Structured output: "auto" (try native, fall back to prompt), "native", or "prompt".
	StructuredOutputMode string `mapstructure:"structured_output_mode"`

	// RAG / embeddings
	EmbeddingModel string `mapstructure:"embedding_model"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		BaseURL:              "http://localhost:11434/v1",
		APIKey:               "",
		Model:                "llama3.2",
		MaxTokens:            1024,
		Temperature:          0.7,
		Stream:               true,
		ConfirmCommands:      true,
		Shell:                "/bin/zsh",
		StructuredOutputMode: "auto",
		EmbeddingModel:       "nomic-embed-text",
	}
}

// Load reads configuration from file and environment variables
func Load() (*Config, error) {
	cfg := DefaultConfig()

	// Set up config file paths
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// Add config paths in order of priority (first match wins)
	// 1. User config dir (~/Library/Application Support/lt on macOS, ~/.config/lt on Linux)
	if configDir, err := os.UserConfigDir(); err == nil {
		viper.AddConfigPath(filepath.Join(configDir, "lt"))
	}
	// 2. Home directory ~/.config/lt (explicit for Linux compatibility)
	if home, err := os.UserHomeDir(); err == nil {
		viper.AddConfigPath(filepath.Join(home, ".config", "lt"))
	}
	// 3. Current directory
	viper.AddConfigPath(".")

	// Environment variable bindings
	viper.SetEnvPrefix("LT")
	viper.AutomaticEnv()

	// Also support OPENAI_* environment variables
	_ = viper.BindEnv("api_key", "LT_API_KEY", "OPENAI_API_KEY")
	_ = viper.BindEnv("base_url", "LT_BASE_URL", "OPENAI_BASE_URL")
	_ = viper.BindEnv("model", "LT_MODEL", "OPENAI_MODEL")

	// AutomaticEnv does not reliably bind keys that are only read via Unmarshal,
	// so bind the remaining settings explicitly for LT_* overrides.
	_ = viper.BindEnv("max_tokens", "LT_MAX_TOKENS")
	_ = viper.BindEnv("temperature", "LT_TEMPERATURE")
	_ = viper.BindEnv("stream", "LT_STREAM")
	_ = viper.BindEnv("confirm_commands", "LT_CONFIRM_COMMANDS")
	_ = viper.BindEnv("shell", "LT_SHELL")
	_ = viper.BindEnv("default_role", "LT_DEFAULT_ROLE")
	_ = viper.BindEnv("structured_output_mode", "LT_STRUCTURED_OUTPUT_MODE")
	_ = viper.BindEnv("embedding_model", "LT_EMBEDDING_MODEL")

	// Read config file (ignore if not found)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config: %w", err)
		}
	}

	// Unmarshal into config struct
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("error parsing config: %w", err)
	}

	return cfg, nil
}

// Injectable functions for testing
var (
	configUserConfigDirFunc = os.UserConfigDir
	configUserHomeDirFunc   = os.UserHomeDir
)

// GetConfigPath returns the path where config file should be stored
func GetConfigPath() string {
	if configDir, err := configUserConfigDirFunc(); err == nil {
		return filepath.Join(configDir, "lt", "config.yaml")
	}
	if home, err := configUserHomeDirFunc(); err == nil {
		return filepath.Join(home, ".ltrc.yaml")
	}
	return ".ltrc.yaml"
}
