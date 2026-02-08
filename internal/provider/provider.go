// Package provider defines the interface for LLM providers
package provider

import (
	"context"
)

// Provider is the interface that all LLM providers must implement
type Provider interface {
	// ID returns the unique identifier for this provider
	ID() string

	// Name returns the human-readable name
	Name() string

	// Chat sends a chat request and returns the response
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)

	// Stream sends a chat request and streams the response
	Stream(ctx context.Context, req *ChatRequest) (<-chan StreamChunk, error)

	// ListModels returns available models for this provider
	ListModels(ctx context.Context) ([]ModelInfo, error)

	// Ping tests connectivity to the provider
	Ping(ctx context.Context) error
}

// ChatRequest represents a chat completion request
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Tools       []Tool    `json:"tools,omitempty"`
	Temperature *float64  `json:"temperature,omitempty"`
	MaxTokens   *int      `json:"max_tokens,omitempty"`
	Stop        []string  `json:"stop,omitempty"`
}

// Message represents a chat message
type Message struct {
	Role       string     `json:"role"` // system, user, assistant, tool
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// Tool represents a tool/function that can be called
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// ToolCall represents a tool call request from the model
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// ChatResponse represents a chat completion response
type ChatResponse struct {
	Content      string     `json:"content"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	Model        string     `json:"model"`
	Provider     string     `json:"provider"`
	Usage        Usage      `json:"usage"`
	FinishReason string     `json:"finish_reason"`
}

// StreamChunk represents a chunk of streamed response
type StreamChunk struct {
	Content      string     `json:"content,omitempty"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	Done         bool       `json:"done"`
	FinishReason string     `json:"finish_reason,omitempty"`
	Error        error      `json:"-"`
}

// Usage represents token usage
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ModelInfo represents information about a model
type ModelInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	MaxTokens   int    `json:"max_tokens,omitempty"`
	Description string `json:"description,omitempty"`
}

// Config holds provider configuration
type Config struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
	APIKey   string `yaml:"api_key,omitempty"`
	BaseURL  string `yaml:"base_url,omitempty"`
}
