// Package team provides the orchestrator for multi-agent collaboration
package team

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/arcslash/ugudu/internal/logger"
	"github.com/arcslash/ugudu/internal/provider"
	"github.com/google/uuid"
)

// Orchestrator coordinates multi-agent collaboration on projects
type Orchestrator struct {
	team           *Team
	projectManager *ProjectManager
	logger         *logger.Logger

	// Active project being worked on
	activeProject *Project

	// Channels for coordination
	clientQuestions chan Question
	clientAnswers   chan QuestionAnswer

	mu sync.RWMutex
}

// QuestionAnswer pairs a question with its answer
type QuestionAnswer struct {
	QuestionID string
	Answer     string
}

// NewOrchestrator creates a new orchestrator for a team
func NewOrchestrator(team *Team, log *logger.Logger) *Orchestrator {
	return &Orchestrator{
		team:            team,
		projectManager:  NewProjectManager(team),
		logger:          log,
		clientQuestions: make(chan Question, 10),
		clientAnswers:   make(chan QuestionAnswer, 10),
	}
}

// StartProject initiates a new project from a client request
func (o *Orchestrator) StartProject(ctx context.Context, clientRequest string) (*Project, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Create the project
	project := o.projectManager.CreateProject(
		"project-"+uuid.New().String()[:8],
		clientRequest,
		clientRequest,
	)
	o.activeProject = project

	o.logger.Info("project created", "id", project.ID, "phase", project.Phase)

	// Start the planning phase
	go o.runPlanningPhase(ctx, project)

	return project, nil
}

// runPlanningPhase coordinates PM and BA to create requirements
func (o *Orchestrator) runPlanningPhase(ctx context.Context, project *Project) {
	project.SetPhase(PhasePlanning)
	o.logger.Info("starting planning phase", "project", project.ID)

	// Get PM and BA
	pm := o.team.GetMemberByRole("pm")
	ba := o.team.GetMemberByRole("ba")

	if pm == nil {
		o.logger.Error("no PM found in team")
		return
	}

	// Step 1: PM analyzes the request and identifies what's needed
	pmAnalysis := o.getPMAnalysis(ctx, pm, project)
	project.AddCommunication("message", pm.ID, "all", pmAnalysis, nil)

	// Step 2: If BA exists, have them create detailed requirements
	var requirements []Requirement
	if ba != nil {
		requirements = o.getBARequirements(ctx, ba, project, pmAnalysis)
	} else {
		// PM creates requirements if no BA
		requirements = o.getPMRequirements(ctx, pm, project, pmAnalysis)
	}

	// Add requirements to project
	for _, req := range requirements {
		project.AddRequirement(req.Title, req.Description, req.Priority, req.CreatedBy)
	}

	// Step 3: Check if we need more info from client
	if len(project.PendingQuestions) > 0 {
		project.SetPhase(PhaseBlocked)
		o.logger.Info("project blocked - waiting for client answers", "questions", len(project.PendingQuestions))
		return
	}

	// Move to task breakdown
	o.runTaskBreakdownPhase(ctx, project)
}

// runTaskBreakdownPhase creates stories from requirements
func (o *Orchestrator) runTaskBreakdownPhase(ctx context.Context, project *Project) {
	project.SetPhase(PhaseTaskBreakdown)
	o.logger.Info("starting task breakdown phase", "project", project.ID)

	pm := o.team.GetMemberByRole("pm")
	if pm == nil {
		return
	}

	// Have PM/BA break requirements into stories
	stories := o.createStoriesFromRequirements(ctx, pm, project)

	for _, story := range stories {
		project.CreateStory(
			story.RequirementID,
			story.Title,
			story.Description,
			story.Type,
			story.AssignedRole,
			story.AcceptanceCriteria,
		)
	}

	o.logger.Info("stories created", "count", len(stories))

	// Move to execution
	o.runExecutionPhase(ctx, project)
}

