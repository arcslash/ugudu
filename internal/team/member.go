package team

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/arcslash/ugudu/internal/logger"
	"github.com/arcslash/ugudu/internal/provider"
	"github.com/arcslash/ugudu/internal/tools"
	"github.com/google/uuid"
)

// MaxToolIterations limits how many tool calls can happen in a single request
const MaxToolIterations = 20

// Member represents an individual team member (an agent with a role)
type Member struct {
	ID       string
	Name     string // Personal name (e.g., "Alice")
	RoleName string
	Role     Role
	Team     *Team
	Provider provider.Provider
	Status   MemberStatus
	Task     *Task

	// Tool execution
	toolRegistry *tools.SandboxedRegistry

	inbox           chan Message
	outbox          chan Message
	ctx             context.Context
	cancel          context.CancelFunc
	logger          *logger.Logger
	mu              sync.RWMutex
	conversationMu  sync.RWMutex
	conversationCtx []provider.Message // LLM conversation history
	contextSequence int                // Sequence counter for persistence
}

// NewMember creates a new team member
func NewMember(id, name, roleName string, role Role, team *Team, prov provider.Provider, log *logger.Logger) *Member {
	displayName := name
	if displayName == "" {
		displayName = role.Title
	}
	return &Member{
		ID:              id,
		Name:            displayName,
		RoleName:        roleName,
		Role:            role,
		Team:            team,
		Provider:        prov,
		Status:          MemberIdle,
		inbox:           make(chan Message, 100),
		outbox:          make(chan Message, 100),
		logger:          log.With("member", id, "name", displayName, "role", roleName),
		conversationCtx: make([]provider.Message, 0),
		contextSequence: 0,
	}
}

// DisplayName returns the member's name with title (e.g., "Alice (PM)")
func (m *Member) DisplayName() string {
	if m.Name != "" && m.Name != m.Role.Title {
		return fmt.Sprintf("%s (%s)", m.Name, m.Role.Title)
	}
	return m.Role.Title
}

// SetToolRegistry sets the tool registry for this member
func (m *Member) SetToolRegistry(registry *tools.SandboxedRegistry) {
	m.toolRegistry = registry
}

// getProviderTools converts the tool registry to provider.Tool format
func (m *Member) getProviderTools() []provider.Tool {
	if m.toolRegistry == nil {
		return nil
	}

	registryTools := m.toolRegistry.List()
	providerTools := make([]provider.Tool, 0, len(registryTools))

	for _, t := range registryTools {
		providerTools = append(providerTools, provider.Tool{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  m.getToolParameters(t.Name()),
		})
	}

	return providerTools
}

// getToolParameters returns JSON schema parameters for a tool
func (m *Member) getToolParameters(toolName string) map[string]interface{} {
	// Define parameter schemas for each tool
	schemas := map[string]map[string]interface{}{
		"read_file": {
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the file to read",
				},
			},
			"required": []string{"path"},
		},
		"write_file": {
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the file to write",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "Content to write to the file",
				},
			},
			"required": []string{"path", "content"},
		},
		"edit_file": {
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the file to edit",
				},
				"old_text": map[string]interface{}{
					"type":        "string",
					"description": "Text to find and replace",
				},
				"new_text": map[string]interface{}{
					"type":        "string",
					"description": "Text to replace with",
				},
			},
			"required": []string{"path", "old_text", "new_text"},
		},
		"list_files": {
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Directory path to list (default: current directory)",
				},
			},
		},
		"search_files": {
			"type": "object",
			"properties": map[string]interface{}{
				"pattern": map[string]interface{}{
					"type":        "string",
					"description": "Glob pattern to match files (e.g., *.go)",
				},
				"root": map[string]interface{}{
					"type":        "string",
					"description": "Root directory to search from",
				},
			},
			"required": []string{"pattern"},
		},
		"run_command": {
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "Shell command to execute",
				},
				"directory": map[string]interface{}{
					"type":        "string",
					"description": "Working directory for the command",
				},
				"timeout": map[string]interface{}{
					"type":        "number",
					"description": "Timeout in seconds (default: 60)",
				},
			},
			"required": []string{"command"},
		},
		"git_status": {
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		"git_diff": {
			"type": "object",
			"properties": map[string]interface{}{
				"staged": map[string]interface{}{
					"type":        "boolean",
					"description": "Show staged changes only",
				},
			},
		},
		"git_commit": {
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{
					"type":        "string",
					"description": "Commit message",
				},
				"files": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Files to stage and commit",
				},
			},
			"required": []string{"message"},
		},
		"create_task": {
			"type": "object",
			"properties": map[string]interface{}{
				"title": map[string]interface{}{
					"type":        "string",
					"description": "Task title",
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "Task description",
				},
				"priority": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"low", "medium", "high", "critical"},
					"description": "Task priority",
				},
				"assignee": map[string]interface{}{
					"type":        "string",
					"description": "Role to assign the task to",
				},
			},
			"required": []string{"title", "description"},
		},
		"run_tests": {
			"type": "object",
			"properties": map[string]interface{}{
				"pattern": map[string]interface{}{
					"type":        "string",
					"description": "Test pattern to run (e.g., ./... or specific path)",
				},
				"verbose": map[string]interface{}{
					"type":        "boolean",
					"description": "Enable verbose output",
				},
			},
		},
		"http_request": {
			"type": "object",
			"properties": map[string]interface{}{
				"url": map[string]interface{}{
					"type":        "string",
					"description": "URL to request",
				},
				"method": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
					"description": "HTTP method (default: GET)",
				},
				"body": map[string]interface{}{
					"type":        "string",
					"description": "Request body",
				},
				"headers": map[string]interface{}{
					"type":        "object",
					"description": "Request headers",
				},
			},
			"required": []string{"url"},
		},
	}

	if schema, ok := schemas[toolName]; ok {
		return schema
	}

	// Default schema for unknown tools
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

