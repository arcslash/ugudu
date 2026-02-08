// Package api provides the HTTP REST API for Ugudu
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/arcslash/ugudu/internal/config"
	"github.com/arcslash/ugudu/internal/logger"
	"github.com/arcslash/ugudu/internal/manager"
	"github.com/arcslash/ugudu/internal/team"
	"github.com/arcslash/ugudu/internal/workspace"
)

// Server is the HTTP API server
type Server struct {
	manager *manager.Manager
	logger  *logger.Logger
	server  *http.Server
	mux     *http.ServeMux
	wsHub   *WSHub
}

// NewServer creates a new API server
func NewServer(mgr *manager.Manager, log *logger.Logger) *Server {
	s := &Server{
		manager: mgr,
		logger:  log,
		mux:     http.NewServeMux(),
		wsHub:   NewWSHub(),
	}
	go s.wsHub.Run()
	s.setupRoutes()

	// Wire up activity callback to broadcast via WebSocket
	mgr.SetActivityCallback(func(teamName, memberID, activityType, message string) {
		if activityType == "status_change" {
			// Broadcast as member status update
			s.wsHub.BroadcastMemberStatus(teamName, memberID, message, "")
		} else {
			// Broadcast as activity
			s.wsHub.BroadcastActivity(teamName, memberID, message, map[string]string{
				"type": activityType,
			})
		}
	})

	return s
}

func (s *Server) setupRoutes() {
	// CORS middleware wrapper
	cors := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			h(w, r)
		}
	}

	// Health
	s.mux.HandleFunc("/api/health", cors(s.handleHealth))

	// Status
	s.mux.HandleFunc("/api/status", cors(s.handleStatus))

	// Providers
	s.mux.HandleFunc("/api/providers", cors(s.handleProviders))
	s.mux.HandleFunc("/api/providers/", cors(s.handleProviderByID))

	// Settings (API keys, config)
	s.mux.HandleFunc("/api/settings", cors(s.handleSettings))

	// Specs (team blueprints)
	s.mux.HandleFunc("/api/specs", cors(s.handleSpecs))
	s.mux.HandleFunc("/api/specs/", cors(s.handleSpecByName))

	// Teams
	s.mux.HandleFunc("/api/teams", cors(s.handleTeams))
	s.mux.HandleFunc("/api/teams/", cors(s.handleTeamByName))

	// Chat/Ask
	s.mux.HandleFunc("/api/chat", cors(s.handleChat))

	// Projects
	s.mux.HandleFunc("/api/projects", cors(s.handleProjects))
	s.mux.HandleFunc("/api/projects/", cors(s.handleProjectByName))

	// Conversations
	s.mux.HandleFunc("/api/conversations/", cors(s.handleConversation))

	// Serve static files (images)
	s.mux.HandleFunc("/api/static/", s.handleStatic)

	// WebSocket for real-time updates
	s.mux.HandleFunc("/api/ws", s.HandleWS)

	// Serve embedded UI at root
	s.mux.Handle("/", UIHandler())
}

// Start begins the HTTP server
func (s *Server) Start(addr string) error {
	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 660 * time.Second, // 11 minutes - slightly longer than chat timeout
	}

	s.logger.Info("API server starting", "addr", addr)
	return s.server.ListenAndServe()
}

// Stop gracefully shuts down the server
func (s *Server) Stop(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

// Handler returns the HTTP handler for embedding
func (s *Server) Handler() http.Handler {
	return s.mux
}

// ============================================================================
// Handlers
// ============================================================================

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.json(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	// Serve static files from images directory
	filename := strings.TrimPrefix(r.URL.Path, "/api/static/")
	if filename == "" {
		http.NotFound(w, r)
		return
	}

	// Look for file in the images directory relative to executable or home
	paths := []string{
		filepath.Join("images", filename),
		filepath.Join(os.Getenv("HOME"), "Projects", "ugudu", "images", filename),
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			http.ServeFile(w, r, path)
			return
		}
	}

	http.NotFound(w, r)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := s.manager.Status()
	s.json(w, http.StatusOK, status)
}

