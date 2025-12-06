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
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		BaseURL:         "http://localhost:11434/v1",
		APIKey:          "",
		Model:           "llama3.2",
		MaxTokens:       1024,
		Temperature:     0.7,
		Stream:          true,
		ConfirmCommands: true,
		Shell:           "/bin/zsh",
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

// GetConfigPath returns the path where config file should be stored
func GetConfigPath() string {
	if configDir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(configDir, "lt", "config.yaml")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".ltrc.yaml")
	}
	return ".ltrc.yaml"
}
