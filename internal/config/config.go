// Package config handles Ugudu configuration
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// UguduHome returns the Ugudu home directory
func UguduHome() string {
	// Check UGUDU_HOME environment variable first
	if home := os.Getenv("UGUDU_HOME"); home != "" {
		return home
	}

	// Default to ~/.ugudu
	userHome, err := os.UserHomeDir()
	if err != nil {
		return ".ugudu"
	}
	return filepath.Join(userHome, ".ugudu")
}

// SpecsDir returns the specs (blueprints) directory
func SpecsDir() string {
	return filepath.Join(UguduHome(), "specs")
}

// DataDir returns the data directory
func DataDir() string {
	return filepath.Join(UguduHome(), "data")
}

// ConfigPath returns the config file path
func ConfigPath() string {
	return filepath.Join(UguduHome(), "config.yaml")
}

// SocketPath returns the socket path
func SocketPath() string {
	return filepath.Join(UguduHome(), "ugudu.sock")
}

// ProjectsDir returns the centralized projects directory
func ProjectsDir() string {
	// Check UGUDU_PROJECTS environment variable first
	if projects := os.Getenv("UGUDU_PROJECTS"); projects != "" {
		return projects
	}

	// Default to ~/ugudu_projects
	userHome, err := os.UserHomeDir()
	if err != nil {
		return "ugudu_projects"
	}
	return filepath.Join(userHome, "ugudu_projects")
}

// ProjectIndexPath returns the path to the project index file
func ProjectIndexPath() string {
	return filepath.Join(ProjectsDir(), ".index.json")
}

// Config holds the global Ugudu configuration
type Config struct {
	// Provider API keys
	Providers ProvidersConfig `yaml:"providers"`

	// Default settings
	Defaults DefaultsConfig `yaml:"defaults"`

	// Daemon settings
	Daemon DaemonConfig `yaml:"daemon"`
}

// ProvidersConfig holds provider API keys
type ProvidersConfig struct {
	Anthropic AnthropicConfig `yaml:"anthropic,omitempty"`
	OpenAI    OpenAIConfig    `yaml:"openai,omitempty"`
	Groq      GroqConfig      `yaml:"groq,omitempty"`
	Ollama    OllamaConfig    `yaml:"ollama,omitempty"`
}

// AnthropicConfig holds Anthropic settings
type AnthropicConfig struct {
	APIKey string `yaml:"api_key,omitempty"`
}

// OpenAIConfig holds OpenAI settings
type OpenAIConfig struct {
	APIKey  string `yaml:"api_key,omitempty"`
	BaseURL string `yaml:"base_url,omitempty"`
}

// GroqConfig holds Groq settings
type GroqConfig struct {
	APIKey string `yaml:"api_key,omitempty"`
}

// OllamaConfig holds Ollama settings
type OllamaConfig struct {
	URL string `yaml:"url,omitempty"`
}

// DefaultsConfig holds default settings
type DefaultsConfig struct {
	Provider string `yaml:"provider,omitempty"`
	Model    string `yaml:"model,omitempty"`
}

// DaemonConfig holds daemon settings
type DaemonConfig struct {
	TCPAddr string `yaml:"tcp_addr,omitempty"`
}

// Load reads the config file
func Load() (*Config, error) {
	path := ConfigPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty config if file doesn't exist
			return &Config{}, nil
		}
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return &cfg, nil
}

// Save writes the config file
func Save(cfg *Config) error {
	path := ConfigPath()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600) // 0600 for security (contains API keys)
}

// EnsureDirectories creates the Ugudu directory structure
func EnsureDirectories() error {
	dirs := []string{
		UguduHome(),
		SpecsDir(),
		DataDir(),
		ProjectsDir(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}

	return nil
}

// ApplyToEnvironment sets environment variables from config
// This allows the provider registry to auto-discover from config
func (c *Config) ApplyToEnvironment() {
	if c.Providers.Anthropic.APIKey != "" && os.Getenv("ANTHROPIC_API_KEY") == "" {
		os.Setenv("ANTHROPIC_API_KEY", c.Providers.Anthropic.APIKey)
	}
	if c.Providers.OpenAI.APIKey != "" && os.Getenv("OPENAI_API_KEY") == "" {
		os.Setenv("OPENAI_API_KEY", c.Providers.OpenAI.APIKey)
	}
	if c.Providers.Groq.APIKey != "" && os.Getenv("GROQ_API_KEY") == "" {
		os.Setenv("GROQ_API_KEY", c.Providers.Groq.APIKey)
	}
	if c.Providers.Ollama.URL != "" && os.Getenv("OLLAMA_URL") == "" {
		os.Setenv("OLLAMA_URL", c.Providers.Ollama.URL)
	}
}

// DefaultConfig returns a config with example values (commented out)
func DefaultConfig() *Config {
	return &Config{
		Providers: ProvidersConfig{
			Anthropic: AnthropicConfig{
				APIKey: "", // Set your Anthropic API key
			},
			OpenAI: OpenAIConfig{
				APIKey: "", // Set your OpenAI API key
			},
			Ollama: OllamaConfig{
				URL: "http://localhost:11434",
			},
		},
		Defaults: DefaultsConfig{
			Provider: "anthropic",
			Model:    "claude-sonnet-4-20250514",
		},
		Daemon: DaemonConfig{
			TCPAddr: ":8080",
		},
	}
}
