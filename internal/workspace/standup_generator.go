package workspace

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// StandupGenerator generates standup reports from activity data
type StandupGenerator struct {
	workspace *Workspace
}

// NewStandupGenerator creates a new standup generator
func NewStandupGenerator(ws *Workspace) *StandupGenerator {
	return &StandupGenerator{workspace: ws}
}

// Generate creates a standup report for the given period
func (g *StandupGenerator) Generate(period StandupPeriod) (*StandupReport, error) {
	start, end := GetPeriodBounds(period)

	// Query activity for the period
	opts := QueryOptions{
		Since: start,
		Until: end,
	}
	activities, err := QueryProjectActivity(g.workspace, opts)
	if err != nil {
		return nil, fmt.Errorf("query activity: %w", err)
	}

	// Get task store for task summary
	taskStore := NewTaskStore(g.workspace)
	tasks, err := taskStore.List()
	if err != nil {
		tasks = []Task{} // Continue without tasks
	}

	report := &StandupReport{
		ProjectName: g.workspace.Name,
		GeneratedAt: time.Now(),
		Period:      period,
		PeriodStart: start,
		PeriodEnd:   end,
	}

	// Generate summaries
	report.TeamSummary = g.generateTeamSummary(activities)
	report.MemberReports = g.generateMemberReports(activities)
	report.TaskSummary = g.generateTaskSummary(tasks, start)
	report.Highlights = g.extractHighlights(activities, tasks)
	report.Blockers = g.extractBlockers(activities, tasks)
	report.NextSteps = g.suggestNextSteps(tasks)

	return report, nil
}

// generateTeamSummary creates a summary of team activity
func (g *StandupGenerator) generateTeamSummary(activities []ActivityEntry) TeamSummary {
	summary := TeamSummary{}

	roles := make(map[string]bool)
	successCount := 0
	var totalDuration int64

	for _, a := range activities {
		roles[a.AgentRole] = true

		if a.Type == ActivityToolCall {
			summary.TotalToolCalls++
			if a.Success {
				successCount++
			}
			totalDuration += a.Duration
		}

		if a.Type == ActivityDelegation {
			summary.TotalDelegations++
		}
	}

	summary.ActiveMembers = len(roles)
	summary.TotalDuration = totalDuration

	if summary.TotalToolCalls > 0 {
		summary.SuccessRate = float64(successCount) / float64(summary.TotalToolCalls) * 100
	}

	return summary
}

// generateMemberReports creates per-member reports
func (g *StandupGenerator) generateMemberReports(activities []ActivityEntry) []MemberReport {
	// Group activities by role
	byRole := make(map[string][]ActivityEntry)
	for _, a := range activities {
		byRole[a.AgentRole] = append(byRole[a.AgentRole], a)
	}

	var reports []MemberReport
	for role, roleActivities := range byRole {
		report := g.generateSingleMemberReport(role, roleActivities)
		reports = append(reports, report)
	}

	// Sort by role name
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].Role < reports[j].Role
	})

	return reports
}

// generateSingleMemberReport creates a report for one member
func (g *StandupGenerator) generateSingleMemberReport(role string, activities []ActivityEntry) MemberReport {
	report := MemberReport{
		Role:   role,
		Status: "active",
	}

	toolCounts := make(map[string]*ToolUsage)

	for _, a := range activities {
		if report.AgentID == "" {
			report.AgentID = a.AgentID
		}

		switch a.Type {
		case ActivityToolCall:
			report.ToolCalls++
			if a.Success {
				report.SuccessfulCalls++
			} else {
				report.FailedCalls++
			}

			// Track tool usage
			if toolName, ok := a.Data["tool"].(string); ok {
				if _, exists := toolCounts[toolName]; !exists {
					toolCounts[toolName] = &ToolUsage{Tool: toolName}
				}
				toolCounts[toolName].Count++
			}

		case ActivityDelegation:
			report.Delegations++

		case ActivityTaskUpdate:
			if newStatus, ok := a.Data["new_status"].(string); ok {
				if newStatus == "completed" {
					report.TasksCompleted++
				} else if newStatus == "in_progress" {
					report.TasksInProgress++
				}
			}

		case ActivityError:
			if a.Error != "" {
				report.Blockers = append(report.Blockers, a.Error)
			}
		}
	}

	// Get top tools
	var toolUsages []ToolUsage
	for _, usage := range toolCounts {
		toolUsages = append(toolUsages, *usage)
	}
	sort.Slice(toolUsages, func(i, j int) bool {
		return toolUsages[i].Count > toolUsages[j].Count
	})
	if len(toolUsages) > 5 {
		toolUsages = toolUsages[:5]
	}
	report.TopTools = toolUsages

	// Get recent activity (last 5)
	if len(activities) > 5 {
		report.RecentActivity = activities[:5]
	} else {
		report.RecentActivity = activities
	}

	// Determine status
	if len(report.Blockers) > 0 {
		report.Status = "blocked"
	} else if len(activities) == 0 {
		report.Status = "idle"
	}

	return report
}