// executeToolCalls executes tool calls and returns tool result messages
func (m *Member) executeToolCalls(ctx context.Context, toolCalls []provider.ToolCall) []provider.Message {
	results := make([]provider.Message, 0, len(toolCalls))

	for _, tc := range toolCalls {
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
			results = append(results, provider.Message{
				Role:       "tool",
				Content:    fmt.Sprintf("Error parsing arguments: %v", err),
				ToolCallID: tc.ID,
			})
			continue
		}

		m.logger.Info("executing tool", "tool", tc.Name, "args", args)

		// Notify activity about tool execution
		m.Team.NotifyActivity(m.ID, "tool_call", fmt.Sprintf("Using tool: %s", tc.Name))

		result, err := m.toolRegistry.Execute(ctx, tc.Name, args)
		if err != nil {
			m.logger.Error("tool execution failed", "tool", tc.Name, "error", err)
			m.Team.NotifyActivity(m.ID, "tool_error", fmt.Sprintf("Tool %s failed: %s", tc.Name, truncateMessage(err.Error(), 50)))
			results = append(results, provider.Message{
				Role:       "tool",
				Content:    fmt.Sprintf("Error: %v", err),
				ToolCallID: tc.ID,
			})
			continue
		}

		// Format result as JSON
		resultJSON, _ := json.MarshalIndent(result, "", "  ")
		m.logger.Debug("tool result", "tool", tc.Name, "result", string(resultJSON))

		results = append(results, provider.Message{
			Role:       "tool",
			Content:    string(resultJSON),
			ToolCallID: tc.ID,
		})
	}

	return results
}

// RestoreContext restores conversation context from persistence
func (m *Member) RestoreContext(history []ContextMessage) {
	m.conversationMu.Lock()
	defer m.conversationMu.Unlock()

	m.conversationCtx = make([]provider.Message, 0, len(history))
	for _, msg := range history {
		m.conversationCtx = append(m.conversationCtx, provider.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}
	m.contextSequence = len(history)
	m.logger.Debug("context restored", "messages", len(history))
}

// addToContext adds a message to conversation context and persists it
func (m *Member) addToContext(role, content string) {
	m.conversationMu.Lock()
	defer m.conversationMu.Unlock()

	m.conversationCtx = append(m.conversationCtx, provider.Message{
		Role:    role,
		Content: content,
	})
	m.contextSequence++

	// Persist to store
	m.Team.SaveMemberContext(m.ID, role, content, m.contextSequence)

	// Trim context based on token mode (fewer messages in low token mode)
	contextLimit := m.getContextLimit()
	if len(m.conversationCtx) > contextLimit {
		m.conversationCtx = m.conversationCtx[len(m.conversationCtx)-contextLimit:]
	}
}

// getContextMessages returns the current conversation context
func (m *Member) getContextMessages() []provider.Message {
	m.conversationMu.RLock()
	defer m.conversationMu.RUnlock()

	// Return a copy to avoid race conditions
	ctx := make([]provider.Message, len(m.conversationCtx))
	copy(ctx, m.conversationCtx)
	return ctx
}

// ClearContext clears the conversation context (for new conversations)
func (m *Member) ClearContext() {
	m.conversationMu.Lock()
	defer m.conversationMu.Unlock()

	m.conversationCtx = make([]provider.Message, 0)
	m.contextSequence = 0
}

// Start begins the member's processing loop
func (m *Member) Start(ctx context.Context) {
	m.ctx, m.cancel = context.WithCancel(ctx)
	go m.run()
	m.logger.Info("member started")
}

// Stop halts the member
func (m *Member) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
	m.logger.Info("member stopped")
}

