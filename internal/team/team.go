package team

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/arcslash/ugudu/internal/logger"
	"github.com/arcslash/ugudu/internal/provider"
	"github.com/arcslash/ugudu/internal/tools"
	"github.com/arcslash/ugudu/internal/workspace"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// PersistenceCallbacks allows teams to persist state without direct store dependency
type PersistenceCallbacks struct {
	// SaveContext saves a message to an agent's conversation context
	SaveContext func(teamName, memberID, conversationID, role, content string, sequence int) error
	// LoadContext retrieves the conversation context for a member
	LoadContext func(teamName, memberID string, limit int) ([]ContextMessage, error)
	// CreateConversation starts a new conversation
	CreateConversation func(teamName string) (string, error)
	// GetActiveConversation returns the active conversation ID
	GetActiveConversation func(teamName string) (string, error)
	// OnActivity is called when there's team activity (delegation, task updates, etc.)
	OnActivity func(teamName, memberID, activityType, message string)
}

// ContextMessage represents a message in conversation context
type ContextMessage struct {
	Role    string
	Content string
}

// Team represents a group of AI agents working together
type Team struct {
	Name          string
	Spec          *TeamSpec
	Members       map[string]*Member   // by ID
	MembersByRole map[string][]*Member // by role name
	ClientFacing  []string

	providers      *provider.Registry
	tasks          map[string]*Task
	clientChan     chan Message // Messages to/from client
	internalChan   chan Message // Internal team messages
	persistence    *PersistenceCallbacks
	conversationID string // Current active conversation

	// Tool execution
	workspace    *workspace.Workspace
	toolRegistry *tools.Registry // Base tool registry

	// Token management
	tokenMode TokenMode // Current token consumption mode

	ctx    context.Context
	cancel context.CancelFunc
	logger *logger.Logger
	mu     sync.RWMutex
	taskMu sync.RWMutex
}

// SetTokenMode sets the token consumption mode for the team
func (t *Team) SetTokenMode(mode TokenMode) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.tokenMode = mode
	t.logger.Info("token mode changed", "mode", mode)
}

// GetTokenMode returns the current token mode
func (t *Team) GetTokenMode() TokenMode {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.tokenMode == "" {
		return TokenModeNormal
	}
	return t.tokenMode
}

// GetTokenSettings returns effective token settings based on mode
func (t *Team) GetTokenSettings() TokenSettings {
	mode := t.GetTokenMode()

	// If spec has explicit settings, use those
	if t.Spec != nil && t.Spec.Settings.Token.Mode != "" {
		return t.Spec.Settings.Token
	}

	// Default settings based on mode
	switch mode {
	case TokenModeLow:
		return TokenSettings{
			Mode:           TokenModeLow,
			MaxTokens:      1024,
			ContextHistory: 10,
		}
	case TokenModeMinimal:
		return TokenSettings{
			Mode:           TokenModeMinimal,
			MaxTokens:      512,
			ContextHistory: 5,
		}
	default:
		return TokenSettings{
			Mode:           TokenModeNormal,
			MaxTokens:      4096,
			ContextHistory: 40,
		}
	}
}

// LoadSpec loads a team specification from YAML file
func LoadSpec(path string) (*TeamSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read spec file: %w", err)
	}

	// Expand environment variables
	expanded := os.ExpandEnv(string(data))

	var spec TeamSpec
	if err := yaml.Unmarshal([]byte(expanded), &spec); err != nil {
		return nil, fmt.Errorf("parse spec: %w", err)
	}

	// Set defaults
	if spec.APIVersion == "" {
		spec.APIVersion = "ugudu/v1"
	}
	if spec.Kind == "" {
		spec.Kind = "Team"
	}

	// Default count to 1 for each role
	for name, role := range spec.Roles {
		if role.Count == 0 {
			role.Count = 1
		}
		if role.Visibility == "" {
			role.Visibility = "internal"
		}
		spec.Roles[name] = role
	}

	return &spec, nil
}