// runExecutionPhase assigns stories to engineers and coordinates work
func (o *Orchestrator) runExecutionPhase(ctx context.Context, project *Project) {
	project.SetPhase(PhaseExecution)
	o.logger.Info("starting execution phase", "project", project.ID)

	// Get available engineers
	backendEngineers := o.team.MembersByRole["backend"]
	frontendEngineers := o.team.MembersByRole["frontend"]
	engineers := o.team.MembersByRole["engineer"]

	// Combine all engineers
	allEngineers := make([]*Member, 0)
	allEngineers = append(allEngineers, backendEngineers...)
	allEngineers = append(allEngineers, frontendEngineers...)
	allEngineers = append(allEngineers, engineers...)

	if len(allEngineers) == 0 {
		o.logger.Warn("no engineers available")
		return
	}

	// Distribute stories to engineers
	var wg sync.WaitGroup
	for i, story := range project.Stories {
		// Assign to appropriate engineer based on role
		var engineer *Member
		switch story.AssignedRole {
		case "backend":
			if len(backendEngineers) > 0 {
				engineer = backendEngineers[i%len(backendEngineers)]
			}
		case "frontend":
			if len(frontendEngineers) > 0 {
				engineer = frontendEngineers[i%len(frontendEngineers)]
			}
		default:
			if len(allEngineers) > 0 {
				engineer = allEngineers[i%len(allEngineers)]
			}
		}

		if engineer != nil {
			story.AssignedMember = engineer.ID
			story.UpdateStatus(StoryReady)

			wg.Add(1)
			go func(s *Story, e *Member) {
				defer wg.Done()
				o.executeStory(ctx, project, s, e)
			}(story, engineer)
		}
	}

	// Wait for all stories to complete
	wg.Wait()

	// Move to review
	o.runReviewPhase(ctx, project)
}

// executeStory has an engineer work on a story
func (o *Orchestrator) executeStory(ctx context.Context, project *Project, story *Story, engineer *Member) {
	story.UpdateStatus(StoryInProgress)
	o.logger.Info("engineer starting story", "engineer", engineer.ID, "story", story.ID)

	// Build the prompt for the engineer
	prompt := o.buildStoryPrompt(project, story)

	// Add to engineer's context
	messages := []provider.Message{
		{Role: "system", Content: engineer.buildSystemPrompt()},
		{Role: "user", Content: prompt},
	}

	// Get tools
	providerTools := engineer.getProviderTools()

	// Execute with tool loop
	var finalContent string
	for iteration := 0; iteration < MaxToolIterations; iteration++ {
		resp, err := engineer.Provider.Chat(ctx, &provider.ChatRequest{
			Model:       engineer.Role.Model.Model,
			Messages:    messages,
			Tools:       providerTools,
			Temperature: engineer.Role.Model.Temperature,
			MaxTokens:   engineer.Role.Model.MaxTokens,
		})

		if err != nil {
			o.logger.Error("engineer chat failed", "error", err)
			story.UpdateStatus(StoryBlocked)
			return
		}

		// Handle tool calls
		if len(resp.ToolCalls) > 0 && engineer.toolRegistry != nil {
			messages = append(messages, provider.Message{
				Role:      "assistant",
				Content:   resp.Content,
				ToolCalls: resp.ToolCalls,
			})

			toolResults := engineer.executeToolCalls(ctx, resp.ToolCalls)
			messages = append(messages, toolResults...)

			// Log artifacts from tool calls
			for _, tc := range resp.ToolCalls {
				if tc.Name == "write_file" || tc.Name == "edit_file" {
					var args map[string]interface{}
					json.Unmarshal([]byte(tc.Arguments), &args)
					if path, ok := args["path"].(string); ok {
						story.AddArtifact("file", path, engineer.ID)
					}
				}
			}
			continue
		}

		finalContent = resp.Content
		break
	}

	// Check if engineer has questions
	if strings.Contains(strings.ToLower(finalContent), "question") ||
		strings.Contains(strings.ToLower(finalContent), "clarification") ||
		strings.Contains(strings.ToLower(finalContent), "unclear") {
		// Parse and route questions
		o.handleEngineerQuestions(project, story, engineer, finalContent)
	}

	// Log the work summary
	project.AddCommunication("message", engineer.ID, "pm",
		fmt.Sprintf("Completed story: %s\n\n%s", story.Title, finalContent),
		map[string]interface{}{"story_id": story.ID})

	story.UpdateStatus(StoryReview)
	o.logger.Info("engineer completed story", "engineer", engineer.ID, "story", story.ID)
}