// Send sends a message to this member
func (m *Member) Send(msg Message) {
	select {
	case m.inbox <- msg:
	default:
		m.logger.Warn("inbox full, dropping message", "type", msg.Type)
	}
}

// GetStatus returns current status
func (m *Member) GetStatus() MemberStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Status
}

// GetCurrentTask returns the current task if any
func (m *Member) GetCurrentTask() *Task {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Task
}

func (m *Member) run() {
	for {
		select {
		case <-m.ctx.Done():
			return

		case msg := <-m.inbox:
			m.handleMessage(msg)
		}
	}
}

func (m *Member) handleMessage(msg Message) {
	m.logger.Debug("received message", "type", msg.Type, "from", msg.From)

	switch msg.Type {
	case MsgClientRequest:
		m.handleClientRequest(msg)
	case MsgTaskAssignment:
		m.handleTaskAssignment(msg)
	case MsgQuestion:
		m.handleQuestion(msg)
	case MsgReport:
		m.handleReport(msg)
	case MsgAnswer:
		m.handleAnswer(msg)
	}
}

func (m *Member) handleClientRequest(msg Message) {
	m.setStatus(MemberWorking)
	defer m.setStatus(MemberIdle)

	content, ok := msg.Content.(string)
	if !ok {
		m.logger.Error("invalid client request content")
		return
	}

	// Build the prompt with persona and conversation history
	messages := []provider.Message{
		{Role: "system", Content: m.buildSystemPrompt()},
	}

	// Add conversation history for context continuity
	messages = append(messages, m.getContextMessages()...)

	// Add the new user message
	messages = append(messages, provider.Message{Role: "user", Content: content})

	// Persist user message to context
	m.addToContext("user", content)

	// Get tools if available
	providerTools := m.getProviderTools()

	// Tool execution loop
	var finalContent string
	for iteration := 0; iteration < MaxToolIterations; iteration++ {
		// Call the model with token mode settings
		resp, err := m.Provider.Chat(m.ctx, &provider.ChatRequest{
			Model:       m.getEffectiveModel(),
			Messages:    messages,
			Tools:       providerTools,
			Temperature: m.Role.Model.Temperature,
			MaxTokens:   m.getEffectiveMaxTokens(),
		})

		if err != nil {
			m.logger.Error("model call failed", "error", err)
			m.sendToTeam(Message{
				ID:        uuid.New().String(),
				Type:      MsgClientResponse,
				From:      m.ID,
				To:        "client",
				Content:   fmt.Sprintf("I encountered an error: %v", err),
				Timestamp: time.Now(),
			})
			return
		}

		// Check for tool calls
		if len(resp.ToolCalls) > 0 && m.toolRegistry != nil {
			m.logger.Debug("processing tool calls", "count", len(resp.ToolCalls))

			// Add assistant message with tool calls to history
			messages = append(messages, provider.Message{
				Role:      "assistant",
				Content:   resp.Content,
				ToolCalls: resp.ToolCalls,
			})

			// Execute tools and add results
			toolResults := m.executeToolCalls(m.ctx, resp.ToolCalls)
			messages = append(messages, toolResults...)

			// Continue loop to let model process tool results
			continue
		}

		// No tool calls - we have the final response
		finalContent = resp.Content
		break
	}

	// Persist assistant response to context
	m.addToContext("assistant", finalContent)

	// Parse the response for potential delegations or direct response
	action := m.parseResponse(finalContent, content)

	switch action.Type {
	case "delegate":
		m.handleDelegation(action, msg)
	case "parallel_delegate":
		m.handleParallelDelegation(action, msg)
	case "question":
		m.askClient(action.Content)
	case "respond":
		m.respondToClient(action.Content)
	}
}

