package manager

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStore_BasicOperations(t *testing.T) {
	// Create temp database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Test SaveTeam
	err = store.SaveTeam("test-team", "/path/to/spec.yaml")
	if err != nil {
		t.Errorf("SaveTeam failed: %v", err)
	}

	// Test GetTeam
	team, err := store.GetTeam("test-team")
	if err != nil {
		t.Errorf("GetTeam failed: %v", err)
	}
	if team == nil {
		t.Fatal("Team should not be nil")
	}
	if team.Name != "test-team" {
		t.Errorf("Expected team name 'test-team', got '%s'", team.Name)
	}
	if team.SpecPath != "/path/to/spec.yaml" {
		t.Errorf("Expected spec path '/path/to/spec.yaml', got '%s'", team.SpecPath)
	}

	// Test ListTeams
	teams, err := store.ListTeams()
	if err != nil {
		t.Errorf("ListTeams failed: %v", err)
	}
	if len(teams) != 1 {
		t.Errorf("Expected 1 team, got %d", len(teams))
	}

	// Test UpdateTeamStatus
	err = store.UpdateTeamStatus("test-team", "running")
	if err != nil {
		t.Errorf("UpdateTeamStatus failed: %v", err)
	}

	team, _ = store.GetTeam("test-team")
	if team.Status != "running" {
		t.Errorf("Expected status 'running', got '%s'", team.Status)
	}

	// Test DeleteTeam
	err = store.DeleteTeam("test-team")
	if err != nil {
		t.Errorf("DeleteTeam failed: %v", err)
	}

	team, _ = store.GetTeam("test-team")
	if team != nil {
		t.Error("Team should be nil after deletion")
	}
}

func TestStore_Conversations(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create a team first
	store.SaveTeam("test-team", "/path/to/spec.yaml")

	// Test CreateConversation
	conv, err := store.CreateConversation("test-team")
	if err != nil {
		t.Fatalf("CreateConversation failed: %v", err)
	}
	if conv == nil {
		t.Fatal("Conversation should not be nil")
	}
	if conv.TeamName != "test-team" {
		t.Errorf("Expected team name 'test-team', got '%s'", conv.TeamName)
	}
	if conv.Status != "active" {
		t.Errorf("Expected status 'active', got '%s'", conv.Status)
	}

	// Test GetActiveConversation
	active, err := store.GetActiveConversation("test-team")
	if err != nil {
		t.Errorf("GetActiveConversation failed: %v", err)
	}
	if active == nil {
		t.Fatal("Active conversation should not be nil")
	}
	if active.ID != conv.ID {
		t.Errorf("Expected conversation ID '%s', got '%s'", conv.ID, active.ID)
	}

	// Test ListConversations
	conversations, err := store.ListConversations("test-team", 10)
	if err != nil {
		t.Errorf("ListConversations failed: %v", err)
	}
	if len(conversations) != 1 {
		t.Errorf("Expected 1 conversation, got %d", len(conversations))
	}

	// Test CloseConversation
	err = store.CloseConversation(conv.ID)
	if err != nil {
		t.Errorf("CloseConversation failed: %v", err)
	}

	// After closing, GetActiveConversation should return nil
	active, _ = store.GetActiveConversation("test-team")
	if active != nil {
		t.Error("Should have no active conversation after closing")
	}
}

func TestStore_AgentContext(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create team and conversation
	store.SaveTeam("test-team", "/path/to/spec.yaml")
	conv, _ := store.CreateConversation("test-team")

	// Test SaveAgentContext
	err = store.SaveAgentContext("test-team", "pm", conv.ID, "user", "Hello, how are you?", 1)
	if err != nil {
		t.Errorf("SaveAgentContext (user) failed: %v", err)
	}

	err = store.SaveAgentContext("test-team", "pm", conv.ID, "assistant", "I'm doing well, thank you!", 2)
	if err != nil {
		t.Errorf("SaveAgentContext (assistant) failed: %v", err)
	}

	err = store.SaveAgentContext("test-team", "pm", conv.ID, "user", "Can you help me with a task?", 3)
	if err != nil {
		t.Errorf("SaveAgentContext (user 2) failed: %v", err)
	}

	// Test LoadAgentContext
	messages, err := store.LoadAgentContext("test-team", "pm", 10)
	if err != nil {
		t.Errorf("LoadAgentContext failed: %v", err)
	}
	if len(messages) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(messages))
	}

	// Verify order (should be chronological)
	if messages[0].Role != "user" || messages[0].Content != "Hello, how are you?" {
		t.Errorf("First message incorrect: %+v", messages[0])
	}
	if messages[1].Role != "assistant" {
		t.Errorf("Second message should be assistant, got %s", messages[1].Role)
	}
	if messages[2].Role != "user" {
		t.Errorf("Third message should be user, got %s", messages[2].Role)
	}

	// Test LoadAgentContext with limit
	messages, _ = store.LoadAgentContext("test-team", "pm", 2)
	if len(messages) != 2 {
		t.Errorf("Expected 2 messages with limit, got %d", len(messages))
	}

	// Test ClearAgentContext
	err = store.ClearAgentContext("test-team", "pm")
	if err != nil {
		t.Errorf("ClearAgentContext failed: %v", err)
	}

	messages, _ = store.LoadAgentContext("test-team", "pm", 10)
	if len(messages) != 0 {
		t.Errorf("Expected 0 messages after clear, got %d", len(messages))
	}
}