// runReviewPhase has QA review the completed work
func (o *Orchestrator) runReviewPhase(ctx context.Context, project *Project) {
	project.SetPhase(PhaseReview)
	o.logger.Info("starting review phase", "project", project.ID)

	qa := o.team.GetMemberByRole("qa")
	if qa == nil {
		// No QA, skip to complete
		project.SetPhase(PhaseComplete)
		return
	}

	// Have QA review each story
	for _, story := range project.Stories {
		if story.Status == StoryReview {
			o.reviewStory(ctx, project, story, qa)
		}
	}

	project.SetPhase(PhaseComplete)
	o.logger.Info("project completed", "project", project.ID)
}

// reviewStory has QA review a story
func (o *Orchestrator) reviewStory(ctx context.Context, project *Project, story *Story, qa *Member) {
	prompt := fmt.Sprintf(`Review the following completed story:

Title: %s
Description: %s

Acceptance Criteria:
%s

Artifacts created:
%s

Please verify the work meets the acceptance criteria.`,
		story.Title,
		story.Description,
		strings.Join(story.AcceptanceCriteria, "\n- "),
		formatArtifacts(story.Artifacts),
	)

	resp, err := qa.Provider.Chat(ctx, &provider.ChatRequest{
		Model:    qa.Role.Model.Model,
		Messages: []provider.Message{
			{Role: "system", Content: qa.buildSystemPrompt()},
			{Role: "user", Content: prompt},
		},
	})

	if err != nil {
		o.logger.Error("QA review failed", "error", err)
		return
	}

	project.AddCommunication("message", qa.ID, "pm",
		fmt.Sprintf("Review of '%s': %s", story.Title, resp.Content),
		map[string]interface{}{"story_id": story.ID})

	story.UpdateStatus(StoryDone)
}

// getPMAnalysis gets PM's initial analysis of the client request
func (o *Orchestrator) getPMAnalysis(ctx context.Context, pm *Member, project *Project) string {
	prompt := fmt.Sprintf(`A client has submitted the following request:

"%s"

As the Project Manager, please:
1. Summarize what the client wants
2. Identify the main components/features needed
3. List any clarifying questions we need to ask the client
4. Suggest which team roles will be involved

Format your response as:
## Summary
[Your summary]

## Key Components
- [Component 1]
- [Component 2]

## Questions for Client (if any)
- [Question 1]

## Team Roles Needed
- [Role 1]: [What they'll do]`,
		project.Description,
	)

	resp, err := pm.Provider.Chat(ctx, &provider.ChatRequest{
		Model:    pm.Role.Model.Model,
		Messages: []provider.Message{
			{Role: "system", Content: pm.buildSystemPrompt()},
			{Role: "user", Content: prompt},
		},
	})

	if err != nil {
		o.logger.Error("PM analysis failed", "error", err)
		return ""
	}

	// Parse questions for client
	if strings.Contains(resp.Content, "Questions for Client") {
		o.parseAndAddQuestions(project, pm, resp.Content)
	}

	return resp.Content
}

// getBARequirements has BA create detailed requirements
func (o *Orchestrator) getBARequirements(ctx context.Context, ba *Member, project *Project, pmAnalysis string) []Requirement {
	prompt := fmt.Sprintf(`Based on the following project analysis from the PM:

%s

Original client request:
"%s"

As the Business Analyst, create detailed requirements. For each requirement:
1. Give it a clear title
2. Write a detailed description
3. Assign priority (must, should, could)

Format as JSON array:
[
  {
    "title": "Requirement title",
    "description": "Detailed description",
    "priority": "must|should|could"
  }
]`,
		pmAnalysis,
		project.Description,
	)

	resp, err := ba.Provider.Chat(ctx, &provider.ChatRequest{
		Model:    ba.Role.Model.Model,
		Messages: []provider.Message{
			{Role: "system", Content: ba.buildSystemPrompt()},
			{Role: "user", Content: prompt},
		},
	})

	if err != nil {
		o.logger.Error("BA requirements failed", "error", err)
		return nil
	}

	return parseRequirements(resp.Content, ba.ID)
}

