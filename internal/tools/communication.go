package tools

import (
	"context"
	"fmt"
	"time"
)

// ColleagueFunc is a function type for communicating with colleagues
type ColleagueFunc func(ctx context.Context, role string, message string) (string, error)

// ProgressFunc is a function type for reporting progress
type ProgressFunc func(ctx context.Context, status string, details map[string]interface{}) error

// ============================================================================
// Communication Tools
// ============================================================================

// AskColleagueTool asks another team member a question
type AskColleagueTool struct {
	FromRole    string
	AskFunc     ColleagueFunc
}

func (t *AskColleagueTool) Name() string        { return "ask_colleague" }
func (t *AskColleagueTool) Description() string { return "Ask a question to another team member" }

func (t *AskColleagueTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	role, ok := args["role"].(string)
	if !ok || role == "" {
		return nil, fmt.Errorf("role is required (e.g., engineer, pm, qa, ba)")
	}

	question, ok := args["question"].(string)
	if !ok || question == "" {
		return nil, fmt.Errorf("question is required")
	}

	if t.AskFunc == nil {
		return nil, fmt.Errorf("ask function not configured")
	}

	response, err := t.AskFunc(ctx, role, question)
	if err != nil {
		return nil, fmt.Errorf("ask %s: %w", role, err)
	}

	return map[string]interface{}{
		"role":      role,
		"question":  question,
		"response":  response,
		"asked_by":  t.FromRole,
		"timestamp": time.Now(),
	}, nil
}

// ReportProgressTool reports progress on current work
type ReportProgressTool struct {
	FromRole   string
	TaskID     string
	ReportFunc ProgressFunc
}

func (t *ReportProgressTool) Name() string        { return "report_progress" }
func (t *ReportProgressTool) Description() string { return "Report progress on current work" }

func (t *ReportProgressTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	status, ok := args["status"].(string)
	if !ok || status == "" {
		return nil, fmt.Errorf("status is required (e.g., in_progress, blocked, completed)")
	}

	message := ""
	if msg, ok := args["message"].(string); ok {
		message = msg
	}

	percentComplete := 0.0
	if pct, ok := args["percent_complete"].(float64); ok {
		percentComplete = pct
	}

	blockers := []string{}
	if b, ok := args["blockers"].([]interface{}); ok {
		for _, blocker := range b {
			if s, ok := blocker.(string); ok {
				blockers = append(blockers, s)
			}
		}
	}

	nextSteps := []string{}
	if ns, ok := args["next_steps"].([]interface{}); ok {
		for _, step := range ns {
			if s, ok := step.(string); ok {
				nextSteps = append(nextSteps, s)
			}
		}
	}

	details := map[string]interface{}{
		"message":          message,
		"percent_complete": percentComplete,
		"blockers":         blockers,
		"next_steps":       nextSteps,
		"from_role":        t.FromRole,
		"task_id":          t.TaskID,
	}

	if t.ReportFunc != nil {
		if err := t.ReportFunc(ctx, status, details); err != nil {
			return nil, fmt.Errorf("report progress: %w", err)
		}
	}

	return map[string]interface{}{
		"reported":         true,
		"status":           status,
		"message":          message,
		"percent_complete": percentComplete,
		"blockers":         blockers,
		"next_steps":       nextSteps,
		"timestamp":        time.Now(),
	}, nil
}