func (m *Member) handleTaskAssignment(msg Message) {
	task, ok := msg.Content.(*Task)
	if !ok {
		m.logger.Error("invalid task assignment content")
		return
	}

	m.mu.Lock()
	m.Task = task
	m.Status = MemberWorking
	m.mu.Unlock()

	m.logger.Info("working on task", "task_id", task.ID)

	// Notify activity
	m.Team.NotifyActivity(m.ID, "task_started", fmt.Sprintf("Started working on: %s", truncateMessage(task.Content, 100)))

	// Build context with task details and history
	messages := []provider.Message{
		{Role: "system", Content: m.buildSystemPrompt()},
	}

	// Add conversation history
	messages = append(messages, m.getContextMessages()...)

	// Add the task content
	messages = append(messages, provider.Message{Role: "user", Content: task.Content})

	// Add any task-specific context
	messages = append(messages, task.Context...)

	// Persist task message to context
	m.addToContext("user", task.Content)

	// Get tools if available
	providerTools := m.getProviderTools()

	// Tool execution loop
	var finalContent string
	for iteration := 0; iteration < MaxToolIterations; iteration++ {
		// Execute the task with token mode settings
		resp, err := m.Provider.Chat(m.ctx, &provider.ChatRequest{
			Model:       m.getEffectiveModel(),
			Messages:    messages,
			Tools:       providerTools,
			Temperature: m.Role.Model.Temperature,
			MaxTokens:   m.getEffectiveMaxTokens(),
		})

		if err != nil {
			m.reportTaskFailure(task, err)
			return
		}

		// Check for tool calls
		if len(resp.ToolCalls) > 0 && m.toolRegistry != nil {
			m.logger.Debug("processing tool calls", "count", len(resp.ToolCalls))

			// Add assistant message with tool calls to history
			messages = append(messages, provider.Message{
				Role:      "assistant",
				Content:   resp.Content,
				ToolCalls: resp.ToolCalls,
			})

			// Execute tools and add results
			toolResults := m.executeToolCalls(m.ctx, resp.ToolCalls)
			messages = append(messages, toolResults...)

			// Continue loop to let model process tool results
			continue
		}

		// No tool calls - we have the final response
		finalContent = resp.Content
		break
	}

	// Persist assistant response to context
	m.addToContext("assistant", finalContent)

	// Check if needs to delegate further or complete
	action := m.parseResponse(finalContent, task.Content)

	switch action.Type {
	case "delegate":
		// Can this role delegate?
		if len(m.Role.CanDelegate) > 0 {
			m.delegateToRole(action.Target, action.Content, task)
		} else {
			// Complete with current result
			m.completeTask(task, finalContent)
		}
	case "complete":
		m.completeTask(task, action.Content)
	default:
		m.completeTask(task, finalContent)
	}
}

func (m *Member) handleQuestion(msg Message) {
	// Another member is asking us a question
	content, ok := msg.Content.(string)
	if !ok {
		return
	}

	m.setStatus(MemberWorking)
	defer m.setStatus(MemberIdle)

	messages := []provider.Message{
		{Role: "system", Content: m.buildSystemPrompt()},
		{Role: "user", Content: fmt.Sprintf("A colleague asks: %s", content)},
	}

	resp, err := m.Provider.Chat(m.ctx, &provider.ChatRequest{
		Model:       m.Role.Model.Model,
		Messages:    messages,
		Temperature: m.Role.Model.Temperature,
		MaxTokens:   m.Role.Model.MaxTokens,
	})

	if err != nil {
		m.logger.Error("failed to answer question", "error", err)
		return
	}

	// Send answer back
	m.sendToTeam(Message{
		ID:        uuid.New().String(),
		Type:      MsgAnswer,
		From:      m.ID,
		To:        msg.From,
		Content:   resp.Content,
		TaskID:    msg.TaskID,
		Timestamp: time.Now(),
	})
}

func (m *Member) handleReport(msg Message) {
	// A subordinate is reporting back
	m.logger.Debug("received report", "from", msg.From)
	// This will be picked up by pending task handlers
}

func (m *Member) handleAnswer(msg Message) {
	// Received answer to a question we asked
	m.logger.Debug("received answer", "from", msg.From)
}

