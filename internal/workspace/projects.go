package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/arcslash/ugudu/internal/config"
)

// ProjectIndexEntry represents an entry in the project index
type ProjectIndexEntry struct {
	Name       string    `json:"name"`
	SourcePath string    `json:"source_path"`
	Team       string    `json:"team"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ProjectIndex manages the global project index
type ProjectIndex struct {
	Projects []ProjectIndexEntry `json:"projects"`
	mu       sync.RWMutex
}

var (
	globalIndex     *ProjectIndex
	globalIndexOnce sync.Once
)

// GetProjectIndex returns the global project index
func GetProjectIndex() (*ProjectIndex, error) {
	var initErr error
	globalIndexOnce.Do(func() {
		globalIndex = &ProjectIndex{
			Projects: []ProjectIndexEntry{},
		}
		initErr = globalIndex.load()
	})

	if initErr != nil {
		return nil, initErr
	}

	return globalIndex, nil
}

// load reads the project index from disk
func (idx *ProjectIndex) load() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	indexPath := config.ProjectIndexPath()

	data, err := os.ReadFile(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create empty index
			idx.Projects = []ProjectIndexEntry{}
			return idx.saveUnsafe()
		}
		return fmt.Errorf("read project index: %w", err)
	}

	if err := json.Unmarshal(data, idx); err != nil {
		return fmt.Errorf("parse project index: %w", err)
	}

	return nil
}

// saveUnsafe writes the project index to disk without locking
func (idx *ProjectIndex) saveUnsafe() error {
	indexPath := config.ProjectIndexPath()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(indexPath), 0755); err != nil {
		return fmt.Errorf("create index directory: %w", err)
	}

	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal project index: %w", err)
	}

	return os.WriteFile(indexPath, data, 0644)
}

// Save writes the project index to disk
func (idx *ProjectIndex) Save() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return idx.saveUnsafe()
}

// Add adds a project to the index
func (idx *ProjectIndex) Add(entry ProjectIndexEntry) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Check for duplicates
	for _, p := range idx.Projects {
		if p.Name == entry.Name {
			return fmt.Errorf("project already exists: %s", entry.Name)
		}
	}

	idx.Projects = append(idx.Projects, entry)
	return idx.saveUnsafe()
}

// Remove removes a project from the index
func (idx *ProjectIndex) Remove(name string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	for i, p := range idx.Projects {
		if p.Name == name {
			idx.Projects = append(idx.Projects[:i], idx.Projects[i+1:]...)
			return idx.saveUnsafe()
		}
	}

	return fmt.Errorf("project not found: %s", name)
}

// Get returns a project entry by name
func (idx *ProjectIndex) Get(name string) (*ProjectIndexEntry, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	for _, p := range idx.Projects {
		if p.Name == name {
			return &p, nil
		}
	}

	return nil, fmt.Errorf("project not found: %s", name)
}

// List returns all projects
func (idx *ProjectIndex) List() []ProjectIndexEntry {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	result := make([]ProjectIndexEntry, len(idx.Projects))
	copy(result, idx.Projects)
	return result
}

// Update updates a project entry
func (idx *ProjectIndex) Update(entry ProjectIndexEntry) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	for i, p := range idx.Projects {
		if p.Name == entry.Name {
			entry.UpdatedAt = time.Now()
			idx.Projects[i] = entry
			return idx.saveUnsafe()
		}
	}

	return fmt.Errorf("project not found: %s", entry.Name)
}

// AddProjectToIndex is a convenience function to add a project to the global index
func AddProjectToIndex(name, sourcePath string) error {
	idx, err := GetProjectIndex()
	if err != nil {
		return err
	}

	now := time.Now()
	return idx.Add(ProjectIndexEntry{
		Name:       name,
		SourcePath: sourcePath,
		CreatedAt:  now,
		UpdatedAt:  now,
	})
}

// RemoveProjectFromIndex is a convenience function to remove a project from the global index
func RemoveProjectFromIndex(name string) error {
	idx, err := GetProjectIndex()
	if err != nil {
		return err
	}

	return idx.Remove(name)
}

// ListProjects returns all projects from the global index
func ListProjects() ([]ProjectIndexEntry, error) {
	idx, err := GetProjectIndex()
	if err != nil {
		return nil, err
	}

	return idx.List(), nil
}

// GetProject returns a project by name from the global index
func GetProject(name string) (*ProjectIndexEntry, error) {
	idx, err := GetProjectIndex()
	if err != nil {
		return nil, err
	}

	return idx.Get(name)
}