// getPMRequirements has PM create requirements when no BA is available
func (o *Orchestrator) getPMRequirements(ctx context.Context, pm *Member, project *Project, pmAnalysis string) []Requirement {
	// Similar to BA but for PM
	return o.getBARequirements(ctx, pm, project, pmAnalysis)
}

// createStoriesFromRequirements breaks requirements into stories
func (o *Orchestrator) createStoriesFromRequirements(ctx context.Context, pm *Member, project *Project) []*Story {
	reqSummary := ""
	for _, req := range project.Requirements {
		reqSummary += fmt.Sprintf("- %s: %s (Priority: %s)\n", req.Title, req.Description, req.Priority)
	}

	prompt := fmt.Sprintf(`Break down these requirements into user stories for the engineering team:

%s

For each story, specify:
1. Title
2. Description
3. Type (feature, task, spike)
4. Assigned role (backend, frontend, or engineer for full-stack)
5. Acceptance criteria (list)
6. Estimated effort (small, medium, large)

Format as JSON array:
[
  {
    "title": "Story title",
    "description": "What needs to be done",
    "type": "feature",
    "assigned_role": "backend",
    "acceptance_criteria": ["Criterion 1", "Criterion 2"],
    "estimated_effort": "medium",
    "requirement_id": "requirement_id_if_known"
  }
]`,
		reqSummary,
	)

	resp, err := pm.Provider.Chat(ctx, &provider.ChatRequest{
		Model:    pm.Role.Model.Model,
		Messages: []provider.Message{
			{Role: "system", Content: pm.buildSystemPrompt()},
			{Role: "user", Content: prompt},
		},
	})

	if err != nil {
		o.logger.Error("story creation failed", "error", err)
		return nil
	}

	return parseStories(resp.Content)
}

// buildStoryPrompt creates the prompt for an engineer to work on a story
func (o *Orchestrator) buildStoryPrompt(project *Project, story *Story) string {
	return fmt.Sprintf(`You have been assigned the following story:

## Project Context
%s

## Your Story
**Title:** %s
**Type:** %s
**Description:** %s

## Acceptance Criteria
%s

## Technical Notes
%s

Please complete this story using your available tools. You can:
- Read existing files to understand the codebase
- Write new files or edit existing ones
- Run commands to test your work

When you're done, summarize what you've accomplished.

If you have questions or need clarification, ask them clearly.`,
		project.Description,
		story.Title,
		story.Type,
		story.Description,
		formatAcceptanceCriteria(story.AcceptanceCriteria),
		story.TechnicalNotes,
	)
}

// handleEngineerQuestions processes questions from engineers
func (o *Orchestrator) handleEngineerQuestions(project *Project, story *Story, engineer *Member, response string) {
	// For now, route to PM
	q := story.AskQuestion(response, engineer.RoleName, engineer.ID, "pm")
	project.AddCommunication("question", engineer.ID, "pm", response,
		map[string]interface{}{"question_id": q.ID, "story_id": story.ID})
}

// parseAndAddQuestions extracts questions from PM's response
func (o *Orchestrator) parseAndAddQuestions(project *Project, pm *Member, content string) {
	// Simple parsing - look for bullet points after "Questions for Client"
	lines := strings.Split(content, "\n")
	inQuestions := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "Questions for Client") {
			inQuestions = true
			continue
		}
		if inQuestions && strings.HasPrefix(line, "-") {
			question := strings.TrimPrefix(line, "- ")
			question = strings.TrimPrefix(question, "* ")
			if question != "" {
				project.AskQuestion(question, "pm", pm.ID, "client", project.ID)
			}
		}
		if inQuestions && strings.HasPrefix(line, "##") {
			break // End of questions section
		}
	}
}