func (m *Member) buildSystemPrompt() string {
	tokenMode := m.Team.GetTokenMode()

	// Use condensed persona in low/minimal token mode if available
	var persona string
	if (tokenMode == TokenModeLow || tokenMode == TokenModeMinimal) && m.Role.PersonaCondensed != "" {
		persona = m.Role.PersonaCondensed
	} else if tokenMode == TokenModeMinimal {
		// Auto-condensed: just the first line of persona
		lines := splitLines(m.Role.Persona)
		if len(lines) > 0 {
			persona = lines[0]
		} else {
			persona = m.Role.Persona
		}
	} else {
		persona = m.Role.Persona
	}

	prompt := persona + "\n\n"

	// Use name if available
	if m.Name != "" && m.Name != m.Role.Title {
		prompt += fmt.Sprintf("Your name is %s. You are the %s on team '%s'.\n", m.Name, m.Role.Title, m.Team.Name)
	} else {
		prompt += fmt.Sprintf("You are the %s on team '%s'.\n", m.Role.Title, m.Team.Name)
	}

	// Only include responsibilities in normal mode
	if tokenMode == TokenModeNormal && len(m.Role.Responsibilities) > 0 {
		prompt += "Your responsibilities:\n"
		for _, r := range m.Role.Responsibilities {
			prompt += fmt.Sprintf("- %s\n", r)
		}
	}

	// Add tool information (condensed in low token mode)
	if m.toolRegistry != nil {
		toolsInfo := m.toolRegistry.FormatToolsForPrompt()
		if toolsInfo != "" && toolsInfo != "No tools available." {
			if tokenMode == TokenModeNormal {
				prompt += "\n" + toolsInfo + "\n"
				prompt += "Use these tools to accomplish your tasks. You can read files, write code, run commands, and interact with the codebase.\n"
			} else {
				prompt += "\nTools available: " + m.toolRegistry.ListToolNames() + "\n"
			}
		}
	}

	if len(m.Role.CanDelegate) > 0 {
		prompt += "\nYou can delegate tasks to: " + fmt.Sprintf("%v", m.Role.CanDelegate) + "\n"
		if tokenMode == TokenModeNormal {
			prompt += "To delegate to ONE member: DELEGATE TO [role]: [task description]\n"
			prompt += "To delegate to MULTIPLE members in parallel:\n"
			prompt += "DELEGATE PARALLEL:\n"
			prompt += "- role1: task for role1\n"
			prompt += "- role2: task for role2\n"
			prompt += "Use parallel delegation when tasks are independent and can run simultaneously.\n"
		} else {
			prompt += "DELEGATE TO [role]: [task] or DELEGATE PARALLEL:\\n- role: task\n"
		}
	}

	if tokenMode == TokenModeNormal {
		if m.Role.Visibility == "client" {
			prompt += "\nYou interact directly with clients. Be professional and clear.\n"
		} else {
			prompt += "\nYou are an internal team member. Report to your lead, not the client.\n"
		}

		if m.Role.ReportsTo != "" {
			prompt += fmt.Sprintf("You report to: %s\n", m.Role.ReportsTo)
		}
	}

	prompt += "\nCOMPLETE: [response] | ASK CLIENT: [question]\n"

	return prompt
}

// getEffectiveMaxTokens returns max tokens based on token mode
func (m *Member) getEffectiveMaxTokens() *int {
	settings := m.Team.GetTokenSettings()

	// If role has explicit max tokens, use that unless in low/minimal mode
	if m.Role.Model.MaxTokens != nil && settings.Mode == TokenModeNormal {
		return m.Role.Model.MaxTokens
	}

	// Use mode-based max tokens
	maxTokens := settings.MaxTokens
	return &maxTokens
}

// getEffectiveModel returns model based on token mode
func (m *Member) getEffectiveModel() string {
	settings := m.Team.GetTokenSettings()

	// Use low token model if available and in low/minimal mode
	if (settings.Mode == TokenModeLow || settings.Mode == TokenModeMinimal) && m.Role.Model.LowTokenModel != "" {
		return m.Role.Model.LowTokenModel
	}

	return m.Role.Model.Model
}

// getContextLimit returns context history limit based on token mode
func (m *Member) getContextLimit() int {
	settings := m.Team.GetTokenSettings()
	return settings.ContextHistory
}

type responseAction struct {
	Type    string // "delegate", "parallel_delegate", "question", "respond", "complete"
	Target  string // For delegation - which role
	Content string
	// For parallel delegation
	ParallelTasks []parallelTask
}

type parallelTask struct {
	Role    string
	Content string
}

