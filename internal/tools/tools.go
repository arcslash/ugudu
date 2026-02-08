// Package tools provides built-in tools for agents
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Tool is the interface for executable tools
type Tool interface {
	Name() string
	Description() string
	Execute(ctx context.Context, args map[string]interface{}) (interface{}, error)
}

// Registry holds available tools
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates a tool registry with built-in tools
func NewRegistry() *Registry {
	r := &Registry{
		tools: make(map[string]Tool),
	}

	// Register built-in tools
	r.Register(&ReadFileTool{})
	r.Register(&WriteFileTool{})
	r.Register(&EditFileTool{})
	r.Register(&ListFilesTool{})
	r.Register(&RunCommandTool{})
	r.Register(&HTTPRequestTool{})
	r.Register(&SearchFilesTool{})

	return r
}

// Register adds a tool to the registry
func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
}

// Get returns a tool by name
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// List returns all registered tools
func (r *Registry) List() []Tool {
	tools := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	return tools
}

// Execute runs a tool by name
func (r *Registry) Execute(ctx context.Context, name string, args map[string]interface{}) (interface{}, error) {
	t, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}
	return t.Execute(ctx, args)
}

// ============================================================================
// Built-in Tools
// ============================================================================

// ReadFileTool reads file contents
type ReadFileTool struct{}

func (t *ReadFileTool) Name() string        { return "read_file" }
func (t *ReadFileTool) Description() string { return "Read the contents of a file" }

func (t *ReadFileTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	path, ok := args["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path is required")
	}

	// Resolve to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	return map[string]interface{}{
		"path":    absPath,
		"content": string(content),
		"size":    len(content),
	}, nil
}

// WriteFileTool writes content to a file
type WriteFileTool struct{}

func (t *WriteFileTool) Name() string        { return "write_file" }
func (t *WriteFileTool) Description() string { return "Write content to a file" }

func (t *WriteFileTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	path, ok := args["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path is required")
	}

	content, ok := args["content"].(string)
	if !ok {
		return nil, fmt.Errorf("content is required")
	}

	// Resolve path - use projects dir for relative paths
	absPath := resolveSafePath(path)

	// Create parent directories if needed
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}

	return map[string]interface{}{
		"path":    absPath,
		"written": len(content),
	}, nil
}

// resolveSafePath ensures paths are within the projects directory
func resolveSafePath(path string) string {
	// If it's already an absolute path within the home dir, allow it
	if filepath.IsAbs(path) {
		homeDir, _ := os.UserHomeDir()
		// Only allow paths under home directory
		if strings.HasPrefix(path, homeDir) {
			return path
		}
		// Otherwise, strip leading slash and treat as relative
		path = strings.TrimPrefix(path, "/")
	}

	// Get projects directory
	homeDir, _ := os.UserHomeDir()
	projectsDir := filepath.Join(homeDir, "ugudu_projects", "default")

	// Create the projects dir if it doesn't exist
	os.MkdirAll(projectsDir, 0755)

	// Clean the path to prevent directory traversal
	cleanPath := filepath.Clean(path)
	cleanPath = strings.TrimPrefix(cleanPath, "../")
	cleanPath = strings.TrimPrefix(cleanPath, "..")

	return filepath.Join(projectsDir, cleanPath)
}

// EditFileTool edits a file with search/replace
type EditFileTool struct{}

func (t *EditFileTool) Name() string        { return "edit_file" }
func (t *EditFileTool) Description() string { return "Edit a file by replacing text" }

func (t *EditFileTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	pathArg, ok := args["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path is required")
	}

	oldText, ok := args["old_text"].(string)
	if !ok {
		return nil, fmt.Errorf("old_text is required")
	}

	newText, ok := args["new_text"].(string)
	if !ok {
		return nil, fmt.Errorf("new_text is required")
	}

	// Resolve path to safe location
	absPath := resolveSafePath(pathArg)

	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	if !strings.Contains(string(content), oldText) {
		return nil, fmt.Errorf("old_text not found in file")
	}

	newContent := strings.Replace(string(content), oldText, newText, 1)

	if err := os.WriteFile(absPath, []byte(newContent), 0644); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}

	return map[string]interface{}{
		"path":     absPath,
		"replaced": true,
	}, nil
}

// ListFilesTool lists files in a directory
type ListFilesTool struct{}

func (t *ListFilesTool) Name() string        { return "list_files" }
func (t *ListFilesTool) Description() string { return "List files in a directory" }

