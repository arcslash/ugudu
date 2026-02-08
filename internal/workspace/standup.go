package workspace

import (
	"time"
)

// StandupPeriod represents the reporting period
type StandupPeriod string

const (
	PeriodDaily  StandupPeriod = "daily"
	PeriodWeekly StandupPeriod = "weekly"
)

// StandupReport represents a standup/status report
type StandupReport struct {
	ProjectName   string          `json:"project_name"`
	GeneratedAt   time.Time       `json:"generated_at"`
	Period        StandupPeriod   `json:"period"`
	PeriodStart   time.Time       `json:"period_start"`
	PeriodEnd     time.Time       `json:"period_end"`
	TeamSummary   TeamSummary     `json:"team_summary"`
	MemberReports []MemberReport  `json:"member_reports"`
	TaskSummary   TaskSummary     `json:"task_summary"`
	Highlights    []string        `json:"highlights"`
	Blockers      []string        `json:"blockers"`
	NextSteps     []string        `json:"next_steps"`
}

// TeamSummary provides an overview of team activity
type TeamSummary struct {
	ActiveMembers    int     `json:"active_members"`
	TotalToolCalls   int     `json:"total_tool_calls"`
	TotalDelegations int     `json:"total_delegations"`
	SuccessRate      float64 `json:"success_rate"`
	TotalDuration    int64   `json:"total_duration_ms"`
}

// MemberReport provides activity details for a single team member
type MemberReport struct {
	Role            string                  `json:"role"`
	AgentID         string                  `json:"agent_id,omitempty"`
	Status          string                  `json:"status"` // active, idle, blocked
	ToolCalls       int                     `json:"tool_calls"`
	SuccessfulCalls int                     `json:"successful_calls"`
	FailedCalls     int                     `json:"failed_calls"`
	Delegations     int                     `json:"delegations"`
	TasksCompleted  int                     `json:"tasks_completed"`
	TasksInProgress int                     `json:"tasks_in_progress"`
	TopTools        []ToolUsage             `json:"top_tools,omitempty"`
	RecentActivity  []ActivityEntry         `json:"recent_activity,omitempty"`
	Blockers        []string                `json:"blockers,omitempty"`
}

// ToolUsage tracks usage of a specific tool
type ToolUsage struct {
	Tool   string  `json:"tool"`
	Count  int     `json:"count"`
	AvgDuration int64 `json:"avg_duration_ms,omitempty"`
}

// TaskSummary summarizes task progress
type TaskSummary struct {
	TotalTasks      int            `json:"total_tasks"`
	Completed       int            `json:"completed"`
	InProgress      int            `json:"in_progress"`
	Pending         int            `json:"pending"`
	Blocked         int            `json:"blocked"`
	CompletedToday  int            `json:"completed_today,omitempty"`
	CompletedPeriod int            `json:"completed_period"`
	ByAssignee      map[string]int `json:"by_assignee,omitempty"`
}

// GetPeriodBounds returns the start and end times for a standup period
func GetPeriodBounds(period StandupPeriod) (start, end time.Time) {
	now := time.Now()
	end = now

	switch period {
	case PeriodDaily:
		// Start of today (midnight)
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case PeriodWeekly:
		// Start of the week (Monday)
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7 // Sunday is 7
		}
		daysBack := weekday - 1 // Days since Monday
		start = time.Date(now.Year(), now.Month(), now.Day()-daysBack, 0, 0, 0, 0, now.Location())
	default:
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	}

	return start, end
}