// NewTeam creates a new team from a specification
func NewTeam(spec *TeamSpec, providers *provider.Registry, log *logger.Logger) (*Team, error) {
	return NewTeamWithPersistence(spec, providers, log, nil)
}

// NewTeamWithPersistence creates a new team with persistence callbacks
func NewTeamWithPersistence(spec *TeamSpec, providers *provider.Registry, log *logger.Logger, persistence *PersistenceCallbacks) (*Team, error) {
	// Create base tool registry
	baseRegistry := tools.NewRegistry()

	t := &Team{
		Name:          spec.Metadata.Name,
		Spec:          spec,
		Members:       make(map[string]*Member),
		MembersByRole: make(map[string][]*Member),
		ClientFacing:  spec.ClientFacing,
		providers:     providers,
		tasks:         make(map[string]*Task),
		clientChan:    make(chan Message, 100),
		internalChan:  make(chan Message, 1000),
		persistence:   persistence,
		toolRegistry:  baseRegistry,
		logger:        log.With("team", spec.Metadata.Name),
	}

	// Create members for each role
	for roleName, role := range spec.Roles {
		// Get provider for this role
		prov, err := providers.Get(role.Model.Provider)
		if err != nil {
			return nil, fmt.Errorf("provider %s not found for role %s: %w", role.Model.Provider, roleName, err)
		}

		// Create the specified number of members for this role
		for i := 0; i < role.Count; i++ {
			memberID := fmt.Sprintf("%s-%s", roleName, uuid.New().String()[:8])
			if role.Count == 1 {
				memberID = roleName // Simpler ID for single-instance roles
			}

			// Determine name for this member
			var memberName string
			if role.Count == 1 && role.Name != "" {
				// Single member with a name
				memberName = role.Name
			} else if len(role.Names) > i {
				// Multiple members with names list
				memberName = role.Names[i]
			}
			// If no name specified, NewMember will use the title as fallback

			member := NewMember(memberID, memberName, roleName, role, t, prov, log)

			// Create sandboxed tool registry for this member
			sandboxedRegistry := tools.NewSandboxedRegistry(baseRegistry, t.workspace, roleName, memberID)

			// Set up activity logging
			sandboxedRegistry.OnToolExecute = func(toolName string, args map[string]interface{}, result interface{}, err error) {
				if err != nil {
					log.Debug("tool execution failed", "member", memberID, "tool", toolName, "error", err)
				} else {
					log.Debug("tool executed", "member", memberID, "tool", toolName)
				}
			}

			member.SetToolRegistry(sandboxedRegistry)

			t.Members[memberID] = member
			t.MembersByRole[roleName] = append(t.MembersByRole[roleName], member)
		}
	}

	// Determine client-facing roles if not specified
	if len(t.ClientFacing) == 0 {
		for roleName, role := range spec.Roles {
			if role.Visibility == "client" {
				t.ClientFacing = append(t.ClientFacing, roleName)
			}
		}
	}

	return t, nil
}

// SetWorkspace sets the workspace for this team (enables sandboxed tool execution)
func (t *Team) SetWorkspace(ws *workspace.Workspace) {
	t.workspace = ws

	// Update all member registries with the workspace
	for _, member := range t.Members {
		sandboxedRegistry := tools.NewSandboxedRegistry(t.toolRegistry, ws, member.RoleName, member.ID)
		sandboxedRegistry.RegisterRoleTools()
		member.SetToolRegistry(sandboxedRegistry)
	}
}

