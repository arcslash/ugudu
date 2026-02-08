package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/arcslash/ugudu/internal/workspace"
	"github.com/spf13/cobra"
)

func standupCmd() *cobra.Command {
	var period string
	var detailed bool
	var outputJSON bool

	cmd := &cobra.Command{
		Use:   "standup [project-name]",
		Short: "Generate a standup report for a project",
		Long: `Generate a standup report showing team activity, task progress,
highlights, and blockers.

Examples:
  ugudu standup my-project                 # Daily standup report
  ugudu standup my-project --period weekly # Weekly summary
  ugudu standup my-project --detailed      # Include per-member breakdown
  ugudu standup my-project --json          # Output as JSON`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			projectName := args[0]

			// Load workspace
			ws, err := workspace.New(projectName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			// Determine period
			var standupPeriod workspace.StandupPeriod
			switch period {
			case "daily":
				standupPeriod = workspace.PeriodDaily
			case "weekly":
				standupPeriod = workspace.PeriodWeekly
			default:
				standupPeriod = workspace.PeriodDaily
			}

			// Generate report
			generator := workspace.NewStandupGenerator(ws)
			report, err := generator.Generate(standupPeriod)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error generating report: %v\n", err)
				os.Exit(1)
			}

			// Output
			if outputJSON {
				data, _ := json.MarshalIndent(report, "", "  ")
				fmt.Println(string(data))
			} else {
				formatted := generator.FormatReport(report, detailed)
				fmt.Print(formatted)
			}
		},
	}

	cmd.Flags().StringVarP(&period, "period", "p", "daily", "report period (daily, weekly)")
	cmd.Flags().BoolVarP(&detailed, "detailed", "d", false, "include per-member breakdown")
	cmd.Flags().BoolVar(&outputJSON, "json", false, "output as JSON")

	return cmd
}
