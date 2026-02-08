// Package api provides the HTTP REST API for Ugudu
package api

import (
	"context"
	"encoding/json"
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
}

// NewServer creates a new API server
func NewServer(mgr *manager.Manager, log *logger.Logger) *Server {
	s := &Server{
		manager: mgr,
		logger:  log,
		mux:     http.NewServeMux(),
	}
	s.setupRoutes()
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

	// Specs (team blueprints)
	s.mux.HandleFunc("/api/specs", cors(s.handleSpecs))

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
	if r.Method != "GET" {
		s.error(w, http.StatusMethodNotAllowed, "GET required")
		return
	}

	specsDir := config.SpecsDir()
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

		// Try to load spec to count members
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
				members = append(members, map[string]interface{}{
					"id":            m.ID,
					"role":          m.RoleName,
					"title":         m.Role.Title,
					"status":        m.GetStatus(),
					"visibility":    m.Role.Visibility,
					"client_facing": m.Role.Visibility == "external",
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

	var respChan <-chan team.Message
	var err error

	if req.To != "" {
		respChan, err = s.manager.AskMember(req.Team, req.To, req.Message)
	} else {
		respChan, err = s.manager.Ask(req.Team, req.Message)
	}

	if err != nil {
		s.error(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Collect responses (with timeout) - 10 minutes for complex multi-agent tasks with tools
	ctx, cancel := context.WithTimeout(r.Context(), 600*time.Second)
	defer cancel()

	var responses []map[string]interface{}
	for {
		select {
		case <-ctx.Done():
			s.json(w, http.StatusOK, map[string]interface{}{
				"responses": responses,
				"timeout":   true,
			})
			return

		case msg, ok := <-respChan:
			if !ok {
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