func (s *Server) handleProviders(w http.ResponseWriter, r *http.Request) {
	providers := s.manager.Providers().List()

	result := make([]map[string]interface{}, 0, len(providers))
	for _, p := range providers {
		result = append(result, map[string]interface{}{
			"id":   p.ID(),
			"name": p.Name(),
		})
	}

	s.json(w, http.StatusOK, map[string]interface{}{
		"providers": result,
	})
}

func (s *Server) handleProviderByID(w http.ResponseWriter, r *http.Request) {
	// Extract provider ID from path: /api/providers/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/providers/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		s.error(w, http.StatusBadRequest, "provider ID required")
		return
	}
	providerID := parts[0]

	p, err := s.manager.Providers().Get(providerID)
	if err != nil {
		s.error(w, http.StatusNotFound, "provider not found")
		return
	}

	// Check for sub-routes
	if len(parts) > 1 {
		switch parts[1] {
		case "models":
			ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
			defer cancel()

			models, err := p.ListModels(ctx)
			if err != nil {
				s.error(w, http.StatusInternalServerError, err.Error())
				return
			}
			s.json(w, http.StatusOK, map[string]interface{}{"models": models})
			return

		case "test":
			ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
			defer cancel()

			if err := p.Ping(ctx); err != nil {
				s.json(w, http.StatusOK, map[string]interface{}{
					"status": "error",
					"error":  err.Error(),
				})
				return
			}
			s.json(w, http.StatusOK, map[string]interface{}{"status": "ok"})
			return
		}
	}

	s.json(w, http.StatusOK, map[string]interface{}{
		"id":   p.ID(),
		"name": p.Name(),
	})
}

