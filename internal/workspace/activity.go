package workspace

import (
	"time"
)

// ActivityType represents the type of activity
type ActivityType string

const (
	// ActivityToolCall represents a tool execution
	ActivityToolCall ActivityType = "tool_call"
	// ActivityDelegation represents a task delegation
	ActivityDelegation ActivityType = "delegation"
	// ActivityTaskUpdate represents a task status change
	ActivityTaskUpdate ActivityType = "task_update"
	// ActivityMessage represents a message sent/received
	ActivityMessage ActivityType = "message"
	// ActivityProgress represents a progress report
	ActivityProgress ActivityType = "progress"
	// ActivityError represents an error occurrence
	ActivityError ActivityType = "error"
)

// ActivityEntry represents a single activity log entry
type ActivityEntry struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Type      ActivityType           `json:"type"`
	AgentID   string                 `json:"agent_id"`
	AgentRole string                 `json:"agent_role"`
	TaskID    string                 `json:"task_id,omitempty"`
	Data      map[string]interface{} `json:"data"`
	Success   bool                   `json:"success"`
	Duration  int64                  `json:"duration_ms,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// NewActivityEntry creates a new activity entry
func NewActivityEntry(actType ActivityType, agentID, agentRole string) *ActivityEntry {
	return &ActivityEntry{
		ID:        generateID(),
		Timestamp: time.Now(),
		Type:      actType,
		AgentID:   agentID,
		AgentRole: agentRole,
		Success:   true,
		Data:      make(map[string]interface{}),
	}
}

// WithTask sets the task ID for the activity
func (e *ActivityEntry) WithTask(taskID string) *ActivityEntry {
	e.TaskID = taskID
	return e
}

// WithData adds data to the activity
func (e *ActivityEntry) WithData(key string, value interface{}) *ActivityEntry {
	e.Data[key] = value
	return e
}

// WithError marks the activity as failed with an error
func (e *ActivityEntry) WithError(err error) *ActivityEntry {
	e.Success = false
	if err != nil {
		e.Error = err.Error()
	}
	return e
}

// WithDuration sets the duration of the activity
func (e *ActivityEntry) WithDuration(d time.Duration) *ActivityEntry {
	e.Duration = d.Milliseconds()
	return e
}

// ToolCallActivity creates an activity entry for a tool call
func ToolCallActivity(agentID, agentRole, toolName string, args map[string]interface{}) *ActivityEntry {
	entry := NewActivityEntry(ActivityToolCall, agentID, agentRole)
	entry.Data["tool"] = toolName
	entry.Data["args"] = sanitizeArgs(args)
	return entry
}

// DelegationActivity creates an activity entry for a delegation
func DelegationActivity(agentID, agentRole, toRole, taskID, message string) *ActivityEntry {
	entry := NewActivityEntry(ActivityDelegation, agentID, agentRole)
	entry.TaskID = taskID
	entry.Data["to_role"] = toRole
	entry.Data["message"] = message
	return entry
}

// TaskUpdateActivity creates an activity entry for a task update
func TaskUpdateActivity(agentID, agentRole, taskID, oldStatus, newStatus string) *ActivityEntry {
	entry := NewActivityEntry(ActivityTaskUpdate, agentID, agentRole)
	entry.TaskID = taskID
	entry.Data["old_status"] = oldStatus
	entry.Data["new_status"] = newStatus
	return entry
}

// MessageActivity creates an activity entry for a message
func MessageActivity(agentID, agentRole, direction, toFrom, content string) *ActivityEntry {
	entry := NewActivityEntry(ActivityMessage, agentID, agentRole)
	entry.Data["direction"] = direction // "sent" or "received"
	entry.Data["to_from"] = toFrom
	entry.Data["content"] = truncate(content, 500)
	return entry
}

// ProgressActivity creates an activity entry for progress reporting
func ProgressActivity(agentID, agentRole, taskID, status string, percentComplete float64) *ActivityEntry {
	entry := NewActivityEntry(ActivityProgress, agentID, agentRole)
	entry.TaskID = taskID
	entry.Data["status"] = status
	entry.Data["percent_complete"] = percentComplete
	return entry
}

// ErrorActivity creates an activity entry for an error
func ErrorActivity(agentID, agentRole, context string, err error) *ActivityEntry {
	entry := NewActivityEntry(ActivityError, agentID, agentRole)
	entry.Success = false
	entry.Data["context"] = context
	if err != nil {
		entry.Error = err.Error()
	}
	return entry
}

// sanitizeArgs removes sensitive data from arguments
func sanitizeArgs(args map[string]interface{}) map[string]interface{} {
	sanitized := make(map[string]interface{})
	sensitiveKeys := map[string]bool{
		"password": true,
		"secret":   true,
		"token":    true,
		"api_key":  true,
		"apikey":   true,
	}

	for k, v := range args {
		if sensitiveKeys[k] {
			sanitized[k] = "[REDACTED]"
		} else if s, ok := v.(string); ok && len(s) > 1000 {
			sanitized[k] = truncate(s, 1000)
		} else {
			sanitized[k] = v
		}
	}

	return sanitized
}

// truncate truncates a string to a maximum length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// generateID generates a unique ID for an activity entry
func generateID() string {
	return time.Now().Format("20060102150405.000000")
}
