package provider

import (
	"context"
	"os"
	"testing"
)

func TestNewRegistry(t *testing.T) {
	reg := NewRegistry()
	if reg == nil {
		t.Fatal("NewRegistry returned nil")
	}

	if reg.providers == nil {
		t.Fatal("providers map is nil")
	}
}

func TestRegisterAndGet(t *testing.T) {
	reg := NewRegistry()

	// Create a mock provider
	mock := &mockProvider{id: "test", name: "Test Provider"}
	reg.Register(mock)

	// Get the provider
	p, err := reg.Get("test")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if p.ID() != "test" {
		t.Errorf("Expected ID 'test', got '%s'", p.ID())
	}

	if p.Name() != "Test Provider" {
		t.Errorf("Expected name 'Test Provider', got '%s'", p.Name())
	}
}

func TestGetNonExistent(t *testing.T) {
	reg := NewRegistry()

	_, err := reg.Get("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent provider")
	}
}

func TestList(t *testing.T) {
	reg := NewRegistry()

	reg.Register(&mockProvider{id: "a", name: "Provider A"})
	reg.Register(&mockProvider{id: "b", name: "Provider B"})

	providers := reg.List()
	if len(providers) != 2 {
		t.Errorf("Expected 2 providers, got %d", len(providers))
	}
}

func TestAutoDiscoverNoKeys(t *testing.T) {
	// Clear all API keys
	os.Unsetenv("ANTHROPIC_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("GROQ_API_KEY")
	os.Unsetenv("OLLAMA_URL")

	reg := NewRegistry()
	reg.AutoDiscover()

	// Should have no providers (or just ollama if it's running locally)
	providers := reg.List()
	t.Logf("Auto-discovered %d providers without keys", len(providers))
}

func TestAutoDiscoverWithAnthropicKey(t *testing.T) {
	// Set Anthropic key
	os.Setenv("ANTHROPIC_API_KEY", "test-key")
	defer os.Unsetenv("ANTHROPIC_API_KEY")

	// Clear others
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("GROQ_API_KEY")
	os.Unsetenv("OLLAMA_URL")

	reg := NewRegistry()
	reg.AutoDiscover()

	// Should have Anthropic provider
	_, err := reg.Get("anthropic")
	if err != nil {
		t.Error("Anthropic provider should be registered when API key is set")
	}
}

func TestAutoDiscoverWithOpenAIKey(t *testing.T) {
	// Set OpenAI key
	os.Setenv("OPENAI_API_KEY", "test-key")
	defer os.Unsetenv("OPENAI_API_KEY")

	// Clear others
	os.Unsetenv("ANTHROPIC_API_KEY")
	os.Unsetenv("GROQ_API_KEY")
	os.Unsetenv("OLLAMA_URL")

	reg := NewRegistry()
	reg.AutoDiscover()

	// Should have OpenAI provider
	_, err := reg.Get("openai")
	if err != nil {
		t.Error("OpenAI provider should be registered when API key is set")
	}
}

func TestAutoDiscoverWithGroqKey(t *testing.T) {
	// Set Groq key
	os.Setenv("GROQ_API_KEY", "test-key")
	defer os.Unsetenv("GROQ_API_KEY")

	// Clear others
	os.Unsetenv("ANTHROPIC_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("OLLAMA_URL")

	reg := NewRegistry()
	reg.AutoDiscover()

	// Should have Groq provider
	_, err := reg.Get("groq")
	if err != nil {
		t.Error("Groq provider should be registered when API key is set")
	}
}

func TestAutoDiscoverWithOllamaURL(t *testing.T) {
	// Set Ollama URL
	os.Setenv("OLLAMA_URL", "http://localhost:11434")
	defer os.Unsetenv("OLLAMA_URL")

	// Clear others
	os.Unsetenv("ANTHROPIC_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("GROQ_API_KEY")

	reg := NewRegistry()
	reg.AutoDiscover()

	// Should have Ollama provider
	_, err := reg.Get("ollama")
	if err != nil {
		t.Error("Ollama provider should be registered when URL is set")
	}
}

// mockProvider is a simple mock for testing
type mockProvider struct {
	id   string
	name string
}

func (m *mockProvider) ID() string   { return m.id }
func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	return &ChatResponse{Content: "mock response"}, nil
}
func (m *mockProvider) Stream(ctx context.Context, req *ChatRequest) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk)
	close(ch)
	return ch, nil
}
func (m *mockProvider) ListModels(ctx context.Context) ([]ModelInfo, error) {
	return []ModelInfo{{ID: "mock-model", Name: "Mock Model"}}, nil
}
func (m *mockProvider) Ping(ctx context.Context) error { return nil }