// generateTaskSummary summarizes task progress
func (g *StandupGenerator) generateTaskSummary(tasks []Task, periodStart time.Time) TaskSummary {
	summary := TaskSummary{
		TotalTasks: len(tasks),
		ByAssignee: make(map[string]int),
	}

	today := time.Now().Truncate(24 * time.Hour)

	for _, t := range tasks {
		switch t.Status {
		case "completed":
			summary.Completed++
			if t.UpdatedAt.After(periodStart) {
				summary.CompletedPeriod++
			}
			if t.UpdatedAt.After(today) {
				summary.CompletedToday++
			}
		case "in_progress":
			summary.InProgress++
		case "pending":
			summary.Pending++
		case "blocked":
			summary.Blocked++
		}

		if t.AssignedTo != "" {
			summary.ByAssignee[t.AssignedTo]++
		}
	}

	return summary
}

// extractHighlights identifies notable achievements
func (g *StandupGenerator) extractHighlights(activities []ActivityEntry, tasks []Task) []string {
	var highlights []string

	// Count completed tasks
	completedCount := 0
	for _, t := range tasks {
		if t.Status == "completed" {
			completedCount++
		}
	}

	if completedCount > 0 {
		highlights = append(highlights, fmt.Sprintf("%d task(s) completed", completedCount))
	}

	// Count successful tool operations
	successfulOps := 0
	for _, a := range activities {
		if a.Type == ActivityToolCall && a.Success {
			successfulOps++
		}
	}

	if successfulOps > 10 {
		highlights = append(highlights, fmt.Sprintf("%d successful operations executed", successfulOps))
	}

	// Look for git commits
	commitCount := 0
	for _, a := range activities {
		if a.Type == ActivityToolCall {
			if tool, ok := a.Data["tool"].(string); ok && tool == "git_commit" {
				commitCount++
			}
		}
	}

	if commitCount > 0 {
		highlights = append(highlights, fmt.Sprintf("%d git commit(s) made", commitCount))
	}

	return highlights
}

// extractBlockers identifies current blockers
func (g *StandupGenerator) extractBlockers(activities []ActivityEntry, tasks []Task) []string {
	blockerSet := make(map[string]bool)

	// Check for blocked tasks
	for _, t := range tasks {
		if t.Status == "blocked" {
			blockerSet[fmt.Sprintf("Task '%s' is blocked", t.Title)] = true
		}
	}

	// Check for errors in activities
	for _, a := range activities {
		if a.Type == ActivityError && a.Error != "" {
			// Truncate long error messages
			errMsg := a.Error
			if len(errMsg) > 100 {
				errMsg = errMsg[:100] + "..."
			}
			blockerSet[errMsg] = true
		}

		// Check for failed tool calls
		if a.Type == ActivityToolCall && !a.Success && a.Error != "" {
			errMsg := a.Error
			if len(errMsg) > 100 {
				errMsg = errMsg[:100] + "..."
			}
			blockerSet[fmt.Sprintf("Tool error: %s", errMsg)] = true
		}
	}

	var blockers []string
	for b := range blockerSet {
		blockers = append(blockers, b)
	}

	return blockers
}

// suggestNextSteps suggests next steps based on current state
func (g *StandupGenerator) suggestNextSteps(tasks []Task) []string {
	var steps []string

	// Find high priority pending tasks
	var highPriority []string
	var inProgress []string

	for _, t := range tasks {
		if t.Status == "pending" && t.Priority == "high" {
			highPriority = append(highPriority, t.Title)
		}
		if t.Status == "in_progress" {
			inProgress = append(inProgress, t.Title)
		}
	}

	if len(highPriority) > 0 {
		steps = append(steps, fmt.Sprintf("High priority: %s", strings.Join(highPriority[:min(3, len(highPriority))], ", ")))
	}

	if len(inProgress) > 0 {
		steps = append(steps, fmt.Sprintf("Continue: %s", strings.Join(inProgress[:min(3, len(inProgress))], ", ")))
	}

	if len(steps) == 0 {
		steps = append(steps, "Review completed work and plan next sprint")
	}

	return steps
}

