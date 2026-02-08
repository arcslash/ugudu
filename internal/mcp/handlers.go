package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/arcslash/ugudu/internal/config"
	"github.com/arcslash/ugudu/internal/daemon"
)

func (s *Server) handleListTeams(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if err := s.ensureClient(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	teams, err := s.client.ListTeams(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list teams: %w", err)
	}

	if len(teams) == 0 {
		return "No teams found. Create one with ugudu_create_team.", nil
	}

	var sb strings.Builder
	sb.WriteString("Teams:\n")
	for _, t := range teams {
		name, _ := t["name"].(string)
		status, _ := t["status"].(string)
		memberCount := int(t["member_count"].(float64))
		desc, _ := t["description"].(string)
		sb.WriteString(fmt.Sprintf("  - %s (%s, %d members): %s\n", name, status, memberCount, desc))
	}

	return sb.String(), nil
}

func (s *Server) handleCreateTeam(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if err := s.ensureClient(); err != nil {
		return nil, err
	}

	name, _ := args["name"].(string)
	spec, _ := args["spec"].(string)

	if name == "" || spec == "" {
		return nil, fmt.Errorf("both 'name' and 'spec' are required")
	}

	// Resolve spec path
	specPath := filepath.Join(config.SpecsDir(), spec+".yaml")
	if _, err := os.Stat(specPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("spec '%s' not found. Use ugudu_list_specs to see available specs", spec)
	}

	// Read spec and modify name
	content, err := os.ReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read spec: %w", err)
	}

	// Replace name in spec
	modifiedSpec := replaceTeamNameInSpec(string(content), name)

	// Write to persistent spec file in ~/.ugudu/specs/
	specFile := filepath.Join(config.SpecsDir(), name+".yaml")
	if err := os.WriteFile(specFile, []byte(modifiedSpec), 0644); err != nil {
		return nil, fmt.Errorf("failed to write spec file: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result, err := s.client.CreateTeam(ctx, specFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create team: %w", err)
	}

	teamName := result["name"].(string)
	members := int(result["members"].(float64))

	return fmt.Sprintf("Team '%s' created successfully with %d members.\n\nYou can now send messages with: ugudu_ask team=%s message=\"Your message\"", teamName, members, teamName), nil
}

func (s *Server) handleAsk(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if err := s.ensureClient(); err != nil {
		return nil, err
	}

	team, _ := args["team"].(string)
	message, _ := args["message"].(string)

	if team == "" || message == "" {
		return nil, fmt.Errorf("both 'team' and 'message' are required")
	}

	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	// Start team if not running
	_ = s.client.StartTeam(ctx, team)

	responses, err := s.client.Chat(ctx, team, message, "")
	if err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	var sb strings.Builder
	for _, resp := range responses {
		from, _ := resp["from"].(string)
		content, _ := resp["content"].(string)
		sb.WriteString(fmt.Sprintf("%s: %s\n", from, content))
	}

	if sb.Len() == 0 {
		return "No response received from team.", nil
	}

	return sb.String(), nil
}

func (s *Server) handleTeamStatus(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if err := s.ensureClient(); err != nil {
		return nil, err
	}

	team, _ := args["team"].(string)
	if team == "" {
		return nil, fmt.Errorf("'team' is required")
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	members, err := s.client.TeamMembers(ctx, team)
	if err != nil {
		return nil, fmt.Errorf("failed to get team status: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Team: %s\n\nMembers:\n", team))

	for _, m := range members {
		name, _ := m["name"].(string)
		title, _ := m["title"].(string)
		status, _ := m["status"].(string)
		visibility, _ := m["visibility"].(string)

		displayName := name
		if displayName == "" || displayName == title {
			displayName = title
		} else {
			displayName = fmt.Sprintf("%s (%s)", name, title)
		}

		icon := "🔒"
		if visibility == "client" {
			icon = "👤"
		}

		sb.WriteString(fmt.Sprintf("  %s %s - %s\n", icon, displayName, status))
	}

	sb.WriteString("\n👤 = client-facing, 🔒 = internal")

	return sb.String(), nil
}

func (s *Server) handleListSpecs(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	specsDir := config.SpecsDir()

	entries, err := os.ReadDir(specsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "No specs found. Create one with: ugudu spec new <name>", nil
		}
		return nil, fmt.Errorf("failed to read specs: %w", err)
	}

	var specs []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
			specs = append(specs, strings.TrimSuffix(e.Name(), ".yaml"))
		}
	}

	if len(specs) == 0 {
		return "No specs found. Create one with: ugudu spec new <name>", nil
	}

	var sb strings.Builder
	sb.WriteString("Available specs:\n")
	for _, name := range specs {
		sb.WriteString(fmt.Sprintf("  - %s\n", name))
	}
	sb.WriteString("\nCreate a team with: ugudu_create_team name=\"my-team\" spec=\"<spec-name>\"")

	return sb.String(), nil
}

