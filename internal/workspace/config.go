// Package workspace handles project workspace management
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// ProjectConfigFile is the name of the project configuration file
	ProjectConfigFile = "project.yaml"

	// IsolationSandbox means agents write to their own sandbox
	IsolationSandbox = "sandbox"
	// IsolationShared means agents share a workspace
	IsolationShared = "shared"
	// IsolationNone means agents can write anywhere
	IsolationNone = "none"
)

// ProjectConfig represents the project.yaml configuration
type ProjectConfig struct {
	APIVersion string          `yaml:"apiVersion" json:"api_version"`
	Kind       string          `yaml:"kind" json:"kind"`
	Metadata   ProjectMetadata `yaml:"metadata" json:"metadata"`
	Source     SourceConfig    `yaml:"source" json:"source"`
	Team       string          `yaml:"team" json:"team"`
	Workspace  WorkspaceConfig `yaml:"workspace" json:"workspace"`
	Activity   ActivityConfig  `yaml:"activity" json:"activity"`
}

// ProjectMetadata contains project metadata
type ProjectMetadata struct {
	Name      string    `yaml:"name" json:"name"`
	CreatedAt time.Time `yaml:"created_at" json:"created_at"`
	UpdatedAt time.Time `yaml:"updated_at,omitempty" json:"updated_at,omitempty"`
}

// SourceConfig defines source code locations
type SourceConfig struct {
	Path        string   `yaml:"path" json:"path"`                 // Primary source code location
	SharedPaths []string `yaml:"shared_paths" json:"shared_paths"` // Additional readable paths
}

// WorkspaceConfig defines workspace behavior
type WorkspaceConfig struct {
	Isolation         string `yaml:"isolation" json:"isolation"`                   // sandbox, shared, none
	ArtifactRetention string `yaml:"artifact_retention" json:"artifact_retention"` // e.g., "30d"
}

// ActivityConfig defines activity logging settings
type ActivityConfig struct {
	LogLevel  string `yaml:"log_level" json:"log_level"`   // debug, info, warn, error
	Retention string `yaml:"retention" json:"retention"`   // e.g., "90d"
}

// NewProjectConfig creates a new project configuration with defaults
func NewProjectConfig(name, sourcePath, team string) *ProjectConfig {
	now := time.Now()
	return &ProjectConfig{
		APIVersion: "ugudu/v1",
		Kind:       "Project",
		Metadata: ProjectMetadata{
			Name:      name,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Source: SourceConfig{
			Path:        sourcePath,
			SharedPaths: []string{},
		},
		Team: team,
		Workspace: WorkspaceConfig{
			Isolation:         IsolationSandbox,
			ArtifactRetention: "30d",
		},
		Activity: ActivityConfig{
			LogLevel:  "info",
			Retention: "90d",
		},
	}
}

// LoadProjectConfig loads a project configuration from a file
func LoadProjectConfig(path string) (*ProjectConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read project config: %w", err)
	}

	var cfg ProjectConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse project config: %w", err)
	}

	return &cfg, nil
}

// Save writes the project configuration to the specified path
func (c *ProjectConfig) Save(path string) error {
	c.Metadata.UpdatedAt = time.Now()

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal project config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// Validate checks that the configuration is valid
func (c *ProjectConfig) Validate() error {
	if c.Metadata.Name == "" {
		return fmt.Errorf("project name is required")
	}

	if c.Source.Path == "" {
		return fmt.Errorf("source path is required")
	}

	// Validate source path exists
	if _, err := os.Stat(c.Source.Path); os.IsNotExist(err) {
		return fmt.Errorf("source path does not exist: %s", c.Source.Path)
	}

	// Validate shared paths exist
	for _, p := range c.Source.SharedPaths {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			return fmt.Errorf("shared path does not exist: %s", p)
		}
	}

	// Validate isolation mode
	switch c.Workspace.Isolation {
	case IsolationSandbox, IsolationShared, IsolationNone:
		// valid
	default:
		return fmt.Errorf("invalid isolation mode: %s", c.Workspace.Isolation)
	}

	return nil
}