// Start begins all team members
func (t *Team) Start(ctx context.Context) error {
	t.ctx, t.cancel = context.WithCancel(ctx)

	// Create or get active conversation for persistence
	if t.persistence != nil && t.persistence.GetActiveConversation != nil {
		convID, err := t.persistence.GetActiveConversation(t.Name)
		if err != nil {
			t.logger.Warn("failed to get active conversation", "error", err)
		}
		if convID == "" && t.persistence.CreateConversation != nil {
			convID, err = t.persistence.CreateConversation(t.Name)
			if err != nil {
				t.logger.Warn("failed to create conversation", "error", err)
			}
		}
		t.conversationID = convID
		t.logger.Debug("conversation context", "id", convID)
	}

	// Load persisted context for each member
	if t.persistence != nil && t.persistence.LoadContext != nil {
		for _, member := range t.Members {
			history, err := t.persistence.LoadContext(t.Name, member.ID, 50) // Load last 50 messages
			if err != nil {
				t.logger.Warn("failed to load member context", "member", member.ID, "error", err)
				continue
			}
			if len(history) > 0 {
				member.RestoreContext(history)
				t.logger.Debug("restored context", "member", member.ID, "messages", len(history))
			}
		}
	}

	// Start internal message router
	go t.routeInternal()

	// Start all members
	for _, member := range t.Members {
		member.Start(t.ctx)
	}

	t.logger.Info("team started", "members", len(t.Members), "conversation", t.conversationID)
	return nil
}

// Stop halts all team members
func (t *Team) Stop() {
	if t.cancel != nil {
		t.cancel()
	}

	for _, member := range t.Members {
		member.Stop()
	}

	t.logger.Info("team stopped")
}

// Ask sends a request to the team (goes to client-facing member)
func (t *Team) Ask(content string) <-chan Message {
	responseChan := make(chan Message, 10)

	go func() {
		defer close(responseChan)

		// Find the primary client-facing member
		var target *Member
		if len(t.ClientFacing) > 0 {
			// Use first client-facing role
			if members, ok := t.MembersByRole[t.ClientFacing[0]]; ok && len(members) > 0 {
				target = members[0]
			}
		}

		if target == nil {
			// Fallback: use first available member
			for _, member := range t.Members {
				target = member
				break
			}
		}

		if target == nil {
			responseChan <- Message{
				Type:    MsgClientResponse,
				From:    "system",
				To:      "client",
				Content: "No team members available",
			}
			return
		}

		// Send request to target
		target.Send(Message{
			ID:      uuid.New().String(),
			Type:    MsgClientRequest,
			From:    "client",
			To:      target.ID,
			Content: content,
		})

		// Wait for responses - keep listening for all messages
		// Use a timeout to detect when work is complete
		timeout := time.After(10 * time.Minute)
		lastActivity := time.Now()
		idleTimeout := 30 * time.Second // Consider done if no activity for 30s

		for {
			select {
			case <-t.ctx.Done():
				return
			case msg := <-t.clientChan:
				responseChan <- msg
				lastActivity = time.Now()
				t.logger.Debug("client message sent", "from", msg.From, "type", msg.Type)
			case <-time.After(idleTimeout):
				// Check if we've been idle long enough to consider done
				if time.Since(lastActivity) >= idleTimeout {
					t.logger.Debug("response complete - idle timeout")
					return
				}
			case <-timeout:
				t.logger.Warn("response timeout")
				return
			}
		}
	}()

	return responseChan
}

// AskMember sends a request to a specific member by role
func (t *Team) AskMember(roleName, content string) <-chan Message {
	responseChan := make(chan Message, 10)

	go func() {
		defer close(responseChan)

		target := t.GetMemberByRole(roleName)
		if target == nil {
			responseChan <- Message{
				Type:    MsgClientResponse,
				From:    "system",
				To:      "client",
				Content: fmt.Sprintf("No member found with role: %s", roleName),
			}
			return
		}

		target.Send(Message{
			ID:      uuid.New().String(),
			Type:    MsgClientRequest,
			From:    "client",
			To:      target.ID,
			Content: content,
		})

		// Wait for response
		select {
		case <-t.ctx.Done():
			return
		case msg := <-t.clientChan:
			responseChan <- msg
		}
	}()

	return responseChan
}

// GetMemberByRole returns the first member with the given role
func (t *Team) GetMemberByRole(roleName string) *Member {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if members, ok := t.MembersByRole[roleName]; ok && len(members) > 0 {
		// Find an idle member if possible
		for _, m := range members {
			if m.GetStatus() == MemberIdle {
				return m
			}
		}
		// Otherwise return first
		return members[0]
	}
	return nil
}