// FormatReport formats a standup report as a string
func (g *StandupGenerator) FormatReport(report *StandupReport, detailed bool) string {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("# Standup Report: %s\n\n", report.ProjectName))
	sb.WriteString(fmt.Sprintf("**Period:** %s (%s - %s)\n",
		report.Period,
		report.PeriodStart.Format("2006-01-02 15:04"),
		report.PeriodEnd.Format("2006-01-02 15:04")))
	sb.WriteString(fmt.Sprintf("**Generated:** %s\n\n", report.GeneratedAt.Format(time.RFC3339)))

	// Team Summary
	sb.WriteString("## Team Summary\n\n")
	sb.WriteString(fmt.Sprintf("- Active Members: %d\n", report.TeamSummary.ActiveMembers))
	sb.WriteString(fmt.Sprintf("- Total Operations: %d\n", report.TeamSummary.TotalToolCalls))
	sb.WriteString(fmt.Sprintf("- Success Rate: %.1f%%\n", report.TeamSummary.SuccessRate))
	sb.WriteString(fmt.Sprintf("- Delegations: %d\n\n", report.TeamSummary.TotalDelegations))

	// Task Summary
	sb.WriteString("## Tasks\n\n")
	sb.WriteString(fmt.Sprintf("- Total: %d\n", report.TaskSummary.TotalTasks))
	sb.WriteString(fmt.Sprintf("- Completed (this period): %d\n", report.TaskSummary.CompletedPeriod))
	sb.WriteString(fmt.Sprintf("- In Progress: %d\n", report.TaskSummary.InProgress))
	sb.WriteString(fmt.Sprintf("- Pending: %d\n", report.TaskSummary.Pending))
	if report.TaskSummary.Blocked > 0 {
		sb.WriteString(fmt.Sprintf("- Blocked: %d\n", report.TaskSummary.Blocked))
	}
	sb.WriteString("\n")

	// Highlights
	if len(report.Highlights) > 0 {
		sb.WriteString("## Highlights\n\n")
		for _, h := range report.Highlights {
			sb.WriteString(fmt.Sprintf("- %s\n", h))
		}
		sb.WriteString("\n")
	}

	// Blockers
	if len(report.Blockers) > 0 {
		sb.WriteString("## Blockers\n\n")
		for _, b := range report.Blockers {
			sb.WriteString(fmt.Sprintf("- %s\n", b))
		}
		sb.WriteString("\n")
	}

	// Next Steps
	if len(report.NextSteps) > 0 {
		sb.WriteString("## Next Steps\n\n")
		for _, s := range report.NextSteps {
			sb.WriteString(fmt.Sprintf("- %s\n", s))
		}
		sb.WriteString("\n")
	}

	// Detailed per-member breakdown
	if detailed && len(report.MemberReports) > 0 {
		sb.WriteString("## Member Details\n\n")
		for _, m := range report.MemberReports {
			sb.WriteString(fmt.Sprintf("### %s\n\n", m.Role))
			sb.WriteString(fmt.Sprintf("- Status: %s\n", m.Status))
			sb.WriteString(fmt.Sprintf("- Tool Calls: %d (success: %d, failed: %d)\n",
				m.ToolCalls, m.SuccessfulCalls, m.FailedCalls))
			sb.WriteString(fmt.Sprintf("- Tasks Completed: %d\n", m.TasksCompleted))

			if len(m.TopTools) > 0 {
				sb.WriteString("- Top Tools: ")
				toolStrs := make([]string, len(m.TopTools))
				for i, t := range m.TopTools {
					toolStrs[i] = fmt.Sprintf("%s (%d)", t.Tool, t.Count)
				}
				sb.WriteString(strings.Join(toolStrs, ", "))
				sb.WriteString("\n")
			}

			if len(m.Blockers) > 0 {
				sb.WriteString("- Blockers:\n")
				for _, b := range m.Blockers {
					sb.WriteString(fmt.Sprintf("  - %s\n", b))
				}
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