func (s *Server) handleDaemonStatus(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if s.client == nil {
		client, err := daemon.NewClient("")
		if err != nil {
			return "Daemon is NOT running.\n\nStart it with: ugudu daemon", nil
		}
		s.client = client
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := s.client.Ping(ctx); err != nil {
		return "Daemon is NOT responding.\n\nRestart it with: ugudu daemon", nil
	}

	status, err := s.client.Status(ctx)
	if err != nil {
		return "Daemon is running but failed to get status.", nil
	}

	var sb strings.Builder
	sb.WriteString("Daemon is running.\n\n")

	if teams, ok := status["teams"].([]interface{}); ok {
		sb.WriteString(fmt.Sprintf("Teams: %d\n", len(teams)))
	}

	if providers, ok := status["providers"].([]interface{}); ok {
		sb.WriteString(fmt.Sprintf("Providers: %d configured\n", len(providers)))
		for _, p := range providers {
			if pm, ok := p.(map[string]interface{}); ok {
				sb.WriteString(fmt.Sprintf("  - %s\n", pm["name"]))
			}
		}
	}

	return sb.String(), nil
}

func (s *Server) handleStartTeam(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if err := s.ensureClient(); err != nil {
		return nil, err
	}

	team, _ := args["team"].(string)
	if team == "" {
		return nil, fmt.Errorf("'team' is required")
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := s.client.StartTeam(ctx, team); err != nil {
		return nil, fmt.Errorf("failed to start team: %w", err)
	}

	return fmt.Sprintf("Team '%s' started.", team), nil
}

func (s *Server) handleStopTeam(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if err := s.ensureClient(); err != nil {
		return nil, err
	}

	team, _ := args["team"].(string)
	if team == "" {
		return nil, fmt.Errorf("'team' is required")
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := s.client.StopTeam(ctx, team); err != nil {
		return nil, fmt.Errorf("failed to stop team: %w", err)
	}

	return fmt.Sprintf("Team '%s' stopped.", team), nil
}

func (s *Server) handleDeleteTeam(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if err := s.ensureClient(); err != nil {
		return nil, err
	}

	team, _ := args["team"].(string)
	if team == "" {
		return nil, fmt.Errorf("'team' is required")
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := s.client.DeleteTeam(ctx, team); err != nil {
		return nil, fmt.Errorf("failed to delete team: %w", err)
	}

	return fmt.Sprintf("Team '%s' deleted.", team), nil
}

// Helper function to replace team name in YAML
func replaceTeamNameInSpec(specContent, teamName string) string {
	lines := strings.Split(specContent, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "name:") {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			lines[i] = indent + "name: " + teamName
			break
		}
	}
	return strings.Join(lines, "\n")
}

// ============================================================================
// Project Workflow Handlers
// ============================================================================

func (s *Server) handleStartProject(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if err := s.ensureClient(); err != nil {
		return nil, err
	}

	team, _ := args["team"].(string)
	request, _ := args["request"].(string)

	if team == "" || request == "" {
		return nil, fmt.Errorf("both 'team' and 'request' are required")
	}

	ctx, cancel := context.WithTimeout(ctx, 600*time.Second)
	defer cancel()

	// Start project via the daemon
	result, err := s.client.StartProject(ctx, team, request)
	if err != nil {
		return nil, fmt.Errorf("failed to start project: %w", err)
	}

	return fmt.Sprintf(`Project started successfully!

Project ID: %s
Phase: %s
Team: %s

The PM and BA are now analyzing your request and creating requirements.
Use ugudu_project_status to check progress.
If there are questions, use ugudu_answer_question to respond.`,
		result["id"], result["phase"], team), nil
}

func (s *Server) handleProjectStatus(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if err := s.ensureClient(); err != nil {
		return nil, err
	}

	team, _ := args["team"].(string)
	if team == "" {
		return nil, fmt.Errorf("'team' is required")
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	status, err := s.client.ProjectStatus(ctx, team)
	if err != nil {
		return nil, fmt.Errorf("failed to get project status: %w", err)
	}

	if status == nil {
		return "No active project for this team.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Project: %s\n", status["id"]))
	sb.WriteString(fmt.Sprintf("Phase: %s\n", status["phase"]))
	sb.WriteString(fmt.Sprintf("Requirements: %v\n", status["requirements"]))

	if stories, ok := status["stories"].([]interface{}); ok {
		sb.WriteString(fmt.Sprintf("\nStories: %d\n", len(stories)))
		for _, s := range stories {
			if story, ok := s.(map[string]interface{}); ok {
				sb.WriteString(fmt.Sprintf("  - %s [%s] -> %s\n",
					story["title"], story["status"], story["assigned"]))
			}
		}
	}

	if pending, ok := status["pending_questions"].(float64); ok && pending > 0 {
		sb.WriteString(fmt.Sprintf("\n⚠️  %d questions pending your response!\n", int(pending)))
		sb.WriteString("Use ugudu_pending_questions to see them.\n")
	}

	return sb.String(), nil
}

func (s *Server) handlePendingQuestions(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if err := s.ensureClient(); err != nil {
		return nil, err
	}

	team, _ := args["team"].(string)
	if team == "" {
		return nil, fmt.Errorf("'team' is required")
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	questions, err := s.client.PendingQuestions(ctx, team)
	if err != nil {
		return nil, fmt.Errorf("failed to get questions: %w", err)
	}

	if len(questions) == 0 {
		return "No pending questions.", nil
	}

	var sb strings.Builder
	sb.WriteString("Pending Questions:\n\n")
	for i, q := range questions {
		qMap := q.(map[string]interface{})
		sb.WriteString(fmt.Sprintf("%d. [ID: %s]\n", i+1, qMap["id"]))
		sb.WriteString(fmt.Sprintf("   From: %s\n", qMap["from_role"]))
		sb.WriteString(fmt.Sprintf("   Question: %s\n\n", qMap["content"]))
	}
	sb.WriteString("\nUse ugudu_answer_question with the question ID to respond.")

	return sb.String(), nil
}

func (s *Server) handleAnswerQuestion(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if err := s.ensureClient(); err != nil {
		return nil, err
	}

	team, _ := args["team"].(string)
	questionID, _ := args["question_id"].(string)
	answer, _ := args["answer"].(string)

	if team == "" || questionID == "" || answer == "" {
		return nil, fmt.Errorf("'team', 'question_id', and 'answer' are required")
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := s.client.AnswerQuestion(ctx, team, questionID, answer); err != nil {
		return nil, fmt.Errorf("failed to answer question: %w", err)
	}

	return "Answer recorded. The team will continue with your input.", nil
}

func (s *Server) handleProjectCommunications(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if err := s.ensureClient(); err != nil {
		return nil, err
	}

	team, _ := args["team"].(string)
	limit := 20
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	if team == "" {
		return nil, fmt.Errorf("'team' is required")
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	comms, err := s.client.ProjectCommunications(ctx, team, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get communications: %w", err)
	}

	if len(comms) == 0 {
		return "No communications yet.", nil
	}

	var sb strings.Builder
	sb.WriteString("Project Communications:\n\n")
	for _, c := range comms {
		cMap := c.(map[string]interface{})
		sb.WriteString(fmt.Sprintf("[%s] %s -> %s: %s\n",
			cMap["type"], cMap["from"], cMap["to"], truncate(cMap["content"].(string), 100)))
	}

	return sb.String(), nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ============================================================================
// Token Mode Handlers
// ============================================================================

func (s *Server) handleSetTokenMode(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if err := s.ensureClient(); err != nil {
		return nil, err
	}

	team, _ := args["team"].(string)
	mode, _ := args["mode"].(string)

	if team == "" || mode == "" {
		return nil, fmt.Errorf("team and mode are required")
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := s.client.SetTokenMode(ctx, team, mode); err != nil {
		return nil, fmt.Errorf("failed to set token mode: %w", err)
	}

	modeDescriptions := map[string]string{
		"normal":  "Full prompts, 4096 max tokens, 40 message context",
		"low":     "Condensed prompts, 1024 max tokens, 10 message context (~60% cost reduction)",
		"minimal": "Bare minimum prompts, 512 max tokens, 5 message context (~80% cost reduction)",
	}

	return fmt.Sprintf("Token mode set to '%s' for team '%s'.\n%s", mode, team, modeDescriptions[mode]), nil
}

func (s *Server) handleClearConversation(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if err := s.ensureClient(); err != nil {
		return nil, err
	}

	team, _ := args["team"].(string)
	if team == "" {
		return nil, fmt.Errorf("team name is required")
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := s.client.ClearConversation(ctx, team); err != nil {
		return nil, fmt.Errorf("failed to clear conversation: %w", err)
	}

	return fmt.Sprintf("Conversation history cleared for team '%s'. A new conversation will start on the next message.", team), nil
}

func (s *Server) handleListSpecialists(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	// List specialist templates from ~/.ugudu/specs/specialists/
	specsDir := filepath.Join(config.SpecsDir(), "specialists")
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		return "No specialists found. Create them in ~/.ugudu/specs/specialists/", nil
	}

	var specialists []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
			name := strings.TrimSuffix(e.Name(), ".yaml")
			specialists = append(specialists, name)
		}
	}

	if len(specialists) == 0 {
		return "No specialists found. Create them in ~/.ugudu/specs/specialists/", nil
	}

	var sb strings.Builder
	sb.WriteString("Available Specialist Templates:\n\n")
	for _, name := range specialists {
		// Add descriptions based on known specialists
		desc := getSpecialistDescription(name)
		sb.WriteString(fmt.Sprintf("  - %s: %s\n", name, desc))
	}
	sb.WriteString("\nTo add a specialist to a team, include them in the team spec's roles section.")

	return sb.String(), nil
}

func getSpecialistDescription(name string) string {
	descriptions := map[string]string{
		"healthcare-sme":    "FHIR, HIPAA, clinical workflows expert",
		"devops":            "CI/CD, Kubernetes, Terraform specialist",
		"security-engineer": "Security architecture, threat modeling",
		"data-engineer":     "ETL, data pipelines, analytics",
		"ux-designer":       "UX/UI design, accessibility",
		"fintech-sme":       "Banking, payments, PCI-DSS compliance",
	}
	if desc, ok := descriptions[name]; ok {
		return desc
	}
	return "Domain specialist"
}
