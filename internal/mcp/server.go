// Package mcp implements the Model Context Protocol server for Ugudu
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/arcslash/ugudu/internal/daemon"
	"github.com/arcslash/ugudu/internal/logger"
)

// Server implements an MCP server for Ugudu
type Server struct {
	client  *daemon.Client
	logger  *logger.Logger
	tools   map[string]Tool
	running bool
	mu      sync.RWMutex
}

// Tool represents an MCP tool
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	Handler     func(context.Context, map[string]interface{}) (interface{}, error)
}

// Message represents an MCP JSON-RPC message
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// NewServer creates a new MCP server
func NewServer(log *logger.Logger) (*Server, error) {
	// Try to connect to daemon
	client, err := daemon.NewClient("")
	if err != nil {
		// Daemon not running - will try to connect later
		log.Warn("daemon not running, some tools will be unavailable")
	}

	s := &Server{
		client: client,
		logger: log,
		tools:  make(map[string]Tool),
	}

	s.registerTools()
	return s, nil
}

func (s *Server) registerTools() {
	// Team management tools
	s.tools["ugudu_list_teams"] = Tool{
		Name:        "ugudu_list_teams",
		Description: "List all AI teams currently created in Ugudu",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: s.handleListTeams,
	}

	s.tools["ugudu_create_team"] = Tool{
		Name:        "ugudu_create_team",
		Description: "Create a new AI team from a spec. Use ugudu_list_specs to see available specs.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Name for the new team instance",
				},
				"spec": map[string]interface{}{
					"type":        "string",
					"description": "Name of the spec to use (from ugudu_list_specs)",
				},
			},
			"required": []string{"name", "spec"},
		},
		Handler: s.handleCreateTeam,
	}

	s.tools["ugudu_ask"] = Tool{
		Name:        "ugudu_ask",
		Description: "Send a message to an AI team and get a response. The message goes to the team's client-facing member (usually a PM or lead).",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"team": map[string]interface{}{
					"type":        "string",
					"description": "Name of the team to message",
				},
				"message": map[string]interface{}{
					"type":        "string",
					"description": "The message to send to the team",
				},
			},
			"required": []string{"team", "message"},
		},
		Handler: s.handleAsk,
	}

	s.tools["ugudu_team_status"] = Tool{
		Name:        "ugudu_team_status",
		Description: "Get the status of a specific team including its members and their current state",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"team": map[string]interface{}{
					"type":        "string",
					"description": "Name of the team",
				},
			},
			"required": []string{"team"},
		},
		Handler: s.handleTeamStatus,
	}

	s.tools["ugudu_list_specs"] = Tool{
		Name:        "ugudu_list_specs",
		Description: "List available team specifications (blueprints) that can be used to create teams",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: s.handleListSpecs,
	}

	s.tools["ugudu_daemon_status"] = Tool{
		Name:        "ugudu_daemon_status",
		Description: "Check if the Ugudu daemon is running and get its status",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: s.handleDaemonStatus,
	}

	s.tools["ugudu_start_team"] = Tool{
		Name:        "ugudu_start_team",
		Description: "Start a stopped team",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"team": map[string]interface{}{
					"type":        "string",
					"description": "Name of the team to start",
				},
			},
			"required": []string{"team"},
		},
		Handler: s.handleStartTeam,
	}

	s.tools["ugudu_stop_team"] = Tool{
		Name:        "ugudu_stop_team",
		Description: "Stop a running team",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"team": map[string]interface{}{
					"type":        "string",
					"description": "Name of the team to stop",
				},
			},
			"required": []string{"team"},
		},
		Handler: s.handleStopTeam,
	}

	s.tools["ugudu_delete_team"] = Tool{
		Name:        "ugudu_delete_team",
		Description: "Delete a team permanently",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"team": map[string]interface{}{
					"type":        "string",
					"description": "Name of the team to delete",
				},
			},
			"required": []string{"team"},
		},
		Handler: s.handleDeleteTeam,
	}

	// Project workflow tools
	s.tools["ugudu_start_project"] = Tool{
		Name:        "ugudu_start_project",
		Description: "Start a new project with a team. The PM and BA will collaborate to create requirements and break them into stories for engineers.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"team": map[string]interface{}{
					"type":        "string",
					"description": "Name of the team to work on the project",
				},
				"request": map[string]interface{}{
					"type":        "string",
					"description": "Your project request/requirements in detail",
				},
			},
			"required": []string{"team", "request"},
		},
		Handler: s.handleStartProject,
	}

	s.tools["ugudu_project_status"] = Tool{
		Name:        "ugudu_project_status",
		Description: "Get the current status of a team's active project including phase, stories, and progress",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"team": map[string]interface{}{
					"type":        "string",
					"description": "Name of the team",
				},
			},
			"required": []string{"team"},
		},
		Handler: s.handleProjectStatus,
	}

	s.tools["ugudu_pending_questions"] = Tool{
		Name:        "ugudu_pending_questions",
		Description: "Get questions from the team that need your (the client's) response",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"team": map[string]interface{}{
					"type":        "string",
					"description": "Name of the team",
				},
			},
			"required": []string{"team"},
		},
		Handler: s.handlePendingQuestions,
	}

	s.tools["ugudu_answer_question"] = Tool{
		Name:        "ugudu_answer_question",
		Description: "Answer a question from the team. Use ugudu_pending_questions to see questions first.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"team": map[string]interface{}{
					"type":        "string",
					"description": "Name of the team",
				},
				"question_id": map[string]interface{}{
					"type":        "string",
					"description": "ID of the question to answer",
				},
				"answer": map[string]interface{}{
					"type":        "string",
					"description": "Your answer to the question",
				},
			},
			"required": []string{"team", "question_id", "answer"},
		},
		Handler: s.handleAnswerQuestion,
	}

	s.tools["ugudu_project_communications"] = Tool{
		Name:        "ugudu_project_communications",
		Description: "View the communication log between team members on a project",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"team": map[string]interface{}{
					"type":        "string",
					"description": "Name of the team",
				},
				"limit": map[string]interface{}{
					"type":        "number",
					"description": "Maximum number of messages to return (default: 20)",
				},
			},
			"required": []string{"team"},
		},
		Handler: s.handleProjectCommunications,
	}

	// Token mode control
	s.tools["ugudu_set_token_mode"] = Tool{
		Name:        "ugudu_set_token_mode",
		Description: "Set the token consumption mode for a team. Use 'low' or 'minimal' to reduce API costs, 'normal' for full capabilities.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"team": map[string]interface{}{
					"type":        "string",
					"description": "Name of the team",
				},
				"mode": map[string]interface{}{
					"type":        "string",
					"description": "Token mode: 'normal' (default), 'low' (reduced prompts/context), or 'minimal' (bare minimum)",
					"enum":        []string{"normal", "low", "minimal"},
				},
			},
			"required": []string{"team", "mode"},
		},
		Handler: s.handleSetTokenMode,
	}

	// Conversation management
	s.tools["ugudu_clear_conversation"] = Tool{
		Name:        "ugudu_clear_conversation",
		Description: "Clear conversation history for a team to start fresh",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"team": map[string]interface{}{
					"type":        "string",
					"description": "Name of the team",
				},
			},
			"required": []string{"team"},
		},
		Handler: s.handleClearConversation,
	}

	// Specialist templates
	s.tools["ugudu_list_specialists"] = Tool{
		Name:        "ugudu_list_specialists",
		Description: "List available specialist templates that can be added to teams (e.g., Healthcare SME, DevOps, Security)",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: s.handleListSpecialists,
	}
}

