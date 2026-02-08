package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Common errors
var (
	ErrFileNotFound     = errors.New("file not found")
	ErrAccessDenied     = errors.New("access denied")
	ErrOutsideSandbox   = errors.New("path is outside sandbox")
	ErrWriteNotAllowed  = errors.New("write operation not allowed on this path")
)

// Sandbox provides isolated file access for agents
type Sandbox struct {
	sandboxPath string   // Agent's writable sandbox directory
	sourcePath  string   // Primary source code location (read-only for agents)
	sharedPaths []string // Additional readable paths
	isolation   string   // Isolation mode
}

// NewSandbox creates a new sandbox instance
func NewSandbox(sandboxPath, sourcePath string, sharedPaths []string, isolation string) *Sandbox {
	return &Sandbox{
		sandboxPath: sandboxPath,
		sourcePath:  sourcePath,
		sharedPaths: sharedPaths,
		isolation:   isolation,
	}
}

// SandboxPath returns the sandbox directory path
func (s *Sandbox) SandboxPath() string {
	return s.sandboxPath
}

// SourcePath returns the source code directory path
func (s *Sandbox) SourcePath() string {
	return s.sourcePath
}

// ResolvePath resolves a path for the given operation
// For write operations: only sandbox is writable
// For read operations: checks sandbox first, then source, then shared paths
func (s *Sandbox) ResolvePath(op, path string) (string, error) {
	// If isolation is none, allow direct access
	if s.isolation == IsolationNone {
		return path, nil
	}

	// Handle absolute paths - convert to relative if within known paths
	if filepath.IsAbs(path) {
		path = s.toRelativePath(path)
	}

	// Prevent path traversal
	cleanPath := filepath.Clean(path)
	if strings.HasPrefix(cleanPath, "..") {
		return "", ErrOutsideSandbox
	}

	if op == "write" || op == "delete" || op == "create" {
		return s.resolveWritePath(cleanPath)
	}

	return s.resolveReadPath(cleanPath)
}

// resolveWritePath resolves a path for write operations
func (s *Sandbox) resolveWritePath(path string) (string, error) {
	// Writes always go to sandbox
	sandboxFullPath := filepath.Join(s.sandboxPath, path)

	// Ensure parent directory exists
	parentDir := filepath.Dir(sandboxFullPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return "", err
	}

	return sandboxFullPath, nil
}

// resolveReadPath resolves a path for read operations
func (s *Sandbox) resolveReadPath(path string) (string, error) {
	// Check sandbox first
	sandboxFullPath := filepath.Join(s.sandboxPath, path)
	if exists(sandboxFullPath) {
		return sandboxFullPath, nil
	}

	// Check source path
	sourceFullPath := filepath.Join(s.sourcePath, path)
	if exists(sourceFullPath) {
		return sourceFullPath, nil
	}

	// Check shared paths
	for _, shared := range s.sharedPaths {
		sharedFullPath := filepath.Join(shared, path)
		if exists(sharedFullPath) {
			return sharedFullPath, nil
		}
	}

	// File not found - return sandbox path for error consistency
	return sandboxFullPath, ErrFileNotFound
}

// ResolveAbsolutePath resolves an absolute path, checking if it's within allowed paths
func (s *Sandbox) ResolveAbsolutePath(op, absPath string) (string, error) {
	// If isolation is none, allow direct access
	if s.isolation == IsolationNone {
		return absPath, nil
	}

	// Check if path is within sandbox
	if strings.HasPrefix(absPath, s.sandboxPath) {
		// Allow read/write within sandbox
		return absPath, nil
	}

	// Check if path is within source (read-only)
	if strings.HasPrefix(absPath, s.sourcePath) {
		if op == "write" || op == "delete" || op == "create" {
			return "", ErrWriteNotAllowed
		}
		return absPath, nil
	}

	// Check if path is within shared paths (read-only)
	for _, shared := range s.sharedPaths {
		if strings.HasPrefix(absPath, shared) {
			if op == "write" || op == "delete" || op == "create" {
				return "", ErrWriteNotAllowed
			}
			return absPath, nil
		}
	}

	return "", ErrAccessDenied
}

// IsReadable checks if a path can be read
func (s *Sandbox) IsReadable(path string) bool {
	resolved, err := s.ResolvePath("read", path)
	if err != nil {
		return false
	}
	return exists(resolved)
}

// IsWritable checks if a path can be written to
func (s *Sandbox) IsWritable(path string) bool {
	_, err := s.ResolvePath("write", path)
	return err == nil
}

// ListAllowedPaths returns all paths the agent can read from
func (s *Sandbox) ListAllowedPaths() []string {
	paths := []string{s.sandboxPath, s.sourcePath}
	paths = append(paths, s.sharedPaths...)
	return paths
}

// toRelativePath converts an absolute path to relative if it's within known paths
func (s *Sandbox) toRelativePath(absPath string) string {
	// Try sandbox first
	if strings.HasPrefix(absPath, s.sandboxPath) {
		rel, err := filepath.Rel(s.sandboxPath, absPath)
		if err == nil {
			return rel
		}
	}

	// Try source
	if strings.HasPrefix(absPath, s.sourcePath) {
		rel, err := filepath.Rel(s.sourcePath, absPath)
		if err == nil {
			return rel
		}
	}

	// Try shared paths
	for _, shared := range s.sharedPaths {
		if strings.HasPrefix(absPath, shared) {
			rel, err := filepath.Rel(shared, absPath)
			if err == nil {
				return rel
			}
		}
	}

	// Return original path if not within known paths
	return absPath
}

// exists checks if a path exists
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