func (m *Member) parseResponse(response, originalRequest string) responseAction {
	// Simple parsing for delegation/question/response patterns

	// Check for parallel delegation first (higher priority)
	if idx := indexOf(response, "DELEGATE PARALLEL:"); idx >= 0 {
		rest := response[idx+18:]
		tasks := parseParallelTasks(rest)
		if len(tasks) > 0 {
			return responseAction{Type: "parallel_delegate", ParallelTasks: tasks}
		}
	}

	// Check for single delegation
	if idx := indexOf(response, "DELEGATE TO "); idx >= 0 {
		rest := response[idx+12:]
		if colonIdx := indexOf(rest, ":"); colonIdx > 0 {
			target := rest[:colonIdx]
			content := rest[colonIdx+1:]
			return responseAction{Type: "delegate", Target: trim(target), Content: trim(content)}
		}
	}

	// Check for ASK <role>: pattern (inter-agent communication)
	if idx := indexOf(response, "ASK "); idx >= 0 {
		rest := response[idx+4:]
		if colonIdx := indexOf(rest, ":"); colonIdx > 0 {
			target := trim(rest[:colonIdx])
			content := trim(rest[colonIdx+1:])
			// Check if it's asking the client or another role
			if target == "CLIENT" || target == "client" {
				return responseAction{Type: "question", Content: content}
			}
			// It's asking another team member - treat as delegation
			return responseAction{Type: "delegate", Target: target, Content: content}
		}
	}

	// Check for completion
	if idx := indexOf(response, "COMPLETE:"); idx >= 0 {
		content := response[idx+9:]
		return responseAction{Type: "complete", Content: trim(content)}
	}

	// Default: treat as complete response
	return responseAction{Type: "respond", Content: response}
}

// parseParallelTasks parses lines like "- role: task content"
func parseParallelTasks(text string) []parallelTask {
	var tasks []parallelTask
	lines := splitLines(text)

	for _, line := range lines {
		line = trim(line)
		if len(line) < 2 {
			continue
		}

		// Skip if doesn't start with "- "
		if line[0] != '-' {
			continue
		}
		line = trim(line[1:]) // Remove the dash

		// Find the role:content separator
		colonIdx := indexOf(line, ":")
		if colonIdx <= 0 {
			continue
		}

		role := trim(line[:colonIdx])
		content := trim(line[colonIdx+1:])

		if role != "" && content != "" {
			tasks = append(tasks, parallelTask{Role: role, Content: content})
		}
	}

	return tasks
}

// splitLines splits text into lines
func splitLines(text string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			lines = append(lines, text[start:i])
			start = i + 1
		}
	}
	if start < len(text) {
		lines = append(lines, text[start:])
	}
	return lines
}

func (m *Member) handleDelegation(action responseAction, originalMsg Message) {
	// Find the target role member
	target := m.Team.GetMemberByRole(action.Target)
	if target == nil {
		m.logger.Warn("delegation target not found", "target", action.Target)
		m.respondToClient(action.Content)
		return
	}

	// Create a task for the target with a result channel
	task := &Task{
		ID:         uuid.New().String(),
		Content:    action.Content,
		From:       m.ID,
		To:         target.ID,
		Status:     TaskAssigned,
		Priority:   1,
		CreatedAt:  time.Now(),
		ResultChan: make(chan *TaskResult, 1),
	}

	m.Team.AddTask(task)

	// Send task assignment
	target.Send(Message{
		ID:        uuid.New().String(),
		Type:      MsgTaskAssignment,
		From:      m.ID,
		To:        target.ID,
		Content:   task,
		TaskID:    task.ID,
		Timestamp: time.Now(),
	})

	m.logger.Info("delegated task, waiting for result", "to", action.Target, "task_id", task.ID)

	// Notify about the delegation
	m.Team.NotifyActivity(m.ID, "delegation", fmt.Sprintf("Delegated to %s: %s", target.DisplayName(), truncateMessage(action.Content, 100)))
	m.Team.NotifyActivity(target.ID, "task_received", fmt.Sprintf("Received task from %s", m.DisplayName()))

	// Wait for the delegated task to complete
	select {
	case <-m.ctx.Done():
		m.logger.Warn("context cancelled while waiting for delegation")
		return
	case result := <-task.ResultChan:
		if result != nil {
			if result.Success {
				// Process the result - let the member decide what to do next
				m.processTaskResult(result.Content, target.RoleName, originalMsg)
			} else {
				m.respondToClient(fmt.Sprintf("Task failed: %s", result.Error))
			}
		} else {
			m.respondToClient("Task completed without result")
		}
	}
}

