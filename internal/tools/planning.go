package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// Task represents a project task
type Task struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description,omitempty"`
	Status      string                 `json:"status"` // pending, in_progress, completed, blocked
	Priority    string                 `json:"priority,omitempty"` // low, medium, high, urgent
	AssignedTo  string                 `json:"assigned_to,omitempty"`
	CreatedBy   string                 `json:"created_by"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	DueDate     *time.Time             `json:"due_date,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// TaskStore provides access to task storage
type TaskStore interface {
	List() ([]Task, error)
	Get(id string) (*Task, error)
	Create(task *Task) error
	Update(task *Task) error
	Delete(id string) error
}

// FileTaskStore stores tasks in a JSON file
type FileTaskStore struct {
	path string
}

// NewFileTaskStore creates a task store backed by a file
func NewFileTaskStore(path string) *FileTaskStore {
	return &FileTaskStore{path: path}
}

func (s *FileTaskStore) load() ([]Task, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Task{}, nil
		}
		return nil, err
	}

	var tasks []Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (s *FileTaskStore) save(tasks []Task) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

func (s *FileTaskStore) List() ([]Task, error) {
	return s.load()
}

func (s *FileTaskStore) Get(id string) (*Task, error) {
	tasks, err := s.load()
	if err != nil {
		return nil, err
	}

	for _, t := range tasks {
		if t.ID == id {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("task not found: %s", id)
}

func (s *FileTaskStore) Create(task *Task) error {
	tasks, err := s.load()
	if err != nil {
		return err
	}

	if task.ID == "" {
		task.ID = uuid.New().String()[:8]
	}
	now := time.Now()
	task.CreatedAt = now
	task.UpdatedAt = now
	if task.Status == "" {
		task.Status = "pending"
	}

	tasks = append(tasks, *task)
	return s.save(tasks)
}

func (s *FileTaskStore) Update(task *Task) error {
	tasks, err := s.load()
	if err != nil {
		return err
	}

	for i, t := range tasks {
		if t.ID == task.ID {
			task.UpdatedAt = time.Now()
			tasks[i] = *task
			return s.save(tasks)
		}
	}
	return fmt.Errorf("task not found: %s", task.ID)
}

func (s *FileTaskStore) Delete(id string) error {
	tasks, err := s.load()
	if err != nil {
		return err
	}

	for i, t := range tasks {
		if t.ID == id {
			tasks = append(tasks[:i], tasks[i+1:]...)
			return s.save(tasks)
		}
	}
	return fmt.Errorf("task not found: %s", id)
}

// ============================================================================
// Planning Tools
// ============================================================================

// CreateTaskTool creates a new task
type CreateTaskTool struct {
	Store     TaskStore
	CreatedBy string
}

func (t *CreateTaskTool) Name() string        { return "create_task" }
func (t *CreateTaskTool) Description() string { return "Create a new task in the project" }

func (t *CreateTaskTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	title, ok := args["title"].(string)
	if !ok || title == "" {
		return nil, fmt.Errorf("title is required")
	}

	task := &Task{
		Title:     title,
		CreatedBy: t.CreatedBy,
		Status:    "pending",
	}

	if desc, ok := args["description"].(string); ok {
		task.Description = desc
	}
	if priority, ok := args["priority"].(string); ok {
		task.Priority = priority
	}
	if assignee, ok := args["assigned_to"].(string); ok {
		task.AssignedTo = assignee
	}
	if tags, ok := args["tags"].([]interface{}); ok {
		for _, tag := range tags {
			if s, ok := tag.(string); ok {
				task.Tags = append(task.Tags, s)
			}
		}
	}

	if err := t.Store.Create(task); err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}

	return map[string]interface{}{
		"id":         task.ID,
		"title":      task.Title,
		"status":     task.Status,
		"created_at": task.CreatedAt,
	}, nil
}

// UpdateTaskTool updates an existing task
type UpdateTaskTool struct {
	Store     TaskStore
	UpdatedBy string
}

func (t *UpdateTaskTool) Name() string        { return "update_task" }
func (t *UpdateTaskTool) Description() string { return "Update an existing task" }

func (t *UpdateTaskTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	id, ok := args["id"].(string)
	if !ok || id == "" {
		return nil, fmt.Errorf("id is required")
	}

	task, err := t.Store.Get(id)
	if err != nil {
		return nil, err
	}

	if title, ok := args["title"].(string); ok {
		task.Title = title
	}
	if desc, ok := args["description"].(string); ok {
		task.Description = desc
	}
	if status, ok := args["status"].(string); ok {
		task.Status = status
	}
	if priority, ok := args["priority"].(string); ok {
		task.Priority = priority
	}
	if assignee, ok := args["assigned_to"].(string); ok {
		task.AssignedTo = assignee
	}

	if err := t.Store.Update(task); err != nil {
		return nil, fmt.Errorf("update task: %w", err)
	}

	return map[string]interface{}{
		"id":         task.ID,
		"title":      task.Title,
		"status":     task.Status,
		"updated_at": task.UpdatedAt,
	}, nil
}

// ListTasksTool lists all tasks
type ListTasksTool struct {
	Store TaskStore
}

func (t *ListTasksTool) Name() string        { return "list_tasks" }
func (t *ListTasksTool) Description() string { return "List all tasks in the project" }

func (t *ListTasksTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	tasks, err := t.Store.List()
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	// Apply filters
	status, hasStatus := args["status"].(string)
	assignee, hasAssignee := args["assigned_to"].(string)

	var filtered []Task
	for _, task := range tasks {
		if hasStatus && task.Status != status {
			continue
		}
		if hasAssignee && task.AssignedTo != assignee {
			continue
		}
		filtered = append(filtered, task)
	}

	return map[string]interface{}{
		"tasks": filtered,
		"count": len(filtered),
	}, nil
}

// AssignTaskTool assigns a task to a team member
type AssignTaskTool struct {
	Store      TaskStore
	AssignedBy string
}

func (t *AssignTaskTool) Name() string        { return "assign_task" }
func (t *AssignTaskTool) Description() string { return "Assign a task to a team member" }

func (t *AssignTaskTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	id, ok := args["id"].(string)
	if !ok || id == "" {
		return nil, fmt.Errorf("id is required")
	}

	assignee, ok := args["assigned_to"].(string)
	if !ok || assignee == "" {
		return nil, fmt.Errorf("assigned_to is required")
	}

	task, err := t.Store.Get(id)
	if err != nil {
		return nil, err
	}

	task.AssignedTo = assignee
	if err := t.Store.Update(task); err != nil {
		return nil, fmt.Errorf("assign task: %w", err)
	}

	return map[string]interface{}{
		"id":          task.ID,
		"title":       task.Title,
		"assigned_to": task.AssignedTo,
	}, nil
}

// CreateReportTool creates a project report
type CreateReportTool struct {
	ArtifactPath string
	CreatedBy    string
}

func (t *CreateReportTool) Name() string        { return "create_report" }
func (t *CreateReportTool) Description() string { return "Create a project report" }

func (t *CreateReportTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	title, ok := args["title"].(string)
	if !ok || title == "" {
		return nil, fmt.Errorf("title is required")
	}

	content, ok := args["content"].(string)
	if !ok || content == "" {
		return nil, fmt.Errorf("content is required")
	}

	reportType := "general"
	if rt, ok := args["type"].(string); ok {
		reportType = rt
	}

	// Generate filename
	timestamp := time.Now().Format("2006-01-02-150405")
	filename := fmt.Sprintf("%s-%s.md", reportType, timestamp)
	reportPath := filepath.Join(t.ArtifactPath, "reports", filename)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(reportPath), 0755); err != nil {
		return nil, fmt.Errorf("create report directory: %w", err)
	}

	// Add metadata header
	fullContent := fmt.Sprintf("# %s\n\n*Created by: %s*\n*Date: %s*\n\n---\n\n%s",
		title, t.CreatedBy, time.Now().Format(time.RFC3339), content)

	if err := os.WriteFile(reportPath, []byte(fullContent), 0644); err != nil {
		return nil, fmt.Errorf("write report: %w", err)
	}

	return map[string]interface{}{
		"path":       reportPath,
		"title":      title,
		"type":       reportType,
		"created_at": time.Now(),
	}, nil
}

