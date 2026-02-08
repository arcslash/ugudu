package provider

import (
	"fmt"
	"os"
	"sync"
)

// Registry manages available providers
type Registry struct {
	providers map[string]Provider
	mu        sync.RWMutex
}

// NewRegistry creates a new provider registry
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// Register adds a provider to the registry
func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.ID()] = p
}

// Get returns a provider by ID
func (r *Registry) Get(id string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if p, ok := r.providers[id]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("provider not found: %s", id)
}

// List returns all registered providers
func (r *Registry) List() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		providers = append(providers, p)
	}
	return providers
}

// Has checks if a provider is registered
func (r *Registry) Has(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.providers[id]
	return ok
}

// AutoDiscover registers providers based on environment variables
func (r *Registry) AutoDiscover() {
	// Anthropic
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		r.Register(NewAnthropic(key, ""))
	}

	// OpenAI
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		r.Register(NewOpenAI(key, ""))
	}

	// Ollama (local, no key needed)
	ollamaURL := os.Getenv("OLLAMA_URL")
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}
	r.Register(NewOllama(ollamaURL))

	// Groq
	if key := os.Getenv("GROQ_API_KEY"); key != "" {
		r.Register(NewGroq(key))
	}

	// OpenRouter (access to many models: Claude, GPT, Gemini, Mistral, DeepSeek, etc.)
	if key := os.Getenv("OPENROUTER_API_KEY"); key != "" {
		siteName := os.Getenv("OPENROUTER_SITE_NAME")
		siteURL := os.Getenv("OPENROUTER_SITE_URL")
		r.Register(NewOpenRouter(key, siteName, siteURL))
	}
}