func (s *Server) handleSpecs(w http.ResponseWriter, r *http.Request) {
	specsDir := config.SpecsDir()

	switch r.Method {
	case "GET":
		entries, err := os.ReadDir(specsDir)
		if err != nil {
			s.json(w, http.StatusOK, map[string]interface{}{"specs": []interface{}{}})
			return
		}

		var specs []map[string]interface{}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
				continue
			}

			name := strings.TrimSuffix(e.Name(), ".yaml")
			specPath := filepath.Join(specsDir, e.Name())

			memberCount := 0
			if spec, err := team.LoadSpec(specPath); err == nil {
				memberCount = len(spec.Roles)
			}

			specs = append(specs, map[string]interface{}{
				"name":    name,
				"path":    specPath,
				"members": memberCount,
			})
		}
		s.json(w, http.StatusOK, map[string]interface{}{"specs": specs})

	case "POST":
		var req struct {
			Name        string                            `json:"name"`
			Description string                            `json:"description"`
			Provider    string                            `json:"provider"`
			Model       string                            `json:"model"`
			Roles       map[string]map[string]interface{} `json:"roles"`
			ClientFacing []string                         `json:"client_facing"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.error(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.Name == "" {
			s.error(w, http.StatusBadRequest, "name required")
			return
		}

		// Build YAML content
		var yaml strings.Builder
		yaml.WriteString("apiVersion: ugudu/v1\n")
		yaml.WriteString("kind: Team\n")
		yaml.WriteString("metadata:\n")
		yaml.WriteString(fmt.Sprintf("  name: %s\n", req.Name))
		if req.Description != "" {
			yaml.WriteString(fmt.Sprintf("  description: %s\n", req.Description))
		}
		yaml.WriteString("\n")

		if len(req.ClientFacing) > 0 {
			yaml.WriteString("client_facing:\n")
			for _, cf := range req.ClientFacing {
				yaml.WriteString(fmt.Sprintf("  - %s\n", cf))
			}
			yaml.WriteString("\n")
		}

		yaml.WriteString("roles:\n")
		for roleID, role := range req.Roles {
			yaml.WriteString(fmt.Sprintf("  %s:\n", roleID))
			if title, ok := role["title"].(string); ok && title != "" {
				yaml.WriteString(fmt.Sprintf("    title: %s\n", title))
			}
			if name, ok := role["name"].(string); ok && name != "" {
				yaml.WriteString(fmt.Sprintf("    name: %s\n", name))
			}
			// Handle can_delegate array
			if canDelegate, ok := role["can_delegate"].([]interface{}); ok && len(canDelegate) > 0 {
				yaml.WriteString("    can_delegate:\n")
				for _, target := range canDelegate {
					if t, ok := target.(string); ok {
						yaml.WriteString(fmt.Sprintf("      - %s\n", t))
					}
				}
			}
			if count, ok := role["count"].(float64); ok && count > 1 {
				yaml.WriteString(fmt.Sprintf("    count: %.0f\n", count))
			}
			// Use per-role provider/model if specified, otherwise use defaults
			roleProvider := req.Provider
			roleModel := req.Model
			if p, ok := role["provider"].(string); ok && p != "" {
				roleProvider = p
			}
			if m, ok := role["model"].(string); ok && m != "" {
				roleModel = m
			}
			yaml.WriteString("    model:\n")
			yaml.WriteString(fmt.Sprintf("      provider: %s\n", roleProvider))
			yaml.WriteString(fmt.Sprintf("      model: %s\n", roleModel))
			if persona, ok := role["persona"].(string); ok && persona != "" {
				yaml.WriteString("    persona: |\n")
				for _, line := range strings.Split(persona, "\n") {
					yaml.WriteString(fmt.Sprintf("      %s\n", line))
				}
			}
			yaml.WriteString("\n")
		}

		// Write to file
		specPath := filepath.Join(specsDir, req.Name+".yaml")
		if err := os.WriteFile(specPath, []byte(yaml.String()), 0644); err != nil {
			s.error(w, http.StatusInternalServerError, "failed to save spec: "+err.Error())
			return
		}

		s.json(w, http.StatusOK, map[string]interface{}{"status": "ok", "path": specPath})

	default:
		s.error(w, http.StatusMethodNotAllowed, "GET or POST required")
	}
}

func (s *Server) handleSpecByName(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/specs/")
	if name == "" {
		s.error(w, http.StatusBadRequest, "spec name required")
		return
	}

	specPath := filepath.Join(config.SpecsDir(), name+".yaml")

	switch r.Method {
	case "GET":
		spec, err := team.LoadSpec(specPath)
		if err != nil {
			s.error(w, http.StatusNotFound, "spec not found")
			return
		}

		// Convert to JSON-friendly format
		roles := make(map[string]interface{})
		defaultProvider := ""
		defaultModel := ""
		for id, role := range spec.Roles {
			// Track first role's model config as default
			if defaultProvider == "" && role.Model.Provider != "" {
				defaultProvider = role.Model.Provider
				defaultModel = role.Model.Model
			}
			roles[id] = map[string]interface{}{
				"title":    role.Title,
				"name":     role.Name,
				"names":    role.Names,
				"count":    role.Count,
				"persona":  role.Persona,
				"provider": role.Model.Provider,
				"model":    role.Model.Model,
			}
		}

		s.json(w, http.StatusOK, map[string]interface{}{
			"name":          spec.Metadata.Name,
			"description":   spec.Metadata.Description,
			"roles":         roles,
			"client_facing": spec.ClientFacing,
			"provider":      defaultProvider,
			"model":         defaultModel,
		})

	case "DELETE":
		if err := os.Remove(specPath); err != nil {
			if os.IsNotExist(err) {
				s.error(w, http.StatusNotFound, "spec not found")
			} else {
				s.error(w, http.StatusInternalServerError, "failed to delete: "+err.Error())
			}
			return
		}
		s.json(w, http.StatusOK, map[string]interface{}{"status": "deleted"})

	default:
		s.error(w, http.StatusMethodNotAllowed, "GET or DELETE required")
	}
}

func (s *Server) handleTeams(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		teams := s.manager.ListTeams()
		result := make([]map[string]interface{}, 0, len(teams))
		for _, t := range teams {
			result = append(result, t.Status())
		}
		s.json(w, http.StatusOK, map[string]interface{}{"teams": result})

	case "POST":
		var req struct {
			Name     string `json:"name"`      // Team instance name
			Spec     string `json:"spec"`      // Spec name (will look in ~/.ugudu/specs/)
			SpecPath string `json:"spec_path"` // Direct path (legacy)
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.error(w, http.StatusBadRequest, "invalid request body")
			return
		}

		// Resolve spec path
		specPath := req.SpecPath
		if specPath == "" && req.Spec != "" {
			specPath = filepath.Join(config.SpecsDir(), req.Spec+".yaml")
		}

		if specPath == "" {
			s.error(w, http.StatusBadRequest, "spec or spec_path required")
			return
		}

		t, err := s.manager.CreateTeamWithName(req.Name, specPath)
		if err != nil {
			s.error(w, http.StatusInternalServerError, err.Error())
			return
		}

		s.json(w, http.StatusCreated, map[string]interface{}{
			"name":    t.Name,
			"members": len(t.Members),
		})

	default:
		s.error(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleTeamByName(w http.ResponseWriter, r *http.Request) {
	// Extract team name from path: /api/teams/{name}
	path := strings.TrimPrefix(r.URL.Path, "/api/teams/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		s.error(w, http.StatusBadRequest, "team name required")
		return
	}
	teamName := parts[0]

	// Check for sub-routes
	if len(parts) > 1 {
		switch parts[1] {
		case "start":
			if r.Method != "POST" {
				s.error(w, http.StatusMethodNotAllowed, "POST required")
				return
			}
			if err := s.manager.StartTeam(teamName); err != nil {
				s.error(w, http.StatusInternalServerError, err.Error())
				return
			}
			s.json(w, http.StatusOK, map[string]interface{}{"status": "started"})
			return

		case "stop":
			if r.Method != "POST" {
				s.error(w, http.StatusMethodNotAllowed, "POST required")
				return
			}
			if err := s.manager.StopTeam(teamName); err != nil {
				s.error(w, http.StatusInternalServerError, err.Error())
				return
			}
			s.json(w, http.StatusOK, map[string]interface{}{"status": "stopped"})
			return

		case "members":
			t, err := s.manager.GetTeam(teamName)
			if err != nil {
				s.error(w, http.StatusNotFound, "team not found")
				return
			}
			members := make([]map[string]interface{}, 0)
			for _, m := range t.ListMembers() {
				// Check if member is client-facing based on spec's client_facing list
				isClientFacing := false
				for _, cf := range t.Spec.ClientFacing {
					if cf == m.RoleName {
						isClientFacing = true
						break
					}
				}
				members = append(members, map[string]interface{}{
					"id":            m.ID,
					"name":          m.Name,
					"role":          m.RoleName,
					"title":         m.Role.Title,
					"status":        m.GetStatus(),
					"visibility":    m.Role.Visibility,
					"client_facing": isClientFacing,
					"provider":      m.Role.Model.Provider,
					"model":         m.Role.Model.Model,
				})
			}
			s.json(w, http.StatusOK, map[string]interface{}{"members": members})
			return

		case "conversations":
			s.handleTeamConversations(w, r, teamName)
			return

		case "token-mode":
			s.handleTeamTokenMode(w, r, teamName)
			return
		}
	}

	// Get single team
	switch r.Method {
	case "GET":
		t, err := s.manager.GetTeam(teamName)
		if err != nil {
			s.error(w, http.StatusNotFound, "team not found")
			return
		}
		s.json(w, http.StatusOK, t.Status())

	case "DELETE":
		if err := s.manager.DeleteTeam(teamName); err != nil {
			s.error(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.json(w, http.StatusOK, map[string]interface{}{"status": "deleted"})

	default:
		s.error(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.error(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req struct {
		Team    string `json:"team"`
		Message string `json:"message"`
		To      string `json:"to,omitempty"` // Optional: specific role
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Team == "" || req.Message == "" {
		s.error(w, http.StatusBadRequest, "team and message required")
		return
	}

	// Start team if not running
	_ = s.manager.StartTeam(req.Team)

	// Get team to access members for status updates
	t, _ := s.manager.GetTeam(req.Team)

	// Determine which member will handle the request
	targetRole := req.To
	if targetRole == "" {
		// Default to first client-facing member
		if t != nil && len(t.Spec.ClientFacing) > 0 {
			targetRole = t.Spec.ClientFacing[0]
		}
	}

	// Broadcast that the target member is now busy
	if targetRole != "" {
		s.wsHub.BroadcastMemberStatus(req.Team, targetRole, "busy", "Processing request...")
	}

	var respChan <-chan team.Message
	var err error

	if req.To != "" {
		respChan, err = s.manager.AskMember(req.Team, req.To, req.Message)
	} else {
		respChan, err = s.manager.Ask(req.Team, req.Message)
	}

	if err != nil {
		// Reset status on error
		if targetRole != "" {
			s.wsHub.BroadcastMemberStatus(req.Team, targetRole, "idle", "")
		}
		s.error(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Collect responses (with timeout) - 10 minutes for complex multi-agent tasks with tools
	ctx, cancel := context.WithTimeout(r.Context(), 600*time.Second)
	defer cancel()

	var responses []map[string]interface{}
	activeMember := targetRole // Track currently active member
	for {
		select {
		case <-ctx.Done():
			// Reset all busy members to idle on timeout
			if activeMember != "" {
				s.wsHub.BroadcastMemberStatus(req.Team, activeMember, "idle", "")
			}
			s.json(w, http.StatusOK, map[string]interface{}{
				"responses": responses,
				"timeout":   true,
			})
			return

		case msg, ok := <-respChan:
			if !ok {
				// Channel closed - all done, reset to idle
				if activeMember != "" {
					s.wsHub.BroadcastMemberStatus(req.Team, activeMember, "idle", "")
				}
				s.json(w, http.StatusOK, map[string]interface{}{
					"responses": responses,
				})
				return
			}

			content, _ := msg.Content.(string)
			responses = append(responses, map[string]interface{}{
				"from":    msg.From,
				"content": content,
				"type":    msg.Type,
			})

			// Broadcast activity update
			s.wsHub.BroadcastActivity(req.Team, msg.From, content, nil)

			// Check for delegation patterns in the message
			if msg.Type == "delegation" || strings.Contains(content, "delegating to") || strings.Contains(content, "asking") {
				// Previous member is now idle, new member is busy
				if activeMember != "" && activeMember != msg.From {
					s.wsHub.BroadcastMemberStatus(req.Team, activeMember, "idle", "")
				}
				s.wsHub.BroadcastMemberStatus(req.Team, msg.From, "busy", "Working on delegated task...")
				activeMember = msg.From
			}
		}
	}
}

// ============================================================================
// Helpers
// ============================================================================

func (s *Server) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) error(w http.ResponseWriter, status int, message string) {
	s.json(w, status, map[string]interface{}{
		"error": message,
	})
}

// APIResponse is a standard response wrapper
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// FormatError creates an error response
func FormatError(message string) APIResponse {
	return APIResponse{Success: false, Error: message}
}

// FormatSuccess creates a success response
func FormatSuccess(data interface{}) APIResponse {
	return APIResponse{Success: true, Data: data}
}

// ============================================================================
// Project Handlers
// ============================================================================

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		projects, err := workspace.ListProjects()
		if err != nil {
			s.error(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.json(w, http.StatusOK, map[string]interface{}{"projects": projects})

	case "POST":
		var req struct {
			Name       string   `json:"name"`
			SourcePath string   `json:"source_path"`
			Team       string   `json:"team"`
			SharedPaths []string `json:"shared_paths,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.error(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.Name == "" || req.SourcePath == "" || req.Team == "" {
			s.error(w, http.StatusBadRequest, "name, source_path, and team are required")
			return
		}

		ws, err := workspace.Init(req.Name, req.SourcePath, req.Team)
		if err != nil {
			s.error(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Add shared paths if specified
		if len(req.SharedPaths) > 0 {
			ws.Config.Source.SharedPaths = req.SharedPaths
			configPath := ws.Path + "/project.yaml"
			ws.Config.Save(configPath)
		}

		s.json(w, http.StatusCreated, map[string]interface{}{
			"name": ws.Name,
			"path": ws.Path,
			"team": req.Team,
		})

	default:
		s.error(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleProjectByName(w http.ResponseWriter, r *http.Request) {
	// Extract project name from path: /api/projects/{name}
	path := strings.TrimPrefix(r.URL.Path, "/api/projects/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		s.error(w, http.StatusBadRequest, "project name required")
		return
	}
	projectName := parts[0]

	// Check for sub-routes
	if len(parts) > 1 {
		switch parts[1] {
		case "standup":
			s.handleProjectStandup(w, r, projectName)
			return
		case "activity":
			s.handleProjectActivity(w, r, projectName)
			return
		case "tasks":
			s.handleProjectTasks(w, r, projectName)
			return
		}
	}

	// Get single project
	switch r.Method {
	case "GET":
		ws, err := workspace.New(projectName)
		if err != nil {
			s.error(w, http.StatusNotFound, "project not found")
			return
		}
		s.json(w, http.StatusOK, ws.Config)

	case "DELETE":
		if err := workspace.Delete(projectName); err != nil {
			s.error(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.json(w, http.StatusOK, map[string]interface{}{"status": "deleted"})

	default:
		s.error(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleProjectStandup(w http.ResponseWriter, r *http.Request, projectName string) {
	if r.Method != "GET" {
		s.error(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	ws, err := workspace.New(projectName)
	if err != nil {
		s.error(w, http.StatusNotFound, "project not found")
		return
	}

	// Get period from query param
	period := workspace.PeriodDaily
	if p := r.URL.Query().Get("period"); p == "weekly" {
		period = workspace.PeriodWeekly
	}

	generator := workspace.NewStandupGenerator(ws)
	report, err := generator.Generate(period)
	if err != nil {
		s.error(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.json(w, http.StatusOK, report)
}

func (s *Server) handleProjectActivity(w http.ResponseWriter, r *http.Request, projectName string) {
	if r.Method != "GET" {
		s.error(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	ws, err := workspace.New(projectName)
	if err != nil {
		s.error(w, http.StatusNotFound, "project not found")
		return
	}

	// Build query options from query params
	opts := workspace.QueryOptions{
		Limit: 50,
	}

	if limit := r.URL.Query().Get("limit"); limit != "" {
		var l int
		if _, err := json.Number(limit).Int64(); err == nil {
			opts.Limit = l
		}
	}

	if since := r.URL.Query().Get("since"); since != "" {
		if duration, err := time.ParseDuration(since); err == nil {
			opts.Since = time.Now().Add(-duration)
		}
	}

	if actType := r.URL.Query().Get("type"); actType != "" {
		opts.Types = []workspace.ActivityType{workspace.ActivityType(actType)}
	}

	entries, err := workspace.QueryProjectActivity(ws, opts)
	if err != nil {
		s.error(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.json(w, http.StatusOK, map[string]interface{}{
		"entries": entries,
		"count":   len(entries),
	})
}

func (s *Server) handleProjectTasks(w http.ResponseWriter, r *http.Request, projectName string) {
	ws, err := workspace.New(projectName)
	if err != nil {
		s.error(w, http.StatusNotFound, "project not found")
		return
	}

	taskStore := workspace.NewTaskStore(ws)

	switch r.Method {
	case "GET":
		tasks, err := taskStore.List()
		if err != nil {
			s.error(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Apply filters
		status := r.URL.Query().Get("status")
		assignee := r.URL.Query().Get("assignee")

		var filtered []workspace.Task
		for _, t := range tasks {
			if status != "" && t.Status != status {
				continue
			}
			if assignee != "" && t.AssignedTo != assignee {
				continue
			}
			filtered = append(filtered, t)
		}

		s.json(w, http.StatusOK, map[string]interface{}{
			"tasks": filtered,
			"count": len(filtered),
		})

	case "POST":
		var task workspace.Task
		if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
			s.error(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if task.Title == "" {
			s.error(w, http.StatusBadRequest, "title is required")
			return
		}

		if err := taskStore.Create(&task); err != nil {
			s.error(w, http.StatusInternalServerError, err.Error())
			return
		}

		s.json(w, http.StatusCreated, task)

	default:
		s.error(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// ============================================================================
// Conversation Handlers
// ============================================================================

func (s *Server) handleTeamConversations(w http.ResponseWriter, r *http.Request, teamName string) {
	store := s.manager.Store()
	if store == nil {
		s.error(w, http.StatusInternalServerError, "store not available")
		return
	}

	switch r.Method {
	case "GET":
		// List conversations for team
		limit := 10
		if l := r.URL.Query().Get("limit"); l != "" {
			var parsed int
			if n, err := json.Number(l).Int64(); err == nil {
				parsed = int(n)
				if parsed > 0 {
					limit = parsed
				}
			}
		}

		conversations, err := store.ListConversations(teamName, limit)
		if err != nil {
			s.error(w, http.StatusInternalServerError, err.Error())
			return
		}

		result := make([]map[string]interface{}, len(conversations))
		for i, conv := range conversations {
			result[i] = map[string]interface{}{
				"id":              conv.ID,
				"team_name":       conv.TeamName,
				"started_at":      conv.StartedAt.Format(time.RFC3339),
				"last_message_at": conv.LastMessageAt.Format(time.RFC3339),
				"status":          conv.Status,
			}
		}

		s.json(w, http.StatusOK, map[string]interface{}{
			"conversations": result,
		})

	case "DELETE":
		// Clear all conversations for team (start fresh)
		// Close active conversation and clear agent context
		conv, err := store.GetActiveConversation(teamName)
		if err != nil {
			s.error(w, http.StatusInternalServerError, err.Error())
			return
		}

		if conv != nil {
			if err := store.CloseConversation(conv.ID); err != nil {
				s.error(w, http.StatusInternalServerError, err.Error())
				return
			}
		}

		// Clear agent context for all members
		t, err := s.manager.GetTeam(teamName)
		if err == nil {
			for _, m := range t.ListMembers() {
				store.ClearAgentContext(teamName, m.ID)
				m.ClearContext()
			}
		}

		s.json(w, http.StatusOK, map[string]interface{}{
			"status":  "cleared",
			"message": "Conversation history cleared. A new conversation will start on the next message.",
		})

	default:
		s.error(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleTeamTokenMode(w http.ResponseWriter, r *http.Request, teamName string) {
	if r.Method != "POST" {
		s.error(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req struct {
		Mode string `json:"mode"` // "normal", "low", "minimal"
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	t, err := s.manager.GetTeam(teamName)
	if err != nil {
		s.error(w, http.StatusNotFound, "team not found")
		return
	}

	// Validate and set token mode
	switch req.Mode {
	case "normal", "":
		t.SetTokenMode(team.TokenModeNormal)
	case "low":
		t.SetTokenMode(team.TokenModeLow)
	case "minimal":
		t.SetTokenMode(team.TokenModeMinimal)
	default:
		s.error(w, http.StatusBadRequest, "invalid token mode (use: normal, low, minimal)")
		return
	}

	s.json(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"mode":   req.Mode,
	})
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		cfg, err := config.Load()
		if err != nil {
			s.error(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Mask API keys for display (show last 4 chars only)
		maskKey := func(key string) string {
			if len(key) <= 4 {
				return strings.Repeat("*", len(key))
			}
			return strings.Repeat("*", len(key)-4) + key[len(key)-4:]
		}

		s.json(w, http.StatusOK, map[string]interface{}{
			"providers": map[string]interface{}{
				"anthropic": map[string]interface{}{
					"configured": cfg.Providers.Anthropic.APIKey != "",
					"api_key":    maskKey(cfg.Providers.Anthropic.APIKey),
				},
				"openai": map[string]interface{}{
					"configured": cfg.Providers.OpenAI.APIKey != "",
					"api_key":    maskKey(cfg.Providers.OpenAI.APIKey),
					"base_url":   cfg.Providers.OpenAI.BaseURL,
				},
				"openrouter": map[string]interface{}{
					"configured": cfg.Providers.OpenRouter.APIKey != "",
					"api_key":    maskKey(cfg.Providers.OpenRouter.APIKey),
				},
				"groq": map[string]interface{}{
					"configured": cfg.Providers.Groq.APIKey != "",
					"api_key":    maskKey(cfg.Providers.Groq.APIKey),
				},
				"ollama": map[string]interface{}{
					"configured": cfg.Providers.Ollama.URL != "",
					"url":        cfg.Providers.Ollama.URL,
				},
			},
			"defaults": map[string]interface{}{
				"provider": cfg.Defaults.Provider,
				"model":    cfg.Defaults.Model,
			},
		})

	case "POST", "PUT":
		var req struct {
			Providers struct {
				Anthropic  *struct{ APIKey string `json:"api_key"` }  `json:"anthropic,omitempty"`
				OpenAI     *struct{ APIKey, BaseURL string }          `json:"openai,omitempty"`
				OpenRouter *struct{ APIKey string `json:"api_key"` }  `json:"openrouter,omitempty"`
				Groq       *struct{ APIKey string `json:"api_key"` }  `json:"groq,omitempty"`
				Ollama     *struct{ URL string }                      `json:"ollama,omitempty"`
			} `json:"providers"`
			Defaults *struct {
				Provider string `json:"provider"`
				Model    string `json:"model"`
			} `json:"defaults,omitempty"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.error(w, http.StatusBadRequest, "invalid request body")
			return
		}

		// Load existing config
		cfg, err := config.Load()
		if err != nil {
			cfg = &config.Config{}
		}

		// Update only provided fields
		if req.Providers.Anthropic != nil && req.Providers.Anthropic.APIKey != "" {
			cfg.Providers.Anthropic.APIKey = req.Providers.Anthropic.APIKey
		}
		if req.Providers.OpenAI != nil {
			if req.Providers.OpenAI.APIKey != "" {
				cfg.Providers.OpenAI.APIKey = req.Providers.OpenAI.APIKey
			}
			if req.Providers.OpenAI.BaseURL != "" {
				cfg.Providers.OpenAI.BaseURL = req.Providers.OpenAI.BaseURL
			}
		}
		if req.Providers.OpenRouter != nil && req.Providers.OpenRouter.APIKey != "" {
			cfg.Providers.OpenRouter.APIKey = req.Providers.OpenRouter.APIKey
		}
		if req.Providers.Groq != nil && req.Providers.Groq.APIKey != "" {
			cfg.Providers.Groq.APIKey = req.Providers.Groq.APIKey
		}
		if req.Providers.Ollama != nil && req.Providers.Ollama.URL != "" {
			cfg.Providers.Ollama.URL = req.Providers.Ollama.URL
		}
		if req.Defaults != nil {
			if req.Defaults.Provider != "" {
				cfg.Defaults.Provider = req.Defaults.Provider
			}
			if req.Defaults.Model != "" {
				cfg.Defaults.Model = req.Defaults.Model
			}
		}

		// Save config
		if err := config.Save(cfg); err != nil {
			s.error(w, http.StatusInternalServerError, "failed to save config: "+err.Error())
			return
		}

		// Apply to environment so providers can be discovered
		cfg.ApplyToEnvironment()

		// Re-initialize providers
		s.manager.Providers().AutoDiscover()

		s.json(w, http.StatusOK, map[string]interface{}{
			"status":  "ok",
			"message": "Settings saved. Providers re-initialized.",
		})

	default:
		s.error(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleConversation(w http.ResponseWriter, r *http.Request) {
	// Extract conversation ID from path: /api/conversations/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/conversations/")
	if path == "" {
		s.error(w, http.StatusBadRequest, "conversation ID required")
		return
	}

	store := s.manager.Store()
	if store == nil {
		s.error(w, http.StatusInternalServerError, "store not available")
		return
	}

	switch r.Method {
	case "GET":
		// Get conversation history
		messages, err := store.GetConversationHistory(path)
		if err != nil {
			s.error(w, http.StatusInternalServerError, err.Error())
			return
		}

		s.json(w, http.StatusOK, map[string]interface{}{
			"messages": messages,
		})

	default:
		s.error(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
