// Package team provides the workflow engine for multi-agent collaboration
package team

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ProjectPhase represents the current phase of a project
type ProjectPhase string

const (
	PhaseInitiated      ProjectPhase = "initiated"
	PhasePlanning       ProjectPhase = "planning"
	PhaseRequirements   ProjectPhase = "requirements"
	PhaseTaskBreakdown  ProjectPhase = "task_breakdown"
	PhaseExecution      ProjectPhase = "execution"
	PhaseReview         ProjectPhase = "review"
	PhaseComplete       ProjectPhase = "complete"
	PhaseBlocked        ProjectPhase = "blocked"
)

// Project represents a client request that requires multi-agent collaboration
type Project struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Phase       ProjectPhase           `json:"phase"`
	ClientID    string                 `json:"client_id"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`

	// Requirements gathered during planning
	Requirements []Requirement `json:"requirements"`

	// Stories broken down from requirements
	Stories []*Story `json:"stories"`

	// Questions pending client response
	PendingQuestions []Question `json:"pending_questions"`

	// Communication log
	Communications []Communication `json:"communications"`

	// Metadata
	Metadata map[string]interface{} `json:"metadata"`

	mu sync.RWMutex
}

// Requirement represents a product requirement
type Requirement struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Priority    string   `json:"priority"` // must, should, could, wont
	CreatedBy   string   `json:"created_by"`
	Stories     []string `json:"stories"` // Story IDs derived from this requirement
}

// Story represents a user story/task for engineers
type Story struct {
	ID                 string            `json:"id"`
	Title              string            `json:"title"`
	Description        string            `json:"description"`
	Type               string            `json:"type"` // feature, bug, task, spike
	AcceptanceCriteria []string          `json:"acceptance_criteria"`
	TechnicalNotes     string            `json:"technical_notes"`
	EstimatedEffort    string            `json:"estimated_effort"` // small, medium, large
	Priority           int               `json:"priority"`
	RequirementID      string            `json:"requirement_id"`

	// Assignment
	AssignedRole   string `json:"assigned_role"`   // e.g., "backend", "frontend"
	AssignedMember string `json:"assigned_member"` // Specific member ID

	// Status tracking
	Status      StoryStatus `json:"status"`
	StartedAt   *time.Time  `json:"started_at,omitempty"`
	CompletedAt *time.Time  `json:"completed_at,omitempty"`

	// Work artifacts
	Artifacts []Artifact `json:"artifacts"`

	// Questions from engineers
	Questions []Question `json:"questions"`

	// Sub-tasks
	SubTasks []SubTask `json:"subtasks"`

	mu sync.RWMutex
}

// StoryStatus represents the status of a story
type StoryStatus string

const (
	StoryBacklog    StoryStatus = "backlog"
	StoryReady      StoryStatus = "ready"
	StoryInProgress StoryStatus = "in_progress"
	StoryBlocked    StoryStatus = "blocked"
	StoryReview     StoryStatus = "review"
	StoryDone       StoryStatus = "done"
)

// SubTask is a smaller unit of work within a story
type SubTask struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Completed   bool      `json:"completed"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

// Artifact represents work output (file, commit, etc.)
type Artifact struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // file, commit, test_result
	Path      string    `json:"path"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// Question represents a question that needs answering
