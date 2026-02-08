package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/arcslash/ugudu/internal/workspace"
)

// SandboxedRegistry wraps a tool registry with sandbox enforcement and role-based access
type SandboxedRegistry struct {
	base      *Registry
	sandbox   *workspace.Sandbox
	role      string
	agentID   string
	workspace *workspace.Workspace

	// Activity logging callback
	OnToolExecute func(toolName string, args map[string]interface{}, result interface{}, err error)
}

// NewSandboxedRegistry creates a new sandboxed registry
func NewSandboxedRegistry(base *Registry, ws *workspace.Workspace, role, agentID string) *SandboxedRegistry {
	var sandbox *workspace.Sandbox
	if ws != nil {
		sandbox = ws.GetSandbox(role)
	}

	return &SandboxedRegistry{
		base:      base,
		sandbox:   sandbox,
		role:      role,
		agentID:   agentID,
		workspace: ws,
	}
}

// Get returns a tool by name if the role has access
func (r *SandboxedRegistry) Get(name string) (Tool, bool) {
	// Check role permission
	if !IsToolAllowedForRole(name, r.role) {
		return nil, false
	}

	return r.base.Get(name)
}

// List returns all tools available to the role
func (r *SandboxedRegistry) List() []Tool {
	allTools := r.base.List()
	var allowed []Tool

	for _, tool := range allTools {
		if IsToolAllowedForRole(tool.Name(), r.role) {
			allowed = append(allowed, tool)
		}
	}

	return allowed
}

// Execute runs a tool with sandbox enforcement
func (r *SandboxedRegistry) Execute(ctx context.Context, name string, args map[string]interface{}) (interface{}, error) {
	// Check role permission
	if !IsToolAllowedForRole(name, r.role) {
		err := fmt.Errorf("tool %s is not available for role %s", name, r.role)
		if r.OnToolExecute != nil {
			r.OnToolExecute(name, args, nil, err)
		}
		return nil, err
	}

	// Apply sandbox path resolution for file operations
	if r.sandbox != nil {
		args = r.sandboxArgs(name, args)
	}

	// Execute the tool
	result, err := r.base.Execute(ctx, name, args)

	// Log the execution
	if r.OnToolExecute != nil {
		r.OnToolExecute(name, args, result, err)
	}

	return result, err
}

// sandboxArgs applies sandbox path resolution to tool arguments
func (r *SandboxedRegistry) sandboxArgs(toolName string, args map[string]interface{}) map[string]interface{} {
	// Clone args to avoid mutation
	newArgs := make(map[string]interface{})
	for k, v := range args {
		newArgs[k] = v
	}

	// Determine operation type based on tool
	op := r.getOperationType(toolName)

	// Resolve path arguments
	pathKeys := []string{"path", "file", "directory", "root", "target"}
	for _, key := range pathKeys {
		if path, ok := newArgs[key].(string); ok && path != "" {
			resolved, err := r.resolvePath(op, path)
			if err == nil {
				newArgs[key] = resolved
			}
		}
	}

	return newArgs
}

// getOperationType determines if a tool performs read or write operations
func (r *SandboxedRegistry) getOperationType(toolName string) string {
	writeTools := map[string]bool{
		"write_file":         true,
		"edit_file":          true,
		"git_commit":         true,
		"create_task":        true,
		"update_task":        true,
		"create_report":      true,
		"create_doc":         true,
		"create_requirement": true,
		"create_spec":        true,
		"create_bug_report":  true,
	}

	if writeTools[toolName] {
		return "write"
	}
	return "read"
}

// resolvePath resolves a path through the sandbox
func (r *SandboxedRegistry) resolvePath(op, path string) (string, error) {
	if r.sandbox == nil {
		return path, nil
	}

	// Handle absolute paths
	if filepath.IsAbs(path) {
		return r.sandbox.ResolveAbsolutePath(op, path)
	}

	return r.sandbox.ResolvePath(op, path)
}

// RegisterRoleTools registers role-specific tools with proper configuration
func (r *SandboxedRegistry) RegisterRoleTools() {
	if r.workspace == nil {
		return
	}

	artifactPath := r.workspace.ArtifactPath("")
	tasksPath := r.workspace.TasksPath()
	sourcePath := ""
	if r.workspace.Config != nil {
		sourcePath = r.workspace.Config.Source.Path
	}

	taskStore := NewFileTaskStore(tasksPath)

	// Register planning tools
	r.base.Register(&CreateTaskTool{Store: taskStore, CreatedBy: r.agentID})
	r.base.Register(&UpdateTaskTool{Store: taskStore, UpdatedBy: r.agentID})
	r.base.Register(&ListTasksTool{Store: taskStore})
	r.base.Register(&AssignTaskTool{Store: taskStore, AssignedBy: r.agentID})
	r.base.Register(&CreateReportTool{ArtifactPath: artifactPath, CreatedBy: r.agentID})
	r.base.Register(&DelegateTaskTool{Store: taskStore, DelegatedBy: r.agentID})

	// Register testing tools
	r.base.Register(&RunTestsTool{WorkingDir: sourcePath, ArtifactPath: artifactPath})
	r.base.Register(&CreateBugReportTool{ArtifactPath: artifactPath, ReportedBy: r.agentID})
	r.base.Register(&VerifyFixTool{ArtifactPath: artifactPath, VerifiedBy: r.agentID})
	r.base.Register(&ListTestResultsTool{ArtifactPath: artifactPath})

	// Register documentation tools
	r.base.Register(&CreateDocTool{ArtifactPath: artifactPath, Author: r.agentID})
	r.base.Register(&CreateRequirementTool{ArtifactPath: artifactPath, Author: r.agentID})
	r.base.Register(&CreateSpecTool{ArtifactPath: artifactPath, Author: r.agentID})

	// Register git tools
	r.base.Register(&GitStatusTool{WorkingDir: sourcePath})
	r.base.Register(&GitDiffTool{WorkingDir: sourcePath})
	r.base.Register(&GitCommitTool{WorkingDir: sourcePath, Author: r.agentID})
	r.base.Register(&GitLogTool{WorkingDir: sourcePath})
	r.base.Register(&GitBranchTool{WorkingDir: sourcePath})
}

// SetCommunicationFuncs sets the communication callbacks for ask_colleague and report_progress
func (r *SandboxedRegistry) SetCommunicationFuncs(askFunc ColleagueFunc, progressFunc ProgressFunc) {
	r.base.Register(&AskColleagueTool{FromRole: r.role, AskFunc: askFunc})
	r.base.Register(&ReportProgressTool{FromRole: r.role, ReportFunc: progressFunc})
}

// GetAllowedToolNames returns the names of tools available to the role
func (r *SandboxedRegistry) GetAllowedToolNames() []string {
	tools := r.List()
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name()
	}
	return names
}

// FormatToolsForPrompt returns a formatted string of available tools for the agent's prompt
func (r *SandboxedRegistry) FormatToolsForPrompt() string {
	tools := r.List()
	if len(tools) == 0 {
		return "No tools available."
	}

	var sb strings.Builder
	sb.WriteString("Available tools:\n")
	for _, tool := range tools {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", tool.Name(), tool.Description()))
	}
	return sb.String()
}

// ListToolNames returns a comma-separated list of tool names (for condensed prompts)
func (r *SandboxedRegistry) ListToolNames() string {
	tools := r.List()
	if len(tools) == 0 {
		return "none"
	}

	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.Name()
	}
	return strings.Join(names, ", ")
}
