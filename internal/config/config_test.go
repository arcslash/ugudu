package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUguduHome(t *testing.T) {
	// Test with UGUDU_HOME set
	os.Setenv("UGUDU_HOME", "/tmp/ugudu-test")
	defer os.Unsetenv("UGUDU_HOME")

	home := UguduHome()
	if home != "/tmp/ugudu-test" {
		t.Errorf("Expected /tmp/ugudu-test, got %s", home)
	}
}

func TestUguduHomeDefault(t *testing.T) {
	os.Unsetenv("UGUDU_HOME")

	home := UguduHome()
	userHome, _ := os.UserHomeDir()
	expected := filepath.Join(userHome, ".ugudu")

	if home != expected {
		t.Errorf("Expected %s, got %s", expected, home)
	}
}

func TestSpecsDir(t *testing.T) {
	os.Setenv("UGUDU_HOME", "/tmp/ugudu-test")
	defer os.Unsetenv("UGUDU_HOME")

	dir := SpecsDir()
	if dir != "/tmp/ugudu-test/specs" {
		t.Errorf("Expected /tmp/ugudu-test/specs, got %s", dir)
	}
}

func TestDataDir(t *testing.T) {
	os.Setenv("UGUDU_HOME", "/tmp/ugudu-test")
	defer os.Unsetenv("UGUDU_HOME")

	dir := DataDir()
	if dir != "/tmp/ugudu-test/data" {
		t.Errorf("Expected /tmp/ugudu-test/data, got %s", dir)
	}
}

func TestConfigPath(t *testing.T) {
	os.Setenv("UGUDU_HOME", "/tmp/ugudu-test")
	defer os.Unsetenv("UGUDU_HOME")

	path := ConfigPath()
	if path != "/tmp/ugudu-test/config.yaml" {
		t.Errorf("Expected /tmp/ugudu-test/config.yaml, got %s", path)
	}
}

func TestEnsureDirectories(t *testing.T) {
	// Use temp directory
	tmpDir := t.TempDir()
	os.Setenv("UGUDU_HOME", tmpDir)
	defer os.Unsetenv("UGUDU_HOME")

	err := EnsureDirectories()
	if err != nil {
		t.Fatalf("EnsureDirectories failed: %v", err)
	}

	// Check directories exist
	dirs := []string{
		tmpDir,
		filepath.Join(tmpDir, "specs"),
		filepath.Join(tmpDir, "data"),
	}

	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Directory %s was not created", dir)
		}
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("UGUDU_HOME", tmpDir)
	defer os.Unsetenv("UGUDU_HOME")

	// Create and save config
	cfg := &Config{
		Providers: ProvidersConfig{
			Anthropic: AnthropicConfig{APIKey: "test-key"},
			Ollama:    OllamaConfig{URL: "http://localhost:11434"},
		},
		Defaults: DefaultsConfig{
			Provider: "anthropic",
			Model:    "claude-sonnet-4-20250514",
		},
	}

	if err := Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load config
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Providers.Anthropic.APIKey != "test-key" {
		t.Errorf("Expected API key 'test-key', got '%s'", loaded.Providers.Anthropic.APIKey)
	}

	if loaded.Defaults.Provider != "anthropic" {
		t.Errorf("Expected provider 'anthropic', got '%s'", loaded.Defaults.Provider)
	}
}

func TestLoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("UGUDU_HOME", tmpDir)
	defer os.Unsetenv("UGUDU_HOME")

	// Load non-existent config should return empty config
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load should not fail for non-existent config: %v", err)
	}

	if cfg == nil {
		t.Fatal("Config should not be nil")
	}
}

func TestApplyToEnvironment(t *testing.T) {
	// Clear any existing env vars
	os.Unsetenv("ANTHROPIC_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")

	cfg := &Config{
		Providers: ProvidersConfig{
			Anthropic: AnthropicConfig{APIKey: "test-anthropic-key"},
			OpenAI:    OpenAIConfig{APIKey: "test-openai-key"},
		},
	}

	cfg.ApplyToEnvironment()

	if os.Getenv("ANTHROPIC_API_KEY") != "test-anthropic-key" {
		t.Error("ANTHROPIC_API_KEY not set correctly")
	}

	if os.Getenv("OPENAI_API_KEY") != "test-openai-key" {
		t.Error("OPENAI_API_KEY not set correctly")
	}

	// Cleanup
	os.Unsetenv("ANTHROPIC_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")
}

func TestApplyToEnvironmentNoOverwrite(t *testing.T) {
	// Set existing env var
	os.Setenv("ANTHROPIC_API_KEY", "existing-key")
	defer os.Unsetenv("ANTHROPIC_API_KEY")

	cfg := &Config{
		Providers: ProvidersConfig{
			Anthropic: AnthropicConfig{APIKey: "new-key"},
		},
	}

	cfg.ApplyToEnvironment()

	// Should NOT overwrite existing
	if os.Getenv("ANTHROPIC_API_KEY") != "existing-key" {
		t.Error("Should not overwrite existing ANTHROPIC_API_KEY")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Defaults.Provider != "anthropic" {
		t.Errorf("Expected default provider 'anthropic', got '%s'", cfg.Defaults.Provider)
	}

	if cfg.Daemon.TCPAddr != ":8080" {
		t.Errorf("Expected default TCP addr ':8080', got '%s'", cfg.Daemon.TCPAddr)
	}
}
