// Package manager provides the central control plane for Ugudu
package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/arcslash/ugudu/internal/logger"
	"github.com/arcslash/ugudu/internal/provider"
	"github.com/arcslash/ugudu/internal/team"
)

// ActivityCallback is called when team activity occurs
type ActivityCallback func(teamName, memberID, activityType, message string)

// Manager is the central controller for all teams
type Manager struct {
	teams      map[string]*team.Team
	providers  *provider.Registry
	store      *Store
	config     Config
	onActivity ActivityCallback

	ctx    context.Context
	cancel context.CancelFunc
	logger *logger.Logger
	mu     sync.RWMutex
}

// SetActivityCallback sets the callback for team activity events
func (m *Manager) SetActivityCallback(cb ActivityCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onActivity = cb
}

// Config holds manager configuration
type Config struct {
	DataDir    string `yaml:"data_dir"`
	SocketPath string `yaml:"socket_path"`
	LogLevel   string `yaml:"log_level"`
	LogFormat  string `yaml:"log_format"`
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	homeDir, _ := os.UserHomeDir()
	return Config{
		DataDir:    filepath.Join(homeDir, ".ugudu"),
		SocketPath: filepath.Join(homeDir, ".ugudu", "ugudu.sock"),
		LogLevel:   "info",
		LogFormat:  "text",
	}
}

// New creates a new Manager
func New(cfg Config, log *logger.Logger) (*Manager, error) {
	// Ensure data directory exists
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	// Initialize provider registry
	providers := provider.NewRegistry()
	providers.AutoDiscover()

	// Initialize store
	store, err := NewStore(filepath.Join(cfg.DataDir, "ugudu.db"))
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}

	m := &Manager{
		teams:     make(map[string]*team.Team),
		providers: providers,
		store:     store,
		config:    cfg,
		logger:    log,
	}

	return m, nil
}

// Start begins the manager
func (m *Manager) Start(ctx context.Context) error {
	m.ctx, m.cancel = context.WithCancel(ctx)

	// Restore teams from store
	if err := m.restoreTeams(); err != nil {
		m.logger.Warn("failed to restore teams", "error", err)
	}

	m.logger.Info("manager started", "data_dir", m.config.DataDir)
	return nil
}

// Stop halts the manager and all teams
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, t := range m.teams {
		t.Stop()
		m.logger.Info("stopped team", "name", name)
	}

	if m.store != nil {
		m.store.Close()
	}

	if m.cancel != nil {
		m.cancel()
	}

	m.logger.Info("manager stopped")
}

// CreateTeam creates and registers a new team from a spec file
func (m *Manager) CreateTeam(specPath string) (*team.Team, error) {
	spec, err := team.LoadSpec(specPath)
	if err != nil {
		return nil, fmt.Errorf("load spec: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.teams[spec.Metadata.Name]; exists {
		return nil, fmt.Errorf("team %s already exists", spec.Metadata.Name)
	}

	t, err := team.NewTeamWithPersistence(spec, m.providers, m.logger, m.createPersistenceCallbacks())
	if err != nil {
		return nil, fmt.Errorf("create team: %w", err)
	}

	m.teams[spec.Metadata.Name] = t

	// Persist
	if err := m.store.SaveTeam(spec.Metadata.Name, specPath); err != nil {
		m.logger.Warn("failed to persist team", "error", err)
	}

	m.logger.Info("team created", "name", spec.Metadata.Name)
	return t, nil
}

// CreateTeamWithName creates a team with a custom instance name
func (m *Manager) CreateTeamWithName(name, specPath string) (*team.Team, error) {
	spec, err := team.LoadSpec(specPath)
	if err != nil {
		return nil, fmt.Errorf("load spec: %w", err)
	}

	// Override the name if provided
	if name != "" {
		spec.Metadata.Name = name
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.teams[spec.Metadata.Name]; exists {
		return nil, fmt.Errorf("team %s already exists", spec.Metadata.Name)
	}

	t, err := team.NewTeamWithPersistence(spec, m.providers, m.logger, m.createPersistenceCallbacks())
	if err != nil {
		return nil, fmt.Errorf("create team: %w", err)
	}

	m.teams[spec.Metadata.Name] = t

	// Persist
	if err := m.store.SaveTeam(spec.Metadata.Name, specPath); err != nil {
		m.logger.Warn("failed to persist team", "error", err)
	}

	m.logger.Info("team created", "name", spec.Metadata.Name)
	return t, nil
}

// createPersistenceCallbacks returns callbacks for team persistence
func (m *Manager) createPersistenceCallbacks() *team.PersistenceCallbacks {
	return &team.PersistenceCallbacks{
		SaveContext: func(teamName, memberID, conversationID, role, content string, sequence int) error {
			return m.store.SaveAgentContext(teamName, memberID, conversationID, role, content, sequence)
		},
		LoadContext: func(teamName, memberID string, limit int) ([]team.ContextMessage, error) {
			messages, err := m.store.LoadAgentContext(teamName, memberID, limit)
			if err != nil {
				return nil, err
			}
			result := make([]team.ContextMessage, len(messages))
			for i, msg := range messages {
				result[i] = team.ContextMessage{
					Role:    msg.Role,
					Content: msg.Content,
				}
			}
			return result, nil
		},
		CreateConversation: func(teamName string) (string, error) {
			conv, err := m.store.CreateConversation(teamName)
			if err != nil {
				return "", err
			}
			return conv.ID, nil
		},
		GetActiveConversation: func(teamName string) (string, error) {
			conv, err := m.store.GetActiveConversation(teamName)
			if err != nil {
				return "", err
			}
			if conv == nil {
				return "", nil
			}
			return conv.ID, nil
		},
		OnActivity: func(teamName, memberID, activityType, message string) {
			m.mu.RLock()
			cb := m.onActivity
			m.mu.RUnlock()
			if cb != nil {
				cb(teamName, memberID, activityType, message)
			}
		},
	}
}

// StartTeam starts a team by name
func (m *Manager) StartTeam(name string) error {
	m.mu.RLock()
	t, ok := m.teams[name]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("team not found: %s", name)
	}

	if err := t.Start(m.ctx); err != nil {
		return fmt.Errorf("start team: %w", err)
	}

	// Update store
	m.store.UpdateTeamStatus(name, "running")

	return nil
}

// StopTeam stops a team by name
func (m *Manager) StopTeam(name string) error {
	m.mu.RLock()
	t, ok := m.teams[name]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("team not found: %s", name)
	}

	t.Stop()
	m.store.UpdateTeamStatus(name, "stopped")

	return nil
}