type Question struct {
	ID         string    `json:"id"`
	Content    string    `json:"content"`
	FromRole   string    `json:"from_role"`
	FromMember string    `json:"from_member"`
	ToRole     string    `json:"to_role"` // "client", "pm", "ba", etc.
	Context    string    `json:"context"` // Story ID, Requirement ID, etc.
	Status     string    `json:"status"`  // pending, answered, dismissed
	Answer     string    `json:"answer,omitempty"`
	AnsweredBy string    `json:"answered_by,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	AnsweredAt time.Time `json:"answered_at,omitempty"`
}

// Communication represents a message in the project log
type Communication struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // message, decision, status_update, question, answer
	From      string    `json:"from"`
	To        string    `json:"to"` // Role, member, or "all"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ProjectManager handles project workflow
type ProjectManager struct {
	projects map[string]*Project
	team     *Team
	mu       sync.RWMutex
}

// NewProjectManager creates a new project manager
func NewProjectManager(team *Team) *ProjectManager {
	return &ProjectManager{
		projects: make(map[string]*Project),
		team:     team,
	}
}

// CreateProject creates a new project from a client request
func (pm *ProjectManager) CreateProject(name, description, clientRequest string) *Project {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	project := &Project{
		ID:               uuid.New().String(),
		Name:             name,
		Description:      description,
		Phase:            PhaseInitiated,
		ClientID:         "client",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		Requirements:     make([]Requirement, 0),
		Stories:          make([]*Story, 0),
		PendingQuestions: make([]Question, 0),
		Communications:   make([]Communication, 0),
		Metadata:         make(map[string]interface{}),
	}

	// Add initial client request as first communication
	project.Communications = append(project.Communications, Communication{
		ID:        uuid.New().String(),
		Type:      "message",
		From:      "client",
		To:        "pm",
		Content:   clientRequest,
		Timestamp: time.Now(),
	})

	pm.projects[project.ID] = project
	return project
}

// GetProject retrieves a project by ID
func (pm *ProjectManager) GetProject(id string) *Project {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.projects[id]
}

// ListProjects returns all projects
func (pm *ProjectManager) ListProjects() []*Project {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	projects := make([]*Project, 0, len(pm.projects))
	for _, p := range pm.projects {
		projects = append(projects, p)
	}
	return projects
}

// AddRequirement adds a requirement to the project
func (p *Project) AddRequirement(title, description, priority, createdBy string) *Requirement {
	p.mu.Lock()
	defer p.mu.Unlock()

	req := Requirement{
		ID:          uuid.New().String(),
		Title:       title,
		Description: description,
		Priority:    priority,
		CreatedBy:   createdBy,
		Stories:     make([]string, 0),
	}

	p.Requirements = append(p.Requirements, req)
	p.UpdatedAt = time.Now()
	return &req
}

// CreateStory creates a story from a requirement
func (p *Project) CreateStory(reqID, title, description, storyType, assignedRole string, criteria []string) *Story {
	p.mu.Lock()
	defer p.mu.Unlock()

	story := &Story{
		ID:                 uuid.New().String(),
		Title:              title,
		Description:        description,
		Type:               storyType,
		AcceptanceCriteria: criteria,
		RequirementID:      reqID,
		AssignedRole:       assignedRole,
		Status:             StoryBacklog,
		Questions:          make([]Question, 0),
		SubTasks:           make([]SubTask, 0),
		Artifacts:          make([]Artifact, 0),
	}

	p.Stories = append(p.Stories, story)

	// Link story to requirement
	for i := range p.Requirements {
		if p.Requirements[i].ID == reqID {
			p.Requirements[i].Stories = append(p.Requirements[i].Stories, story.ID)
			break
		}
	}

	p.UpdatedAt = time.Now()
	return story
}

// AskQuestion adds a question that needs answering
func (p *Project) AskQuestion(content, fromRole, fromMember, toRole, context string) *Question {
	p.mu.Lock()
	defer p.mu.Unlock()

	q := Question{
		ID:         uuid.New().String(),
		Content:    content,
		FromRole:   fromRole,
		FromMember: fromMember,
		ToRole:     toRole,
		Context:    context,
		Status:     "pending",
		CreatedAt:  time.Now(),
	}

	if toRole == "client" {
		p.PendingQuestions = append(p.PendingQuestions, q)
	}

	// Add to communications log
	p.Communications = append(p.Communications, Communication{
		ID:        uuid.New().String(),
		Type:      "question",
		From:      fromMember,
		To:        toRole,
		Content:   content,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"question_id": q.ID,
			"context":     context,
		},
	})

	p.UpdatedAt = time.Now()
	return &q
}

// AnswerQuestion provides an answer to a pending question
func (p *Project) AnswerQuestion(questionID, answer, answeredBy string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i := range p.PendingQuestions {
		if p.PendingQuestions[i].ID == questionID {
			p.PendingQuestions[i].Answer = answer
			p.PendingQuestions[i].AnsweredBy = answeredBy
			p.PendingQuestions[i].Status = "answered"
			p.PendingQuestions[i].AnsweredAt = time.Now()

			// Add to communications log
			p.Communications = append(p.Communications, Communication{
				ID:        uuid.New().String(),
				Type:      "answer",
				From:      answeredBy,
				To:        p.PendingQuestions[i].FromMember,
				Content:   answer,
				Timestamp: time.Now(),
				Metadata: map[string]interface{}{
					"question_id": questionID,
				},
			})

			p.UpdatedAt = time.Now()
			return nil
		}
	}

	return fmt.Errorf("question not found: %s", questionID)
}

// SetPhase updates the project phase
func (p *Project) SetPhase(phase ProjectPhase) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Phase = phase
	p.UpdatedAt = time.Now()

	// Add phase change to communications
	p.Communications = append(p.Communications, Communication{
		ID:        uuid.New().String(),
		Type:      "status_update",
		From:      "system",
		To:        "all",
		Content:   fmt.Sprintf("Project phase changed to: %s", phase),
		Timestamp: time.Now(),
	})
}

// AddCommunication adds a message to the project log
func (p *Project) AddCommunication(msgType, from, to, content string, metadata map[string]interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.Communications = append(p.Communications, Communication{
		ID:        uuid.New().String(),
		Type:      msgType,
		From:      from,
		To:        to,
		Content:   content,
		Timestamp: time.Now(),
		Metadata:  metadata,
	})
	p.UpdatedAt = time.Now()
}

// GetStoriesForRole returns stories assigned to a role
func (p *Project) GetStoriesForRole(role string) []*Story {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stories := make([]*Story, 0)
	for _, s := range p.Stories {
		if s.AssignedRole == role {
			stories = append(stories, s)
		}
	}
	return stories
}

// GetReadyStories returns stories that are ready to work on
func (p *Project) GetReadyStories() []*Story {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stories := make([]*Story, 0)
	for _, s := range p.Stories {
		if s.Status == StoryReady {
			stories = append(stories, s)
		}
	}
	return stories
}

// UpdateStoryStatus updates a story's status
func (s *Story) UpdateStatus(status StoryStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Status = status
	now := time.Now()

	if status == StoryInProgress && s.StartedAt == nil {
		s.StartedAt = &now
	}
	if status == StoryDone {
		s.CompletedAt = &now
	}
}

// AddArtifact adds a work artifact to a story
func (s *Story) AddArtifact(artifactType, path, createdBy string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Artifacts = append(s.Artifacts, Artifact{
		ID:        uuid.New().String(),
		Type:      artifactType,
		Path:      path,
		CreatedBy: createdBy,
		CreatedAt: time.Now(),
	})
}

// AskQuestion adds a question to a story
func (s *Story) AskQuestion(content, fromRole, fromMember, toRole string) *Question {
	s.mu.Lock()
	defer s.mu.Unlock()

	q := Question{
		ID:         uuid.New().String(),
		Content:    content,
		FromRole:   fromRole,
		FromMember: fromMember,
		ToRole:     toRole,
		Context:    s.ID,
		Status:     "pending",
		CreatedAt:  time.Now(),
	}

	s.Questions = append(s.Questions, q)
	return &q
}