// Run starts the MCP server on stdin/stdout
func (s *Server) Run() error {
	s.mu.Lock()
	s.running = true
	s.mu.Unlock()

	reader := bufio.NewReader(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	s.logger.Info("MCP server started")

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("read error: %w", err)
		}

		var msg Message
		if err := json.Unmarshal(line, &msg); err != nil {
			s.logger.Error("failed to parse message", "error", err)
			continue
		}

		response := s.handleMessage(msg)
		if response != nil {
			if err := encoder.Encode(response); err != nil {
				s.logger.Error("failed to send response", "error", err)
			}
		}
	}

	return nil
}

func (s *Server) handleMessage(msg Message) *Message {
	switch msg.Method {
	case "initialize":
		return s.handleInitialize(msg)
	case "tools/list":
		return s.handleToolsList(msg)
	case "tools/call":
		return s.handleToolsCall(msg)
	case "notifications/initialized":
		// No response needed for notifications
		return nil
	default:
		s.logger.Debug("unknown method", "method", msg.Method)
		return &Message{
			JSONRPC: "2.0",
			ID:      msg.ID,
			Error: &RPCError{
				Code:    -32601,
				Message: "Method not found",
			},
		}
	}
}

func (s *Server) handleInitialize(msg Message) *Message {
	return &Message{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "ugudu",
				"version": "0.1.0",
			},
		},
	}
}

func (s *Server) handleToolsList(msg Message) *Message {
	tools := make([]map[string]interface{}, 0, len(s.tools))
	for _, tool := range s.tools {
		tools = append(tools, map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": tool.InputSchema,
		})
	}

	return &Message{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result: map[string]interface{}{
			"tools": tools,
		},
	}
}

func (s *Server) handleToolsCall(msg Message) *Message {
	var params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}

	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return &Message{
			JSONRPC: "2.0",
			ID:      msg.ID,
			Error: &RPCError{
				Code:    -32602,
				Message: "Invalid params",
			},
		}
	}

	tool, ok := s.tools[params.Name]
	if !ok {
		return &Message{
			JSONRPC: "2.0",
			ID:      msg.ID,
			Error: &RPCError{
				Code:    -32602,
				Message: fmt.Sprintf("Unknown tool: %s", params.Name),
			},
		}
	}

	ctx := context.Background()
	result, err := tool.Handler(ctx, params.Arguments)
	if err != nil {
		return &Message{
			JSONRPC: "2.0",
			ID:      msg.ID,
			Result: map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": fmt.Sprintf("Error: %v", err),
					},
				},
				"isError": true,
			},
		}
	}

	// Format result as text content
	var text string
	switch v := result.(type) {
	case string:
		text = v
	default:
		jsonBytes, _ := json.MarshalIndent(result, "", "  ")
		text = string(jsonBytes)
	}

	return &Message{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": text,
				},
			},
		},
	}
}

// ensureClient ensures we have a connection to the daemon
func (s *Server) ensureClient() error {
	if s.client != nil {
		// Test connection
		ctx := context.Background()
		if err := s.client.Ping(ctx); err == nil {
			return nil
		}
	}

	// Try to reconnect
	client, err := daemon.NewClient("")
	if err != nil {
		return fmt.Errorf("daemon not running. Start it with: ugudu daemon")
	}
	s.client = client
	return nil
}