// DelegateTaskTool delegates a task to another team member
type DelegateTaskTool struct {
	Store       TaskStore
	DelegatedBy string
	// DelegateFunc is called to actually delegate (set by member)
	DelegateFunc func(ctx context.Context, role string, taskID string, message string) error
}

func (t *DelegateTaskTool) Name() string        { return "delegate_task" }
func (t *DelegateTaskTool) Description() string { return "Delegate a task to another team member" }

func (t *DelegateTaskTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	taskID, ok := args["task_id"].(string)
	if !ok || taskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}

	role, ok := args["to_role"].(string)
	if !ok || role == "" {
		return nil, fmt.Errorf("to_role is required")
	}

	message := ""
	if msg, ok := args["message"].(string); ok {
		message = msg
	}

	// Update task assignee
	task, err := t.Store.Get(taskID)
	if err != nil {
		return nil, err
	}
	task.AssignedTo = role
	if err := t.Store.Update(task); err != nil {
		return nil, err
	}

	// Call delegate function if available
	if t.DelegateFunc != nil {
		if err := t.DelegateFunc(ctx, role, taskID, message); err != nil {
			return nil, fmt.Errorf("delegate: %w", err)
		}
	}

	return map[string]interface{}{
		"task_id":     taskID,
		"delegated_to": role,
		"message":     message,
	}, nil
}
