package team

import (
	"context"
	"testing"

	"github.com/arcslash/ugudu/internal/logger"
	"github.com/arcslash/ugudu/internal/provider"
)

// MockProvider implements provider.Provider for testing
type MockProvider struct {
	ChatFunc func(*provider.ChatRequest) (*provider.ChatResponse, error)
}

func (m *MockProvider) ID() string                   { return "mock" }
func (m *MockProvider) Name() string                 { return "Mock Provider" }
func (m *MockProvider) Ping(_ context.Context) error { return nil }
func (m *MockProvider) ListModels(_ context.Context) ([]provider.ModelInfo, error) {
	return []provider.ModelInfo{{ID: "mock-model", Name: "Mock Model"}}, nil
}
func (m *MockProvider) Stream(_ context.Context, _ *provider.ChatRequest) (<-chan provider.StreamChunk, error) {
	ch := make(chan provider.StreamChunk)
	close(ch)
	return ch, nil
}
func (m *MockProvider) Chat(_ context.Context, req *provider.ChatRequest) (*provider.ChatResponse, error) {
	if m.ChatFunc != nil {
		return m.ChatFunc(req)
	}
	return &provider.ChatResponse{Content: "Mock response"}, nil
}

func TestMember_ContextTracking(t *testing.T) {
	log := logger.New("error")

	// Create a minimal team for testing
	spec := &TeamSpec{
		Metadata: Metadata{Name: "test-team"},
		Roles: map[string]Role{
			"pm": {
				Title:   "PM",
				Count:   1,
				Model:   ModelConfig{Provider: "mock", Model: "mock-model"},
				Persona: "You are a PM.",
			},
		},
	}

	// Track saved context
	var savedContext []struct {
		memberID string
		role     string
		content  string
		sequence int
	}

	team := &Team{
		Name:          "test-team",
		Spec:          spec,
		Members:       make(map[string]*Member),
		MembersByRole: make(map[string][]*Member),
		persistence: &PersistenceCallbacks{
			SaveContext: func(teamName, memberID, convID, role, content string, seq int) error {
				savedContext = append(savedContext, struct {
					memberID string
					role     string
					content  string
					sequence int
				}{memberID, role, content, seq})
				return nil
			},
		},
		logger: log,
	}

	mockProv := &MockProvider{}
	member := NewMember("pm", "Sarah", "pm", spec.Roles["pm"], team, mockProv, log)
	team.Members["pm"] = member
	team.MembersByRole["pm"] = []*Member{member}

	// Test addToContext
	member.addToContext("user", "Hello!")
	member.addToContext("assistant", "Hi there!")
	member.addToContext("user", "How are you?")

	// Verify context was saved
	if len(savedContext) != 3 {
		t.Errorf("Expected 3 saved context entries, got %d", len(savedContext))
	}

	// Verify sequence numbers
	if savedContext[0].sequence != 1 || savedContext[1].sequence != 2 || savedContext[2].sequence != 3 {
		t.Errorf("Sequence numbers incorrect: %v", savedContext)
	}

	// Verify in-memory context
	ctx := member.getContextMessages()
	if len(ctx) != 3 {
		t.Errorf("Expected 3 context messages, got %d", len(ctx))
	}
	if ctx[0].Role != "user" || ctx[0].Content != "Hello!" {
		t.Errorf("First context message incorrect: %+v", ctx[0])
	}
}

func TestMember_RestoreContext(t *testing.T) {
	log := logger.New("error")

	spec := &TeamSpec{
		Metadata: Metadata{Name: "test-team"},
		Roles: map[string]Role{
			"pm": {
				Title:   "PM",
				Count:   1,
				Model:   ModelConfig{Provider: "mock", Model: "mock-model"},
				Persona: "You are a PM.",
			},
		},
	}

	team := &Team{
		Name:          "test-team",
		Spec:          spec,
		Members:       make(map[string]*Member),
		MembersByRole: make(map[string][]*Member),
		logger:        log,
	}

	mockProv := &MockProvider{}
	member := NewMember("pm", "Sarah", "pm", spec.Roles["pm"], team, mockProv, log)

	// Restore context from "persisted" history
	history := []ContextMessage{
		{Role: "user", Content: "Previous message 1"},
		{Role: "assistant", Content: "Previous response 1"},
		{Role: "user", Content: "Previous message 2"},
	}

	member.RestoreContext(history)

	// Verify context was restored
	ctx := member.getContextMessages()
	if len(ctx) != 3 {
		t.Errorf("Expected 3 context messages after restore, got %d", len(ctx))
	}
	if ctx[0].Role != "user" || ctx[0].Content != "Previous message 1" {
		t.Errorf("First restored message incorrect: %+v", ctx[0])
	}

	// Verify sequence counter was updated
	if member.contextSequence != 3 {
		t.Errorf("Expected sequence 3 after restore, got %d", member.contextSequence)
	}
}

