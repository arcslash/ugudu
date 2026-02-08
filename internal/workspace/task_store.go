package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Task represents a project task
type Task struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description,omitempty"`
	Status      string                 `json:"status"` // pending, in_progress, completed, blocked
	Priority    string                 `json:"priority,omitempty"`
	AssignedTo  string                 `json:"assigned_to,omitempty"`
	CreatedBy   string                 `json:"created_by"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	DueDate     *time.Time             `json:"due_date,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Comments    []TaskComment          `json:"comments,omitempty"`
}

// TaskComment represents a comment on a task
type TaskComment struct {
	ID        string    `json:"id"`
	Author    string    `json:"author"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// TaskStore provides access to task storage
type TaskStore struct {
	path     string
	mu       sync.RWMutex
	tasks    []Task
	loaded   bool
	logger   *ActivityLogger
}

// NewTaskStore creates a new task store
func NewTaskStore(ws *Workspace) *TaskStore {
	return &TaskStore{
		path: ws.TasksPath(),
	}
}

// NewTaskStoreWithLogger creates a new task store with activity logging
func NewTaskStoreWithLogger(ws *Workspace, logger *ActivityLogger) *TaskStore {
	return &TaskStore{
		path:   ws.TasksPath(),
		logger: logger,
	}
}

// load reads tasks from disk
func (s *TaskStore) load() error {
	if s.loaded {
		return nil
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.tasks = []Task{}
			s.loaded = true
			return nil
		}
		return fmt.Errorf("read tasks: %w", err)
	}

	if err := json.Unmarshal(data, &s.tasks); err != nil {
		return fmt.Errorf("parse tasks: %w", err)
	}

	s.loaded = true
	return nil
}

// save writes tasks to disk
func (s *TaskStore) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return fmt.Errorf("create tasks directory: %w", err)
	}

	data, err := json.MarshalIndent(s.tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal tasks: %w", err)
	}

	return os.WriteFile(s.path, data, 0644)
}

// List returns all tasks
func (s *TaskStore) List() ([]Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := s.load(); err != nil {
		return nil, err
	}

	result := make([]Task, len(s.tasks))
	copy(result, s.tasks)
	return result, nil
}

// ListByStatus returns tasks with the given status
func (s *TaskStore) ListByStatus(status string) ([]Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := s.load(); err != nil {
		return nil, err
	}

	var result []Task
	for _, t := range s.tasks {
		if t.Status == status {
			result = append(result, t)
		}
	}
	return result, nil
}

// ListByAssignee returns tasks assigned to the given role
func (s *TaskStore) ListByAssignee(assignee string) ([]Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := s.load(); err != nil {
		return nil, err
	}

	var result []Task
	for _, t := range s.tasks {
		if t.AssignedTo == assignee {
			result = append(result, t)
		}
	}
	return result, nil
}

// Get returns a task by ID
func (s *TaskStore) Get(id string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := s.load(); err != nil {
		return nil, err
	}

	for _, t := range s.tasks {
		if t.ID == id {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("task not found: %s", id)
}

// Create creates a new task
func (s *TaskStore) Create(task *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.load(); err != nil {
		return err
	}

	// Generate ID if not set
	if task.ID == "" {
		task.ID = fmt.Sprintf("TASK-%d", time.Now().UnixNano()%1000000)
	}

	now := time.Now()
	task.CreatedAt = now
	task.UpdatedAt = now

	if task.Status == "" {
		task.Status = "pending"
	}

	s.tasks = append(s.tasks, *task)

	if err := s.save(); err != nil {
		return err
	}

	// Log activity
	if s.logger != nil {
		entry := TaskUpdateActivity(task.CreatedBy, "", task.ID, "", task.Status)
		entry.Data["action"] = "created"
		entry.Data["title"] = task.Title
		s.logger.Log(entry)
	}

	return nil
}

// Update updates an existing task
func (s *TaskStore) Update(task *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.load(); err != nil {
		return err
	}

	for i, t := range s.tasks {
		if t.ID == task.ID {
			oldStatus := t.Status
			task.UpdatedAt = time.Now()
			s.tasks[i] = *task

			if err := s.save(); err != nil {
				return err
			}

			// Log activity if status changed
			if s.logger != nil && oldStatus != task.Status {
				entry := TaskUpdateActivity("", "", task.ID, oldStatus, task.Status)
				s.logger.Log(entry)
			}

			return nil
		}
	}

	return fmt.Errorf("task not found: %s", task.ID)
}

// Delete removes a task
func (s *TaskStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.load(); err != nil {
		return err
	}

	for i, t := range s.tasks {
		if t.ID == id {
			s.tasks = append(s.tasks[:i], s.tasks[i+1:]...)
			return s.save()
		}
	}

	return fmt.Errorf("task not found: %s", id)
}

// AddComment adds a comment to a task
func (s *TaskStore) AddComment(taskID, author, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.load(); err != nil {
		return err
	}

	for i, t := range s.tasks {
		if t.ID == taskID {
			comment := TaskComment{
				ID:        fmt.Sprintf("CMT-%d", time.Now().UnixNano()%1000000),
				Author:    author,
				Content:   content,
				CreatedAt: time.Now(),
			}
			s.tasks[i].Comments = append(s.tasks[i].Comments, comment)
			s.tasks[i].UpdatedAt = time.Now()
			return s.save()
		}
	}

	return fmt.Errorf("task not found: %s", taskID)
}

// TaskStats provides statistics about tasks
type TaskStats struct {
	Total      int            `json:"total"`
	ByStatus   map[string]int `json:"by_status"`
	ByPriority map[string]int `json:"by_priority"`
	ByAssignee map[string]int `json:"by_assignee"`
	Overdue    int            `json:"overdue"`
}

// Stats returns statistics about tasks
func (s *TaskStore) Stats() (*TaskStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := s.load(); err != nil {
		return nil, err
	}

	stats := &TaskStats{
		Total:      len(s.tasks),
		ByStatus:   make(map[string]int),
		ByPriority: make(map[string]int),
		ByAssignee: make(map[string]int),
	}

	now := time.Now()
	for _, t := range s.tasks {
		stats.ByStatus[t.Status]++
		if t.Priority != "" {
			stats.ByPriority[t.Priority]++
		}
		if t.AssignedTo != "" {
			stats.ByAssignee[t.AssignedTo]++
		}
		if t.DueDate != nil && t.DueDate.Before(now) && t.Status != "completed" {
			stats.Overdue++
		}
	}

	return stats, nil
}
