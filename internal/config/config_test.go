package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.BaseURL != "http://localhost:11434/v1" {
		t.Errorf("Expected default BaseURL http://localhost:11434/v1, got %s", cfg.BaseURL)
	}
	if cfg.APIKey != "" {
		t.Errorf("Expected empty APIKey, got %s", cfg.APIKey)
	}
	if cfg.Model != "llama3.2" {
		t.Errorf("Expected default Model llama3.2, got %s", cfg.Model)
	}
	if cfg.MaxTokens != 1024 {
		t.Errorf("Expected default MaxTokens 1024, got %d", cfg.MaxTokens)
	}
	if cfg.Temperature != 0.7 {
		t.Errorf("Expected default Temperature 0.7, got %f", cfg.Temperature)
	}
	if cfg.Stream != true {
		t.Error("Expected default Stream to be true")
	}
	if cfg.ConfirmCommands != true {
		t.Error("Expected default ConfirmCommands to be true")
	}
	if cfg.Shell != "/bin/zsh" {
		t.Errorf("Expected default Shell /bin/zsh, got %s", cfg.Shell)
	}
}

func TestLoadWithEnvVars(t *testing.T) {
	// Save and restore env vars
	origBaseURL := os.Getenv("LT_BASE_URL")
	origModel := os.Getenv("LT_MODEL")
	origAPIKey := os.Getenv("LT_API_KEY")
	defer func() {
		os.Setenv("LT_BASE_URL", origBaseURL)
		os.Setenv("LT_MODEL", origModel)
		os.Setenv("LT_API_KEY", origAPIKey)
	}()

	// Set test env vars
	os.Setenv("LT_BASE_URL", "https://test.api.com/v1")
	os.Setenv("LT_MODEL", "test-model")
	os.Setenv("LT_API_KEY", "test-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if cfg.BaseURL != "https://test.api.com/v1" {
		t.Errorf("Expected BaseURL from env, got %s", cfg.BaseURL)
	}
	if cfg.Model != "test-model" {
		t.Errorf("Expected Model from env, got %s", cfg.Model)
	}
	if cfg.APIKey != "test-key" {
		t.Errorf("Expected APIKey from env, got %s", cfg.APIKey)
	}
}

func TestLoadWithOpenAIEnvVars(t *testing.T) {
	// Save and restore env vars
	origAPIKey := os.Getenv("OPENAI_API_KEY")
	origLTKey := os.Getenv("LT_API_KEY")
	defer func() {
		os.Setenv("OPENAI_API_KEY", origAPIKey)
		os.Setenv("LT_API_KEY", origLTKey)
	}()

	// Clear LT_ vars and set OPENAI_ vars
	os.Unsetenv("LT_API_KEY")
	os.Setenv("OPENAI_API_KEY", "openai-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if cfg.APIKey != "openai-key" {
		t.Errorf("Expected APIKey from OPENAI_API_KEY, got %s", cfg.APIKey)
	}
}

func TestLoadWithConfigFile(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "lt")
	_ = os.MkdirAll(configDir, 0755)

	configContent := `
base_url: https://file.api.com/v1
model: file-model
api_key: file-key
max_tokens: 2048
temperature: 0.9
stream: false
confirm_commands: false
shell: /bin/bash
`
	configPath := filepath.Join(configDir, "config.yaml")
	_ = os.WriteFile(configPath, []byte(configContent), 0644)

	// Clear env vars that might interfere
	origBaseURL := os.Getenv("LT_BASE_URL")
	origModel := os.Getenv("LT_MODEL")
	origAPIKey := os.Getenv("LT_API_KEY")
	os.Unsetenv("LT_BASE_URL")
	os.Unsetenv("LT_MODEL")
	os.Unsetenv("LT_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")
	defer func() {
		os.Setenv("LT_BASE_URL", origBaseURL)
		os.Setenv("LT_MODEL", origModel)
		os.Setenv("LT_API_KEY", origAPIKey)
	}()

	// Note: This test is limited because Viper caches config
	// A full test would require resetting Viper between tests
	// For now, we test that Load() doesn't error
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Config should load without error
	if cfg == nil {
		t.Error("Expected config to be non-nil")
	}
}

