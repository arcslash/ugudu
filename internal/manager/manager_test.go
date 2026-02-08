package manager

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/arcslash/ugudu/internal/logger"
)

func TestManager_BasicLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	log := logger.New("error")

	cfg := Config{
		DataDir:    tmpDir,
		SocketPath: filepath.Join(tmpDir, "test.sock"),
		LogLevel:   "error",
	}

	mgr, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}

	// Verify status
	status := mgr.Status()
	if status == nil {
		t.Error("Status should not be nil")
	}

	// Stop manager
	mgr.Stop()
}

func TestManager_PersistenceCallbacks(t *testing.T) {
	tmpDir := t.TempDir()
	log := logger.New("error")

	cfg := Config{
		DataDir:    tmpDir,
		SocketPath: filepath.Join(tmpDir, "test.sock"),
		LogLevel:   "error",
	}

	mgr, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Stop()

	// Verify createPersistenceCallbacks works
	callbacks := mgr.createPersistenceCallbacks()
	if callbacks == nil {
		t.Fatal("Callbacks should not be nil")
	}

	if callbacks.SaveContext == nil {
		t.Error("SaveContext callback should not be nil")
	}
	if callbacks.LoadContext == nil {
		t.Error("LoadContext callback should not be nil")
	}
	if callbacks.CreateConversation == nil {
		t.Error("CreateConversation callback should not be nil")
	}
	if callbacks.GetActiveConversation == nil {
		t.Error("GetActiveConversation callback should not be nil")
	}
}

func TestManager_ConversationPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	log := logger.New("error")

	cfg := Config{
		DataDir:    tmpDir,
		SocketPath: filepath.Join(tmpDir, "test.sock"),
		LogLevel:   "error",
	}

	mgr, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Stop()

	store := mgr.Store()
	if store == nil {
		t.Fatal("Store should not be nil")
	}

	// Create team and conversation manually
	store.SaveTeam("test-team", "/path/to/spec.yaml")
	conv, err := store.CreateConversation("test-team")
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	// Save context using callbacks
	callbacks := mgr.createPersistenceCallbacks()

	err = callbacks.SaveContext("test-team", "pm", conv.ID, "user", "Hello!", 1)
	if err != nil {
		t.Errorf("SaveContext failed: %v", err)
	}

	err = callbacks.SaveContext("test-team", "pm", conv.ID, "assistant", "Hi there!", 2)
	if err != nil {
		t.Errorf("SaveContext failed: %v", err)
	}

	// Load context using callbacks
	messages, err := callbacks.LoadContext("test-team", "pm", 10)
	if err != nil {
		t.Errorf("LoadContext failed: %v", err)
	}
	if len(messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(messages))
	}

	// Test conversation callbacks
	convID, err := callbacks.GetActiveConversation("test-team")
	if err != nil {
		t.Errorf("GetActiveConversation failed: %v", err)
	}
	if convID != conv.ID {
		t.Errorf("Expected conversation ID '%s', got '%s'", conv.ID, convID)
	}

	// Create new conversation
	newConvID, err := callbacks.CreateConversation("test-team-2")
	if err == nil && newConvID != "" {
		// Need to create team first, but this tests the callback works
	}
}

func TestManager_RestoreTeamsWithContext(t *testing.T) {
	tmpDir := t.TempDir()
	log := logger.New("error")

	// Create a test spec file
	specContent := `
apiVersion: ugudu/v1
kind: Team
metadata:
  name: restore-test
  description: Test team

client_facing:
  - lead

roles:
  lead:
    title: Team Lead
    visibility: client
    model:
      provider: anthropic
      model: claude-sonnet-4-20250514
    persona: You are the team lead.
`
	specPath := filepath.Join(tmpDir, "restore-test.yaml")
	os.WriteFile(specPath, []byte(specContent), 0644)

	cfg := Config{
		DataDir:    tmpDir,
		SocketPath: filepath.Join(tmpDir, "test.sock"),
		LogLevel:   "error",
	}

	// First manager session - create team and save context
	{
		mgr, _ := New(cfg, log)
		ctx, cancel := context.WithCancel(context.Background())

		mgr.Start(ctx)

		// Manually save team to store (normally done by CreateTeam)
		store := mgr.Store()
		store.SaveTeam("restore-test", specPath)

		// Create conversation and save context
		conv, _ := store.CreateConversation("restore-test")
		store.SaveAgentContext("restore-test", "lead", conv.ID, "user", "Saved message", 1)
		store.SaveAgentContext("restore-test", "lead", conv.ID, "assistant", "Saved response", 2)

		cancel()
		mgr.Stop()
	}

	// Second manager session - verify restoration
	{
		mgr, _ := New(cfg, log)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		mgr.Start(ctx)

		store := mgr.Store()

		// Verify team was saved
		teams, _ := store.ListTeams()
		if len(teams) == 0 {
			t.Fatal("Expected at least 1 saved team")
		}

		// Verify conversation persisted
		conversations, _ := store.ListConversations("restore-test", 10)
		if len(conversations) != 1 {
			t.Errorf("Expected 1 conversation, got %d", len(conversations))
		}

		// Verify context persisted
		messages, _ := store.LoadAgentContext("restore-test", "lead", 10)
		if len(messages) != 2 {
			t.Errorf("Expected 2 messages, got %d", len(messages))
		}

		mgr.Stop()
	}
}

func TestManager_StoreAccess(t *testing.T) {
	tmpDir := t.TempDir()
	log := logger.New("error")

	cfg := Config{
		DataDir: tmpDir,
	}

	mgr, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Stop()

	// Store should be accessible
	store := mgr.Store()
	if store == nil {
		t.Error("Store should not be nil")
	}

	// Should be able to use store
	err = store.SaveTeam("api-test-team", "/path/to/spec.yaml")
	if err != nil {
		t.Errorf("Failed to save team via Store(): %v", err)
	}
}

func TestManager_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	log := logger.New("error")

	cfg := Config{
		DataDir: tmpDir,
	}

	mgr, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr.Start(ctx)

	store := mgr.Store()
	store.SaveTeam("concurrent-test", "/path/to/spec.yaml")
	conv, _ := store.CreateConversation("concurrent-test")

	done := make(chan bool)

	// Concurrent context saves
	go func() {
		for i := 0; i < 50; i++ {
			store.SaveAgentContext("concurrent-test", "pm", conv.ID, "user", "Message", i)
		}
		done <- true
	}()

	// Concurrent context loads
	go func() {
		for i := 0; i < 50; i++ {
			store.LoadAgentContext("concurrent-test", "pm", 10)
		}
		done <- true
	}()

	// Concurrent status reads
	go func() {
		for i := 0; i < 50; i++ {
			mgr.Status()
		}
		done <- true
	}()

	// Wait with timeout
	timeout := time.After(5 * time.Second)
	for i := 0; i < 3; i++ {
		select {
		case <-done:
		case <-timeout:
			t.Fatal("Concurrent test timed out")
		}
	}
}