// processTaskResult handles the result from a delegated task
// The member can decide to delegate further, respond to client, etc.
func (m *Member) processTaskResult(result, fromRole string, originalMsg Message) {
	m.setStatus(MemberWorking)
	defer m.setStatus(MemberIdle)

	// Build context with the delegation result
	messages := []provider.Message{
		{Role: "system", Content: m.buildSystemPrompt()},
	}

	// Add conversation history
	messages = append(messages, m.getContextMessages()...)

	// Add the result as context
	prompt := fmt.Sprintf("The %s completed their task and returned:\n\n%s\n\nBased on this, decide your next action. You can:\n1. DELEGATE TO another role for the next task\n2. Send a brief friendly update to the client (keep it short, no technical details)\n\nRemember: Keep client messages to 1-2 sentences. Technical details stay internal.", fromRole, result)
	messages = append(messages, provider.Message{Role: "user", Content: prompt})

	// Get response from LLM
	resp, err := m.Provider.Chat(m.ctx, &provider.ChatRequest{
		Model:    m.Role.Model.Model,
		Messages: messages,
	})

	if err != nil {
		m.logger.Error("failed to process task result", "error", err)
		m.respondToClient("Working on it!")
		return
	}

	// Parse and handle the response
	action := m.parseResponse(resp.Content, prompt)

	switch action.Type {
	case "delegate":
		m.handleDelegation(action, originalMsg)
	case "parallel_delegate":
		m.handleParallelDelegation(action, originalMsg)
	case "question":
		m.askClient(action.Content)
	default:
		m.respondToClient(action.Content)
	}
}

// handleParallelDelegation delegates to multiple team members simultaneously
func (m *Member) handleParallelDelegation(action responseAction, originalMsg Message) {
	if len(action.ParallelTasks) == 0 {
		m.respondToClient("No tasks to delegate")
		return
	}

	m.logger.Info("starting parallel delegation", "count", len(action.ParallelTasks))

	// Create tasks and result channels for all targets
	type taskInfo struct {
		role    string
		task    *Task
		target  *Member
	}
	var tasks []taskInfo

	for _, pt := range action.ParallelTasks {
		target := m.Team.GetMemberByRole(pt.Role)
		if target == nil {
			m.logger.Warn("parallel delegation target not found", "target", pt.Role)
			continue
		}

		task := &Task{
			ID:         uuid.New().String(),
			Content:    pt.Content,
			From:       m.ID,
			To:         target.ID,
			Status:     TaskAssigned,
			Priority:   1,
			CreatedAt:  time.Now(),
			ResultChan: make(chan *TaskResult, 1),
		}

		m.Team.AddTask(task)
		tasks = append(tasks, taskInfo{role: pt.Role, task: task, target: target})
	}

	if len(tasks) == 0 {
		m.respondToClient("No valid delegation targets found")
		return
	}

	// Send all task assignments simultaneously
	for _, ti := range tasks {
		ti.target.Send(Message{
			ID:        uuid.New().String(),
			Type:      MsgTaskAssignment,
			From:      m.ID,
			To:        ti.target.ID,
			Content:   ti.task,
			TaskID:    ti.task.ID,
			Timestamp: time.Now(),
		})
		m.logger.Info("parallel task sent", "to", ti.role, "task_id", ti.task.ID)
	}

	// Collect results from all tasks in parallel
	type resultInfo struct {
		role   string
		result *TaskResult
	}
	resultsChan := make(chan resultInfo, len(tasks))

	// Start goroutines to wait for each result
	for _, ti := range tasks {
		go func(role string, task *Task) {
			select {
			case <-m.ctx.Done():
				resultsChan <- resultInfo{role: role, result: &TaskResult{Success: false, Error: "context cancelled"}}
			case result := <-task.ResultChan:
				resultsChan <- resultInfo{role: role, result: result}
			}
		}(ti.role, ti.task)
	}

	// Collect all results
	var results []resultInfo
	for i := 0; i < len(tasks); i++ {
		select {
		case <-m.ctx.Done():
			m.logger.Warn("context cancelled while waiting for parallel results")
			m.respondToClient("Parallel tasks cancelled")
			return
		case r := <-resultsChan:
			results = append(results, r)
			m.logger.Info("parallel result received", "role", r.role, "success", r.result != nil && r.result.Success)
		}
	}

	// Combine results into a single response
	var combined string
	combined = "Team responses:\n\n"
	for _, r := range results {
		combined += fmt.Sprintf("**%s**:\n", r.role)
		if r.result != nil && r.result.Success {
			combined += r.result.Content + "\n\n"
		} else if r.result != nil {
			combined += fmt.Sprintf("(Failed: %s)\n\n", r.result.Error)
		} else {
			combined += "(No response)\n\n"
		}
	}

	m.respondToClient(combined)
}