func (t *ListFilesTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	path, ok := args["path"].(string)
	if !ok {
		path = "."
	}

	// Use safe path resolution for relative paths
	var absPath string
	if path == "." || !filepath.IsAbs(path) {
		absPath = resolveSafePath(path)
	} else {
		absPath = path
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}

	files := make([]map[string]interface{}, 0, len(entries))
	for _, entry := range entries {
		info, _ := entry.Info()
		file := map[string]interface{}{
			"name":  entry.Name(),
			"isDir": entry.IsDir(),
		}
		if info != nil {
			file["size"] = info.Size()
			file["modified"] = info.ModTime().Format(time.RFC3339)
		}
		files = append(files, file)
	}

	return map[string]interface{}{
		"path":  absPath,
		"files": files,
		"count": len(files),
	}, nil
}

// RunCommandTool executes shell commands
type RunCommandTool struct{}

func (t *RunCommandTool) Name() string        { return "run_command" }
func (t *RunCommandTool) Description() string { return "Execute a shell command" }

func (t *RunCommandTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	command, ok := args["command"].(string)
	if !ok {
		return nil, fmt.Errorf("command is required")
	}

	// Set timeout
	timeout := 60 * time.Second
	if t, ok := args["timeout"].(float64); ok {
		timeout = time.Duration(t) * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)

	// Set working directory - use safe path resolution
	if dir, ok := args["directory"].(string); ok {
		if dir == "." || !filepath.IsAbs(dir) {
			cmd.Dir = resolveSafePath(dir)
		} else {
			cmd.Dir = dir
		}
	} else {
		// Default to projects directory
		cmd.Dir = resolveSafePath(".")
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := map[string]interface{}{
		"stdout":   stdout.String(),
		"stderr":   stderr.String(),
		"exitCode": 0,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result["exitCode"] = exitErr.ExitCode()
		} else {
			result["error"] = err.Error()
		}
	}

	return result, nil
}

// HTTPRequestTool makes HTTP requests
type HTTPRequestTool struct {
	client *http.Client
}

func (t *HTTPRequestTool) Name() string        { return "http_request" }
func (t *HTTPRequestTool) Description() string { return "Make an HTTP request" }

func (t *HTTPRequestTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	url, ok := args["url"].(string)
	if !ok {
		return nil, fmt.Errorf("url is required")
	}

	method := "GET"
	if m, ok := args["method"].(string); ok {
		method = strings.ToUpper(m)
	}

	var body io.Reader
	if b, ok := args["body"].(string); ok {
		body = strings.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Add headers
	if headers, ok := args["headers"].(map[string]interface{}); ok {
		for k, v := range headers {
			if vs, ok := v.(string); ok {
				req.Header.Set(k, vs)
			}
		}
	}

	if t.client == nil {
		t.client = &http.Client{Timeout: 30 * time.Second}
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Try to parse as JSON
	var jsonBody interface{}
	if err := json.Unmarshal(respBody, &jsonBody); err == nil {
		return map[string]interface{}{
			"status":  resp.StatusCode,
			"headers": resp.Header,
			"body":    jsonBody,
		}, nil
	}

	return map[string]interface{}{
		"status":  resp.StatusCode,
		"headers": resp.Header,
		"body":    string(respBody),
	}, nil
}

// SearchFilesTool searches for files matching a pattern
type SearchFilesTool struct{}

func (t *SearchFilesTool) Name() string        { return "search_files" }
func (t *SearchFilesTool) Description() string { return "Search for files matching a pattern" }

func (t *SearchFilesTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	pattern, ok := args["pattern"].(string)
	if !ok {
		return nil, fmt.Errorf("pattern is required")
	}

	root := "."
	if r, ok := args["root"].(string); ok {
		root = r
	}

	// Use safe path resolution for relative paths
	var absRoot string
	if root == "." || !filepath.IsAbs(root) {
		absRoot = resolveSafePath(root)
	} else {
		absRoot = root
	}

	var matches []string
	err := filepath.Walk(absRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip hidden directories
		if info.IsDir() && strings.HasPrefix(info.Name(), ".") {
			return filepath.SkipDir
		}

		matched, _ := filepath.Match(pattern, info.Name())
		if matched {
			relPath, _ := filepath.Rel(absRoot, path)
			matches = append(matches, relPath)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	return map[string]interface{}{
		"root":    absRoot,
		"pattern": pattern,
		"matches": matches,
		"count":   len(matches),
	}, nil
}