// ProvideAnswer allows client to answer a pending question
func (o *Orchestrator) ProvideAnswer(questionID, answer string) error {
	if o.activeProject == nil {
		return fmt.Errorf("no active project")
	}

	err := o.activeProject.AnswerQuestion(questionID, answer, "client")
	if err != nil {
		return err
	}

	// Check if all questions are answered
	allAnswered := true
	for _, q := range o.activeProject.PendingQuestions {
		if q.Status == "pending" {
			allAnswered = false
			break
		}
	}

	// If all answered and we were blocked, resume
	if allAnswered && o.activeProject.Phase == PhaseBlocked {
		go o.runTaskBreakdownPhase(context.Background(), o.activeProject)
	}

	return nil
}

// GetPendingQuestions returns questions waiting for client answers
func (o *Orchestrator) GetPendingQuestions() []Question {
	if o.activeProject == nil {
		return nil
	}

	pending := make([]Question, 0)
	for _, q := range o.activeProject.PendingQuestions {
		if q.Status == "pending" {
			pending = append(pending, q)
		}
	}
	return pending
}

// GetProjectStatus returns the current project status
func (o *Orchestrator) GetProjectStatus() map[string]interface{} {
	if o.activeProject == nil {
		return nil
	}

	p := o.activeProject
	p.mu.RLock()
	defer p.mu.RUnlock()

	storySummary := make([]map[string]interface{}, 0)
	for _, s := range p.Stories {
		storySummary = append(storySummary, map[string]interface{}{
			"id":       s.ID,
			"title":    s.Title,
			"status":   s.Status,
			"assigned": s.AssignedMember,
		})
	}

	return map[string]interface{}{
		"id":                p.ID,
		"phase":             p.Phase,
		"requirements":      len(p.Requirements),
		"stories":           storySummary,
		"pending_questions": len(o.GetPendingQuestions()),
		"communications":    len(p.Communications),
	}
}

// Helper functions

func parseRequirements(content, createdBy string) []Requirement {
	var reqs []Requirement

	// Find JSON in the content
	start := strings.Index(content, "[")
	end := strings.LastIndex(content, "]")

	if start >= 0 && end > start {
		jsonStr := content[start : end+1]
		var parsed []struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			Priority    string `json:"priority"`
		}

		if err := json.Unmarshal([]byte(jsonStr), &parsed); err == nil {
			for _, r := range parsed {
				reqs = append(reqs, Requirement{
					ID:          uuid.New().String(),
					Title:       r.Title,
					Description: r.Description,
					Priority:    r.Priority,
					CreatedBy:   createdBy,
				})
			}
		}
	}

	return reqs
}

func parseStories(content string) []*Story {
	var stories []*Story

	start := strings.Index(content, "[")
	end := strings.LastIndex(content, "]")

	if start >= 0 && end > start {
		jsonStr := content[start : end+1]
		var parsed []struct {
			Title              string   `json:"title"`
			Description        string   `json:"description"`
			Type               string   `json:"type"`
			AssignedRole       string   `json:"assigned_role"`
			AcceptanceCriteria []string `json:"acceptance_criteria"`
			EstimatedEffort    string   `json:"estimated_effort"`
			RequirementID      string   `json:"requirement_id"`
		}

		if err := json.Unmarshal([]byte(jsonStr), &parsed); err == nil {
			for _, s := range parsed {
				stories = append(stories, &Story{
					ID:                 uuid.New().String(),
					Title:              s.Title,
					Description:        s.Description,
					Type:               s.Type,
					AssignedRole:       s.AssignedRole,
					AcceptanceCriteria: s.AcceptanceCriteria,
					EstimatedEffort:    s.EstimatedEffort,
					RequirementID:      s.RequirementID,
					Status:             StoryBacklog,
				})
			}
		}
	}

	return stories
}

func formatArtifacts(artifacts []Artifact) string {
	if len(artifacts) == 0 {
		return "(none)"
	}

	var sb strings.Builder
	for _, a := range artifacts {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", a.Type, a.Path))
	}
	return sb.String()
}

func formatAcceptanceCriteria(criteria []string) string {
	if len(criteria) == 0 {
		return "(none specified)"
	}

	var sb strings.Builder
	for _, c := range criteria {
		sb.WriteString(fmt.Sprintf("- %s\n", c))
	}
	return sb.String()
}