func TestMember_ClearContext(t *testing.T) {
	log := logger.New("error")

	spec := &TeamSpec{
		Metadata: Metadata{Name: "test-team"},
		Roles: map[string]Role{
			"pm": {
				Title: "PM",
				Count: 1,
				Model: ModelConfig{Provider: "mock", Model: "mock-model"},
			},
		},
	}

	team := &Team{
		Name:          "test-team",
		Spec:          spec,
		Members:       make(map[string]*Member),
		MembersByRole: make(map[string][]*Member),
		logger:        log,
	}

	mockProv := &MockProvider{}
	member := NewMember("pm", "Sarah", "pm", spec.Roles["pm"], team, mockProv, log)

	// Add some context
	member.RestoreContext([]ContextMessage{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi"},
	})

	// Clear context
	member.ClearContext()

	ctx := member.getContextMessages()
	if len(ctx) != 0 {
		t.Errorf("Expected 0 context messages after clear, got %d", len(ctx))
	}
	if member.contextSequence != 0 {
		t.Errorf("Expected sequence 0 after clear, got %d", member.contextSequence)
	}
}

func TestMember_ContextTrimming(t *testing.T) {
	log := logger.New("error")

	spec := &TeamSpec{
		Metadata: Metadata{Name: "test-team"},
		Roles: map[string]Role{
			"pm": {
				Title:   "PM",
				Count:   1,
				Model:   ModelConfig{Provider: "mock", Model: "mock-model"},
				Persona: "You are a PM.",
			},
		},
	}

	team := &Team{
		Name:          "test-team",
		Spec:          spec,
		Members:       make(map[string]*Member),
		MembersByRole: make(map[string][]*Member),
		persistence: &PersistenceCallbacks{
			SaveContext: func(string, string, string, string, string, int) error { return nil },
		},
		logger: log,
	}

	mockProv := &MockProvider{}
	member := NewMember("pm", "Sarah", "pm", spec.Roles["pm"], team, mockProv, log)

	// Add more than 40 messages (the trim limit)
	for i := 0; i < 50; i++ {
		if i%2 == 0 {
			member.addToContext("user", "Message")
		} else {
			member.addToContext("assistant", "Response")
		}
	}

	// Context should be trimmed to 40
	ctx := member.getContextMessages()
	if len(ctx) != 40 {
		t.Errorf("Expected context to be trimmed to 40, got %d", len(ctx))
	}
}

func TestMember_ContextThreadSafety(t *testing.T) {
	log := logger.New("error")

	spec := &TeamSpec{
		Metadata: Metadata{Name: "test-team"},
		Roles: map[string]Role{
			"pm": {
				Title: "PM",
				Count: 1,
				Model: ModelConfig{Provider: "mock", Model: "mock-model"},
			},
		},
	}

	team := &Team{
		Name:          "test-team",
		Spec:          spec,
		Members:       make(map[string]*Member),
		MembersByRole: make(map[string][]*Member),
		persistence: &PersistenceCallbacks{
			SaveContext: func(string, string, string, string, string, int) error { return nil },
		},
		logger: log,
	}

	mockProv := &MockProvider{}
	member := NewMember("pm", "Sarah", "pm", spec.Roles["pm"], team, mockProv, log)

	// Concurrent access
	done := make(chan bool)

	go func() {
		for i := 0; i < 100; i++ {
			member.addToContext("user", "Message")
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			member.getContextMessages()
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			member.ClearContext()
		}
		done <- true
	}()

	<-done
	<-done
	<-done

	// Should complete without panics
}
