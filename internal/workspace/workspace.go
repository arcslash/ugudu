package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/arcslash/ugudu/internal/config"
)

// Workspace represents a project workspace
type Workspace struct {
	Name       string
	Path       string
	Config     *ProjectConfig
	sandboxes  map[string]*Sandbox // role -> sandbox
}

// New creates a new workspace for an existing project
func New(projectName string) (*Workspace, error) {
	projectPath := filepath.Join(config.ProjectsDir(), projectName)

	// Check project exists
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("project not found: %s", projectName)
	}

	// Load config
	configPath := filepath.Join(projectPath, ProjectConfigFile)
	cfg, err := LoadProjectConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("load project config: %w", err)
	}

	return &Workspace{
		Name:      projectName,
		Path:      projectPath,
		Config:    cfg,
		sandboxes: make(map[string]*Sandbox),
	}, nil
}

// Init creates a new project workspace with the given configuration
func Init(name, sourcePath, team string) (*Workspace, error) {
	// Validate name
	if name == "" {
		return nil, fmt.Errorf("project name is required")
	}
	if strings.ContainsAny(name, "/\\") {
		return nil, fmt.Errorf("project name cannot contain path separators")
	}

	// Resolve source path to absolute
	absSourcePath, err := filepath.Abs(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("resolve source path: %w", err)
	}

	// Check source path exists
	if _, err := os.Stat(absSourcePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("source path does not exist: %s", absSourcePath)
	}

	projectPath := filepath.Join(config.ProjectsDir(), name)

	// Check if project already exists
	if _, err := os.Stat(projectPath); err == nil {
		return nil, fmt.Errorf("project already exists: %s", name)
	}

	// Create project config
	cfg := NewProjectConfig(name, absSourcePath, team)

	// Create directory structure
	dirs := []string{
		projectPath,
		filepath.Join(projectPath, "tasks"),
		filepath.Join(projectPath, "activity"),
		filepath.Join(projectPath, "artifacts", "reports"),
		filepath.Join(projectPath, "artifacts", "specs"),
		filepath.Join(projectPath, "artifacts", "tests"),
		filepath.Join(projectPath, "workspaces"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	// Save config
	configPath := filepath.Join(projectPath, ProjectConfigFile)
	if err := cfg.Save(configPath); err != nil {
		return nil, fmt.Errorf("save project config: %w", err)
	}

	// Initialize empty tasks file
	tasksPath := filepath.Join(projectPath, "tasks", "tasks.json")
	if err := os.WriteFile(tasksPath, []byte("[]"), 0644); err != nil {
		return nil, fmt.Errorf("create tasks file: %w", err)
	}

	// Add to project index
	if err := AddProjectToIndex(name, absSourcePath); err != nil {
		return nil, fmt.Errorf("update project index: %w", err)
	}

	return &Workspace{
		Name:      name,
		Path:      projectPath,
		Config:    cfg,
		sandboxes: make(map[string]*Sandbox),
	}, nil
}

// Delete removes a project workspace
func Delete(name string) error {
	projectPath := filepath.Join(config.ProjectsDir(), name)

	// Check project exists
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return fmt.Errorf("project not found: %s", name)
	}

	// Remove from index
	if err := RemoveProjectFromIndex(name); err != nil {
		return fmt.Errorf("update project index: %w", err)
	}

	// Delete project directory
	if err := os.RemoveAll(projectPath); err != nil {
		return fmt.Errorf("delete project directory: %w", err)
	}

	return nil
}

// GetSandbox returns the sandbox for a specific agent role
func (w *Workspace) GetSandbox(role string) *Sandbox {
	if sandbox, ok := w.sandboxes[role]; ok {
		return sandbox
	}

	// Create sandbox
	sandboxPath := w.GetSandboxPath(role)
	sandbox := NewSandbox(
		sandboxPath,
		w.Config.Source.Path,
		w.Config.Source.SharedPaths,
		w.Config.Workspace.Isolation,
	)
	w.sandboxes[role] = sandbox

	// Ensure sandbox directory exists
	os.MkdirAll(sandboxPath, 0755)

	return sandbox
}

// GetSandboxPath returns the sandbox directory for an agent role
func (w *Workspace) GetSandboxPath(role string) string {
	return filepath.Join(w.Path, "workspaces", role)
}

// TasksPath returns the path to the tasks file
func (w *Workspace) TasksPath() string {
	return filepath.Join(w.Path, "tasks", "tasks.json")
}

// ActivityPath returns the path to the activity log for a role
func (w *Workspace) ActivityPath(role string) string {
	return filepath.Join(w.Path, "activity", role+".log")
}

// ArtifactPath returns the path to an artifact directory
func (w *Workspace) ArtifactPath(artifactType string) string {
	return filepath.Join(w.Path, "artifacts", artifactType)
}

// ResolveSourcePath resolves a relative path within the source directory
func (w *Workspace) ResolveSourcePath(relativePath string) string {
	return filepath.Join(w.Config.Source.Path, relativePath)
}
