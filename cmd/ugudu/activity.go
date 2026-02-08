package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/arcslash/ugudu/internal/workspace"
	"github.com/spf13/cobra"
)

func activityCmd() *cobra.Command {
	var role string
	var activityType string
	var taskID string
	var since string
	var limit int
	var outputJSON bool

	cmd := &cobra.Command{
		Use:   "activity [project-name]",
		Short: "View activity logs for a project",
		Long: `View and query activity logs for a project.

Activity types: tool_call, delegation, task_update, message, progress, error

Examples:
  ugudu activity my-project                    # Recent activity
  ugudu activity my-project --role engineer    # Filter by role
  ugudu activity my-project --type tool_call   # Filter by type
  ugudu activity my-project --since 1h         # Last hour
  ugudu activity my-project --limit 50         # Last 50 entries`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			projectName := args[0]

			// Load workspace
			ws, err := workspace.New(projectName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			// Build query options
			opts := workspace.QueryOptions{
				Limit: limit,
			}

			// Parse since duration
			if since != "" {
				duration, err := parseDuration(since)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Invalid duration: %v\n", err)
					os.Exit(1)
				}
				opts.Since = time.Now().Add(-duration)
			}

			// Parse activity type filter
			if activityType != "" {
				opts.Types = []workspace.ActivityType{workspace.ActivityType(activityType)}
			}

			// Task ID filter
			if taskID != "" {
				opts.TaskID = taskID
			}

			// Query activity
			var entries []workspace.ActivityEntry
			if role != "" {
				// Query single role log
				logPath := ws.ActivityPath(role)
				entries, err = workspace.QueryActivityLog(logPath, opts)
			} else {
				// Query all activity
				entries, err = workspace.QueryProjectActivity(ws, opts)
			}

			if err != nil {
				fmt.Fprintf(os.Stderr, "Error querying activity: %v\n", err)
				os.Exit(1)
			}

			if len(entries) == 0 {
				fmt.Println("No activity found.")
				return
			}

			// Output
			if outputJSON {
				data, _ := json.MarshalIndent(entries, "", "  ")
				fmt.Println(string(data))
			} else {
				printActivityTable(entries)
			}
		},
	}

	cmd.Flags().StringVarP(&role, "role", "r", "", "filter by role (engineer, pm, qa, ba)")
	cmd.Flags().StringVarP(&activityType, "type", "t", "", "filter by activity type")
	cmd.Flags().StringVar(&taskID, "task", "", "filter by task ID")
	cmd.Flags().StringVarP(&since, "since", "s", "", "show activity since duration (e.g., 1h, 30m, 1d)")
	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "maximum number of entries")
	cmd.Flags().BoolVar(&outputJSON, "json", false, "output as JSON")

	return cmd
}

func printActivityTable(entries []workspace.ActivityEntry) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TIMESTAMP\tROLE\tTYPE\tDETAILS\tSTATUS")
	fmt.Fprintln(w, "─────────\t────\t────\t───────\t──────")

	for _, e := range entries {
		timestamp := e.Timestamp.Format("15:04:05")
		status := "OK"
		if !e.Success {
			status = "FAIL"
		}

		details := formatActivityDetails(e)
		if len(details) > 50 {
			details = details[:47] + "..."
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			timestamp, e.AgentRole, e.Type, details, status)
	}
	w.Flush()
}

func formatActivityDetails(e workspace.ActivityEntry) string {
	switch e.Type {
	case workspace.ActivityToolCall:
		if tool, ok := e.Data["tool"].(string); ok {
			return fmt.Sprintf("tool: %s", tool)
		}
	case workspace.ActivityDelegation:
		if toRole, ok := e.Data["to_role"].(string); ok {
			return fmt.Sprintf("to: %s", toRole)
		}
	case workspace.ActivityTaskUpdate:
		if newStatus, ok := e.Data["new_status"].(string); ok {
			return fmt.Sprintf("status: %s", newStatus)
		}
	case workspace.ActivityMessage:
		if dir, ok := e.Data["direction"].(string); ok {
			if toFrom, ok := e.Data["to_from"].(string); ok {
				return fmt.Sprintf("%s %s", dir, toFrom)
			}
		}
	case workspace.ActivityProgress:
		if status, ok := e.Data["status"].(string); ok {
			return fmt.Sprintf("progress: %s", status)
		}
	case workspace.ActivityError:
		if e.Error != "" {
			return e.Error
		}
	}

	// Default: show first data key
	for k, v := range e.Data {
		return fmt.Sprintf("%s: %v", k, v)
	}
	return ""
}

func parseDuration(s string) (time.Duration, error) {
	// Support "d" suffix for days
	if strings.HasSuffix(s, "d") {
		s = strings.TrimSuffix(s, "d")
		var days int
		if _, err := fmt.Sscanf(s, "%d", &days); err != nil {
			return 0, fmt.Errorf("invalid day format: %s", s)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}

	// Use standard Go duration parsing
	return time.ParseDuration(s)
}