func TestStore_ConversationHistory(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create team and conversation
	store.SaveTeam("test-team", "/path/to/spec.yaml")
	conv, _ := store.CreateConversation("test-team")

	// Add messages from multiple members
	store.SaveAgentContext("test-team", "pm", conv.ID, "user", "Task request", 1)
	store.SaveAgentContext("test-team", "pm", conv.ID, "assistant", "PM response", 2)
	store.SaveAgentContext("test-team", "engineer", conv.ID, "user", "Delegated task", 1)
	store.SaveAgentContext("test-team", "engineer", conv.ID, "assistant", "Engineer response", 2)

	// Test GetConversationHistory
	history, err := store.GetConversationHistory(conv.ID)
	if err != nil {
		t.Errorf("GetConversationHistory failed: %v", err)
	}
	if len(history) != 4 {
		t.Errorf("Expected 4 messages in history, got %d", len(history))
	}
}

func TestStore_MultipleConversations(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	store.SaveTeam("test-team", "/path/to/spec.yaml")

	// Create multiple conversations
	conv1, _ := store.CreateConversation("test-team")
	time.Sleep(10 * time.Millisecond) // Ensure different timestamps

	store.CloseConversation(conv1.ID)

	conv2, _ := store.CreateConversation("test-team")

	// GetActiveConversation should return the latest active one
	active, _ := store.GetActiveConversation("test-team")
	if active.ID != conv2.ID {
		t.Errorf("Expected active conversation to be '%s', got '%s'", conv2.ID, active.ID)
	}

	// ListConversations should return both
	conversations, _ := store.ListConversations("test-team", 10)
	if len(conversations) != 2 {
		t.Errorf("Expected 2 conversations, got %d", len(conversations))
	}
}

func TestStore_NoActiveConversation(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	store.SaveTeam("test-team", "/path/to/spec.yaml")

	// No conversations yet
	active, err := store.GetActiveConversation("test-team")
	if err != nil {
		t.Errorf("GetActiveConversation should not error: %v", err)
	}
	if active != nil {
		t.Error("Should have no active conversation")
	}
}

func TestStore_PersistenceAcrossRestarts(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// First session - create data
	{
		store, _ := NewStore(dbPath)
		store.SaveTeam("test-team", "/path/to/spec.yaml")
		conv, _ := store.CreateConversation("test-team")
		store.SaveAgentContext("test-team", "pm", conv.ID, "user", "Hello", 1)
		store.SaveAgentContext("test-team", "pm", conv.ID, "assistant", "Hi there!", 2)
		store.Close()
	}

	// Second session - verify data persisted
	{
		store, err := NewStore(dbPath)
		if err != nil {
			t.Fatalf("Failed to reopen store: %v", err)
		}
		defer store.Close()

		// Team should exist
		team, _ := store.GetTeam("test-team")
		if team == nil {
			t.Fatal("Team should exist after restart")
		}

		// Conversation should exist
		conversations, _ := store.ListConversations("test-team", 10)
		if len(conversations) != 1 {
			t.Fatalf("Expected 1 conversation after restart, got %d", len(conversations))
		}

		// Messages should exist
		messages, _ := store.LoadAgentContext("test-team", "pm", 10)
		if len(messages) != 2 {
			t.Errorf("Expected 2 messages after restart, got %d", len(messages))
		}
		if messages[0].Content != "Hello" {
			t.Errorf("First message content mismatch: %s", messages[0].Content)
		}
		if messages[1].Content != "Hi there!" {
			t.Errorf("Second message content mismatch: %s", messages[1].Content)
		}
	}
}

func TestStore_DBFileCreation(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "subdir", "test.db")

	// Parent dir doesn't exist
	store, err := NewStore(dbPath)
	if err != nil {
		// SQLite should create parent directories or we handle it
		t.Logf("Store creation with missing parent: %v", err)
	} else {
		store.Close()

		// Verify file exists
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			t.Error("Database file should exist")
		}
	}
}