func (m *Member) delegateToRole(roleName, content string, parentTask *Task) {
	target := m.Team.GetMemberByRole(roleName)
	if target == nil {
		m.logger.Warn("delegation target not found", "target", roleName)
		m.completeTask(parentTask, content)
		return
	}

	task := &Task{
		ID:         uuid.New().String(),
		Content:    content,
		From:       m.ID,
		To:         target.ID,
		Status:     TaskAssigned,
		Priority:   parentTask.Priority,
		Metadata:   map[string]interface{}{"parent_task": parentTask.ID},
		CreatedAt:  time.Now(),
		ResultChan: make(chan *TaskResult, 1),
	}

	m.Team.AddTask(task)

	target.Send(Message{
		ID:        uuid.New().String(),
		Type:      MsgTaskAssignment,
		From:      m.ID,
		To:        target.ID,
		Content:   task,
		TaskID:    task.ID,
		Timestamp: time.Now(),
	})

	m.logger.Info("delegated subtask, waiting for result", "to", roleName, "task_id", task.ID)

	// Wait for the delegated task to complete
	select {
	case <-m.ctx.Done():
		m.logger.Warn("context cancelled while waiting for delegation")
		return
	case result := <-task.ResultChan:
		if result != nil {
			// Complete parent task with the delegated result
			m.completeTask(parentTask, result.Content)
		} else {
			m.completeTask(parentTask, "Subtask completed without result")
		}
	}
}

func (m *Member) askClient(question string) {
	m.sendToTeam(Message{
		ID:        uuid.New().String(),
		Type:      MsgClientResponse,
		From:      m.ID,
		To:        "client",
		Content:   question,
		Timestamp: time.Now(),
	})
}

func (m *Member) respondToClient(content string) {
	m.sendToTeam(Message{
		ID:        uuid.New().String(),
		Type:      MsgClientResponse,
		From:      m.ID,
		To:        "client",
		Content:   content,
		Timestamp: time.Now(),
	})
}

func (m *Member) completeTask(task *Task, result string) {
	task.Status = TaskCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.Result = &TaskResult{
		Content: result,
		Success: true,
	}

	m.mu.Lock()
	m.Task = nil
	m.Status = MemberIdle
	m.mu.Unlock()

	// If there's a result channel, send the result (for delegation wait)
	if task.ResultChan != nil {
		select {
		case task.ResultChan <- task.Result:
		default:
			m.logger.Warn("result channel full or closed", "task_id", task.ID)
		}
	}

	// Report to who assigned the task
	m.sendToTeam(Message{
		ID:        uuid.New().String(),
		Type:      MsgTaskComplete,
		From:      m.ID,
		To:        task.From,
		Content:   task,
		TaskID:    task.ID,
		Timestamp: time.Now(),
	})

	// Notify activity
	m.Team.NotifyActivity(m.ID, "task_completed", fmt.Sprintf("Completed task: %s", truncateMessage(result, 100)))

	m.logger.Info("task completed", "task_id", task.ID)
}

func (m *Member) reportTaskFailure(task *Task, err error) {
	task.Status = TaskFailed
	now := time.Now()
	task.CompletedAt = &now
	task.Result = &TaskResult{
		Success: false,
		Error:   err.Error(),
	}

	m.mu.Lock()
	m.Task = nil
	m.Status = MemberIdle
	m.mu.Unlock()

	// If there's a result channel, send the failure result
	if task.ResultChan != nil {
		select {
		case task.ResultChan <- task.Result:
		default:
			m.logger.Warn("result channel full or closed", "task_id", task.ID)
		}
	}

	m.sendToTeam(Message{
		ID:        uuid.New().String(),
		Type:      MsgTaskComplete,
		From:      m.ID,
		To:        task.From,
		Content:   task,
		TaskID:    task.ID,
		Timestamp: time.Now(),
	})

	m.logger.Error("task failed", "task_id", task.ID, "error", err)
}

func (m *Member) setStatus(status MemberStatus) {
	m.mu.Lock()
	m.Status = status
	m.mu.Unlock()

	// Notify status change
	m.Team.NotifyActivity(m.ID, "status_change", string(status))
}

func (m *Member) sendToTeam(msg Message) {
	m.Team.RouteMessage(msg)
}

// MarshalJSON for API responses
func (m *Member) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":           m.ID,
		"name":         m.Name,
		"display_name": m.DisplayName(),
		"role":         m.RoleName,
		"title":        m.Role.Title,
		"status":       m.Status,
		"task":         m.Task,
	})
}

// Helper functions
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func trim(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n') {
		end--
	}
	return s[start:end]
}

func truncateMessage(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