// GetMember returns a member by ID
func (t *Team) GetMember(id string) *Member {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.Members[id]
}

// ListMembers returns all members
func (t *Team) ListMembers() []*Member {
	t.mu.RLock()
	defer t.mu.RUnlock()

	members := make([]*Member, 0, len(t.Members))
	for _, m := range t.Members {
		members = append(members, m)
	}
	return members
}

// AddTask adds a task to the team's task list
func (t *Team) AddTask(task *Task) {
	t.taskMu.Lock()
	defer t.taskMu.Unlock()
	t.tasks[task.ID] = task
}

// GetTask retrieves a task by ID
func (t *Team) GetTask(id string) *Task {
	t.taskMu.RLock()
	defer t.taskMu.RUnlock()
	return t.tasks[id]
}

// ListTasks returns all tasks
func (t *Team) ListTasks() []*Task {
	t.taskMu.RLock()
	defer t.taskMu.RUnlock()

	tasks := make([]*Task, 0, len(t.tasks))
	for _, task := range t.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// RouteMessage routes a message to the appropriate destination
func (t *Team) RouteMessage(msg Message) {
	if msg.To == "client" {
		select {
		case t.clientChan <- msg:
		default:
			t.logger.Warn("client channel full, dropping message")
		}
		return
	}

	// Route to internal
	select {
	case t.internalChan <- msg:
	default:
		t.logger.Warn("internal channel full, dropping message")
	}
}

func (t *Team) routeInternal() {
	for {
		select {
		case <-t.ctx.Done():
			return
		case msg := <-t.internalChan:
			// Find target member
			if member, ok := t.Members[msg.To]; ok {
				member.Send(msg)
			} else {
				t.logger.Warn("unknown message target", "to", msg.To)
			}
		}
	}
}

// SetPersistence sets the persistence callbacks (can be called after creation)
func (t *Team) SetPersistence(p *PersistenceCallbacks) {
	t.persistence = p
}

// SaveMemberContext persists a member's context message
func (t *Team) SaveMemberContext(memberID, role, content string, sequence int) {
	if t.persistence == nil || t.persistence.SaveContext == nil {
		return
	}

	if err := t.persistence.SaveContext(t.Name, memberID, t.conversationID, role, content, sequence); err != nil {
		t.logger.Warn("failed to save context", "member", memberID, "error", err)
	}
}

// NotifyActivity broadcasts an activity event
func (t *Team) NotifyActivity(memberID, activityType, message string) {
	if t.persistence != nil && t.persistence.OnActivity != nil {
		t.persistence.OnActivity(t.Name, memberID, activityType, message)
	}
}

// GetConversationID returns the current conversation ID
func (t *Team) GetConversationID() string {
	return t.conversationID
}

// Status returns the team's current status
func (t *Team) Status() map[string]interface{} {
	members := make([]map[string]interface{}, 0)
	for _, m := range t.ListMembers() {
		members = append(members, map[string]interface{}{
			"id":           m.ID,
			"name":         m.Name,
			"display_name": m.DisplayName(),
			"role":         m.RoleName,
			"title":        m.Role.Title,
			"status":       m.GetStatus(),
			"task":         m.GetCurrentTask(),
		})
	}

	tasks := t.ListTasks()
	pending := 0
	inProgress := 0
	completed := 0
	for _, task := range tasks {
		switch task.Status {
		case TaskPending:
			pending++
		case TaskInProgress, TaskAssigned:
			inProgress++
		case TaskCompleted:
			completed++
		}
	}

	return map[string]interface{}{
		"name":         t.Name,
		"description":  t.Spec.Metadata.Description,
		"members":      members,
		"member_count": len(t.Members),
		"tasks": map[string]int{
			"pending":     pending,
			"in_progress": inProgress,
			"completed":   completed,
			"total":       len(tasks),
		},
		"client_facing": t.ClientFacing,
	}
}
