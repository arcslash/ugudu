// Package team provides the team orchestration system
package team

import (
	"time"

	"github.com/arcslash/ugudu/internal/provider"
)

// TokenMode controls token consumption level
type TokenMode string

const (
	TokenModeNormal  TokenMode = "normal"  // Full prompts, large context
	TokenModeLow     TokenMode = "low"     // Condensed prompts, reduced context
	TokenModeMinimal TokenMode = "minimal" // Bare minimum for basic function
)

// TokenSettings configures token consumption behavior
type TokenSettings struct {
	Mode           TokenMode `yaml:"mode,omitempty"`            // normal, low, minimal
	MaxTokens      int       `yaml:"max_tokens,omitempty"`      // Override max tokens (0 = use default)
	ContextHistory int       `yaml:"context_history,omitempty"` // Number of messages to keep (0 = use default)
}

// TeamSpec defines a team from YAML configuration
type TeamSpec struct {
	APIVersion   string            `yaml:"apiVersion"`
	Kind         string            `yaml:"kind"`
	Metadata     Metadata          `yaml:"metadata"`
	ClientFacing []string          `yaml:"client_facing,omitempty"`
	Roles        map[string]Role   `yaml:"roles"`
	Workflow     WorkflowSpec      `yaml:"workflow,omitempty"`
	Shared       SharedResources   `yaml:"shared,omitempty"`
	Settings     TeamSettings      `yaml:"settings,omitempty"`
}

// TeamSettings contains runtime settings for the team
type TeamSettings struct {
	Token TokenSettings `yaml:"token,omitempty"`
}

// Metadata contains team metadata
type Metadata struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
}

// Role defines a team member role
type Role struct {
	Title            string         `yaml:"title"`
	Name             string         `yaml:"name,omitempty"`             // Personal name (e.g., "Alice")
	Names            []string       `yaml:"names,omitempty"`            // Names for multiple instances (e.g., ["Alice", "Bob"])
	Count            int            `yaml:"count,omitempty"`            // Number of members with this role (default 1)
	Visibility       string         `yaml:"visibility,omitempty"`       // "client" or "internal"
	Model            ModelConfig    `yaml:"model"`
	Persona          string         `yaml:"persona"`
	PersonaCondensed string         `yaml:"persona_condensed,omitempty"` // Shorter persona for low token mode
	Responsibilities []string       `yaml:"responsibilities,omitempty"`
	Skills           []string       `yaml:"skills,omitempty"`
	Tools            []ToolConfig   `yaml:"tools,omitempty"`
	ReportsTo        string         `yaml:"reports_to,omitempty"`
	CanDelegate      []string       `yaml:"can_delegate,omitempty"` // Roles this role can delegate to
}

// ModelConfig specifies which model to use
type ModelConfig struct {
	Provider      string        `yaml:"provider"`
	Model         string        `yaml:"model"`
	Temperature   *float64      `yaml:"temperature,omitempty"`
	MaxTokens     *int          `yaml:"max_tokens,omitempty"`
	Fallback      []ModelConfig `yaml:"fallback,omitempty"`
	LowTokenModel string        `yaml:"low_token_model,omitempty"` // Cheaper model for low token mode
}

// ToolConfig defines a tool available to a role
type ToolConfig struct {
	Name        string                 `yaml:"name"`
	Type        string                 `yaml:"type,omitempty"`        // "builtin", "http", "command"
	Description string                 `yaml:"description,omitempty"`
	Endpoint    string                 `yaml:"endpoint,omitempty"`
	Command     string                 `yaml:"command,omitempty"`
	Parameters  map[string]interface{} `yaml:"parameters,omitempty"`
}

// WorkflowSpec defines how the team works together
type WorkflowSpec struct {
	Pattern     string        `yaml:"pattern,omitempty"`    // "hub-spoke", "pipeline", "collaborative"
	Stages      []StageSpec   `yaml:"stages,omitempty"`
	AutoAssign  bool          `yaml:"auto_assign,omitempty"`
}

// StageSpec defines a workflow stage
type StageSpec struct {
	Name   string `yaml:"name"`
	Owner  string `yaml:"owner"`   // Role name
	Input  string `yaml:"input,omitempty"`
	Output string `yaml:"output,omitempty"`
}

// SharedResources defines shared team resources
type SharedResources struct {
	Memory MemoryConfig `yaml:"memory,omitempty"`
	Tools  []ToolConfig `yaml:"tools,omitempty"`
}

// MemoryConfig defines team memory/state storage
type MemoryConfig struct {
	Type      string `yaml:"type,omitempty"`   // "sqlite", "postgres", "redis"
	URL       string `yaml:"url,omitempty"`
	Retention string `yaml:"retention,omitempty"`
}

// Task represents a unit of work
type Task struct {
	ID          string                 `json:"id"`
	Content     string                 `json:"content"`
	From        string                 `json:"from"`         // "client" or member ID
	To          string                 `json:"to"`           // Role or member ID
	Status      TaskStatus             `json:"status"`
	Priority    int                    `json:"priority"`
	Context     []provider.Message     `json:"context"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Result      *TaskResult            `json:"result,omitempty"`
	ResultChan  chan *TaskResult       `json:"-"` // Channel for async result delivery
}

// TaskStatus represents task state
type TaskStatus string

const (
	TaskPending    TaskStatus = "pending"
	TaskAssigned   TaskStatus = "assigned"
	TaskInProgress TaskStatus = "in_progress"
	TaskCompleted  TaskStatus = "completed"
	TaskFailed     TaskStatus = "failed"
	TaskBlocked    TaskStatus = "blocked"
)

// TaskResult holds the output of a completed task
type TaskResult struct {
	Content   string                 `json:"content"`
	Success   bool                   `json:"success"`
	Error     string                 `json:"error,omitempty"`
	Artifacts map[string]interface{} `json:"artifacts,omitempty"`
}

// Message represents internal team communication
type Message struct {
	ID        string         `json:"id"`
	Type      MessageType    `json:"type"`
	From      string         `json:"from"`
	To        string         `json:"to"`
	Content   interface{}    `json:"content"`
	TaskID    string         `json:"task_id,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// MessageType identifies the kind of message
type MessageType string

const (
	MsgClientRequest   MessageType = "client_request"
	MsgClientResponse  MessageType = "client_response"
	MsgTaskAssignment  MessageType = "task_assignment"
	MsgTaskUpdate      MessageType = "task_update"
	MsgTaskComplete    MessageType = "task_complete"
	MsgQuestion        MessageType = "question"
	MsgAnswer          MessageType = "answer"
	MsgDelegation      MessageType = "delegation"
	MsgReport          MessageType = "report"
	MsgHeartbeat       MessageType = "heartbeat"
)

// MemberStatus represents the current state of a team member
type MemberStatus string

const (
	MemberIdle     MemberStatus = "idle"
	MemberWorking  MemberStatus = "working"
	MemberWaiting  MemberStatus = "waiting"
	MemberBlocked  MemberStatus = "blocked"
	MemberOffline  MemberStatus = "offline"
)