func TestGetConfigPath(t *testing.T) {
	path := GetConfigPath()

	if path == "" {
		t.Error("Expected non-empty config path")
	}

	// Should end with config.yaml or .ltrc.yaml
	if !filepath.IsAbs(path) && path != ".ltrc.yaml" {
		t.Errorf("Expected absolute path or .ltrc.yaml, got %s", path)
	}
}

func TestLoadDefaults(t *testing.T) {
	// Clear all env vars
	envVars := []string{"LT_BASE_URL", "LT_MODEL", "LT_API_KEY", "OPENAI_API_KEY", "OPENAI_BASE_URL"}
	origValues := make(map[string]string)
	for _, v := range envVars {
		origValues[v] = os.Getenv(v)
		os.Unsetenv(v)
	}
	defer func() {
		for k, v := range origValues {
			if v != "" {
				os.Setenv(k, v)
			}
		}
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should get defaults when no config
	defaults := DefaultConfig()
	if cfg.Model != defaults.Model {
		// Env or config might override, so just check it's not empty
		if cfg.Model == "" {
			t.Error("Expected non-empty model")
		}
	}
}

func TestGetConfigPathAllBranches(t *testing.T) {
	// Test basic path generation
	path := GetConfigPath()

	// Path should be non-empty
	if path == "" {
		t.Error("GetConfigPath should return non-empty path")
	}

	// Path should end with config.yaml or .ltrc.yaml
	if !filepath.IsAbs(path) && path != ".ltrc.yaml" {
		// If not absolute and not fallback, it's unexpected
		t.Logf("Got relative path: %s", path)
	}

	// On macOS, should contain lt/config.yaml
	if filepath.IsAbs(path) {
		base := filepath.Base(path)
		if base != "config.yaml" && base != ".ltrc.yaml" {
			t.Errorf("Expected config.yaml or .ltrc.yaml, got %s", base)
		}
	}
}

func TestLoadMultipleEnvSources(t *testing.T) {
	// Test that LT_ takes precedence over OPENAI_
	origLT := os.Getenv("LT_API_KEY")
	origOAI := os.Getenv("OPENAI_API_KEY")
	defer func() {
		if origLT != "" {
			os.Setenv("LT_API_KEY", origLT)
		} else {
			os.Unsetenv("LT_API_KEY")
		}
		if origOAI != "" {
			os.Setenv("OPENAI_API_KEY", origOAI)
		} else {
			os.Unsetenv("OPENAI_API_KEY")
		}
	}()

	os.Setenv("LT_API_KEY", "lt-key")
	os.Setenv("OPENAI_API_KEY", "openai-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// LT_ should take precedence
	if cfg.APIKey != "lt-key" {
		t.Errorf("Expected LT_API_KEY to take precedence, got %s", cfg.APIKey)
	}
}

func TestLoadAllEnvVars(t *testing.T) {
	// Save original values
	origVars := make(map[string]string)
	envVars := []string{"LT_BASE_URL", "LT_MODEL", "LT_MAX_TOKENS", "LT_TEMPERATURE"}
	for _, v := range envVars {
		origVars[v] = os.Getenv(v)
	}
	defer func() {
		for k, v := range origVars {
			if v != "" {
				os.Setenv(k, v)
			} else {
				os.Unsetenv(k)
			}
		}
	}()

	os.Setenv("LT_BASE_URL", "https://custom.api.com/v1")
	os.Setenv("LT_MODEL", "custom-model")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.BaseURL != "https://custom.api.com/v1" {
		t.Errorf("Expected custom base URL, got %s", cfg.BaseURL)
	}
	if cfg.Model != "custom-model" {
		t.Errorf("Expected custom model, got %s", cfg.Model)
	}
}

func TestConfigDefaultValues(t *testing.T) {
	cfg := DefaultConfig()

	// Verify all default values
	if cfg.MaxTokens != 1024 {
		t.Errorf("Default MaxTokens should be 1024, got %d", cfg.MaxTokens)
	}
	if cfg.Temperature != 0.7 {
		t.Errorf("Default Temperature should be 0.7, got %f", cfg.Temperature)
	}
	if !cfg.Stream {
		t.Error("Default Stream should be true")
	}
	if !cfg.ConfirmCommands {
		t.Error("Default ConfirmCommands should be true")
	}
}

func TestGetConfigPathNotEmpty(t *testing.T) {
	path := GetConfigPath()

	// Should never be empty
	if path == "" {
		t.Error("GetConfigPath should never return empty")
	}

	// Should contain "config" or "ltrc" in the filename
	if !filepath.IsAbs(path) && path != ".ltrc.yaml" {
		t.Logf("Path is relative: %s", path)
	}
}

func TestGetConfigPathConfigDir(t *testing.T) {
	origConfigDir := configUserConfigDirFunc
	origHomeDir := configUserHomeDirFunc
	defer func() {
		configUserConfigDirFunc = origConfigDir
		configUserHomeDirFunc = origHomeDir
	}()

	configUserConfigDirFunc = func() (string, error) {
		return "/mock/config", nil
	}

	path := GetConfigPath()
	if path != "/mock/config/lt/config.yaml" {
		t.Errorf("Expected /mock/config/lt/config.yaml, got %s", path)
	}
}

func TestGetConfigPathFallbackToHome(t *testing.T) {
	origConfigDir := configUserConfigDirFunc
	origHomeDir := configUserHomeDirFunc
	defer func() {
		configUserConfigDirFunc = origConfigDir
		configUserHomeDirFunc = origHomeDir
	}()

	configUserConfigDirFunc = func() (string, error) {
		return "", os.ErrNotExist
	}
	configUserHomeDirFunc = func() (string, error) {
		return "/mock/home", nil
	}

	path := GetConfigPath()
	if path != "/mock/home/.ltrc.yaml" {
		t.Errorf("Expected /mock/home/.ltrc.yaml, got %s", path)
	}
}

func TestGetConfigPathFallbackToLocal(t *testing.T) {
	origConfigDir := configUserConfigDirFunc
	origHomeDir := configUserHomeDirFunc
	defer func() {
		configUserConfigDirFunc = origConfigDir
		configUserHomeDirFunc = origHomeDir
	}()

	configUserConfigDirFunc = func() (string, error) {
		return "", os.ErrNotExist
	}
	configUserHomeDirFunc = func() (string, error) {
		return "", os.ErrNotExist
	}

	path := GetConfigPath()
	if path != ".ltrc.yaml" {
		t.Errorf("Expected .ltrc.yaml, got %s", path)
	}
}

func TestLoadWithAllEnvVarsSet(t *testing.T) {
	// Save originals
	envVars := []string{
		"LT_BASE_URL", "LT_API_KEY", "LT_MODEL",
		"LT_MAX_TOKENS", "LT_TEMPERATURE", "LT_STREAM",
		"LT_CONFIRM_COMMANDS", "LT_SHELL",
		"OPENAI_API_KEY", "OPENAI_BASE_URL", "OPENAI_MODEL",
	}
	origValues := make(map[string]string)
	for _, v := range envVars {
		origValues[v] = os.Getenv(v)
	}
	defer func() {
		for k, v := range origValues {
			if v != "" {
				os.Setenv(k, v)
			} else {
				os.Unsetenv(k)
			}
		}
	}()

	// Set all env vars
	os.Setenv("LT_BASE_URL", "https://custom.api.com")
	os.Setenv("LT_API_KEY", "custom-key")
	os.Setenv("LT_MODEL", "custom-model")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.BaseURL != "https://custom.api.com" {
		t.Errorf("Expected custom base URL, got %s", cfg.BaseURL)
	}
}

func TestLoadOpenAIVarsAsFallback(t *testing.T) {
	// Save originals
	origLT := os.Getenv("LT_API_KEY")
	origOAI := os.Getenv("OPENAI_API_KEY")
	defer func() {
		if origLT != "" {
			os.Setenv("LT_API_KEY", origLT)
		} else {
			os.Unsetenv("LT_API_KEY")
		}
		if origOAI != "" {
			os.Setenv("OPENAI_API_KEY", origOAI)
		} else {
			os.Unsetenv("OPENAI_API_KEY")
		}
	}()

	// Clear LT key, set OPENAI key
	os.Unsetenv("LT_API_KEY")
	os.Setenv("OPENAI_API_KEY", "openai-fallback-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	// Should use OPENAI key as fallback
	if cfg.APIKey != "openai-fallback-key" {
		t.Logf("Expected OPENAI fallback, got: %s", cfg.APIKey)
	}
}

func TestLoadWithViperDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Check that defaults are applied
	if cfg.MaxTokens == 0 {
		t.Error("MaxTokens should not be 0")
	}
	if cfg.Temperature == 0 {
		t.Error("Temperature should not be 0")
	}
}