// DeleteTeam removes a team
func (m *Manager) DeleteTeam(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.teams[name]
	if !ok {
		return fmt.Errorf("team not found: %s", name)
	}

	t.Stop()
	delete(m.teams, name)
	m.store.DeleteTeam(name)

	m.logger.Info("team deleted", "name", name)
	return nil
}

// GetTeam returns a team by name
func (m *Manager) GetTeam(name string) (*team.Team, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.teams[name]
	if !ok {
		return nil, fmt.Errorf("team not found: %s", name)
	}
	return t, nil
}

// ListTeams returns all teams
func (m *Manager) ListTeams() []*team.Team {
	m.mu.RLock()
	defer m.mu.RUnlock()

	teams := make([]*team.Team, 0, len(m.teams))
	for _, t := range m.teams {
		teams = append(teams, t)
	}
	return teams
}

// Ask sends a message to a team
func (m *Manager) Ask(teamName, message string) (<-chan team.Message, error) {
	t, err := m.GetTeam(teamName)
	if err != nil {
		return nil, err
	}

	return t.Ask(message), nil
}

// AskMember sends a message to a specific team member
func (m *Manager) AskMember(teamName, role, message string) (<-chan team.Message, error) {
	t, err := m.GetTeam(teamName)
	if err != nil {
		return nil, err
	}

	return t.AskMember(role, message), nil
}

// Status returns overall manager status
func (m *Manager) Status() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	teams := make([]map[string]interface{}, 0)
	for _, t := range m.teams {
		teams = append(teams, t.Status())
	}

	providers := make([]map[string]interface{}, 0)
	for _, p := range m.providers.List() {
		providers = append(providers, map[string]interface{}{
			"id":   p.ID(),
			"name": p.Name(),
		})
	}

	return map[string]interface{}{
		"teams":      teams,
		"team_count": len(m.teams),
		"providers":  providers,
		"data_dir":   m.config.DataDir,
	}
}

// Providers returns the provider registry
func (m *Manager) Providers() *provider.Registry {
	return m.providers
}

// Store returns the persistence store
func (m *Manager) Store() *Store {
	return m.store
}

// RestoreTeams restores teams from persistent storage
func (m *Manager) RestoreTeams() error {
	return m.restoreTeams()
}

func (m *Manager) restoreTeams() error {
	teams, err := m.store.ListTeams()
	if err != nil {
		return err
	}

	for _, saved := range teams {
		if saved.SpecPath == "" {
			continue
		}

		spec, err := team.LoadSpec(saved.SpecPath)
		if err != nil {
			m.logger.Warn("failed to load saved team spec", "name", saved.Name, "error", err)
			continue
		}

		// Override spec name with saved team name (team instance name may differ from spec name)
		spec.Metadata.Name = saved.Name

		// Create team with persistence callbacks for context restoration
		t, err := team.NewTeamWithPersistence(spec, m.providers, m.logger, m.createPersistenceCallbacks())
		if err != nil {
			m.logger.Warn("failed to restore team", "name", saved.Name, "error", err)
			continue
		}

		m.teams[saved.Name] = t
		m.logger.Info("team restored", "name", saved.Name)

		// Auto-start if it was running
		if saved.Status == "running" {
			go func(name string) {
				time.Sleep(100 * time.Millisecond)
				if err := m.StartTeam(name); err != nil {
					m.logger.Warn("failed to auto-start team", "name", name, "error", err)
				} else {
					m.logger.Info("team auto-started with context restored", "name", name)
				}
			}(saved.Name)
		}
	}

	return nil
}

// SavedTeam represents a persisted team record
type SavedTeam struct {
	Name     string `json:"name"`
	SpecPath string `json:"spec_path"`
	Status   string `json:"status"`
}

// ToJSON serializes manager status to JSON
func (m *Manager) ToJSON() ([]byte, error) {
	return json.MarshalIndent(m.Status(), "", "  ")
}
