package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/arcslash/ugudu/internal/workspace"
	"github.com/spf13/cobra"
)

func projectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage project workspaces",
		Long: `Manage isolated project workspaces for teams.

Projects provide:
- Isolated sandboxes for each team member
- Activity tracking and reporting
- Task management
- Artifact storage

Examples:
  ugudu project create my-app --source ~/code/my-app --team dev-team
  ugudu project list
  ugudu project show my-app
  ugudu project delete my-app`,
	}

	cmd.AddCommand(projectCreateCmd())
	cmd.AddCommand(projectListCmd())
	cmd.AddCommand(projectShowCmd())
	cmd.AddCommand(projectDeleteCmd())

	return cmd
}

func projectCreateCmd() *cobra.Command {
	var sourcePath string
	var team string
	var sharedPaths []string

	cmd := &cobra.Command{
		Use:   "create [project-name]",
		Short: "Create a new project workspace",
		Long: `Create a new project workspace with isolated sandboxes.

The source path points to the actual source code location.
Agents can read from source but write to their own sandbox.

Examples:
  ugudu project create my-app --source ~/code/my-app --team dev-team
  ugudu project create api-project --source ./api --team backend-team --shared ~/docs`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			projectName := args[0]

			// Validate required flags
			if sourcePath == "" {
				fmt.Fprintln(os.Stderr, "Error: --source is required")
				os.Exit(1)
			}
			if team == "" {
				fmt.Fprintln(os.Stderr, "Error: --team is required")
				os.Exit(1)
			}

			// Create workspace
			ws, err := workspace.Init(projectName, sourcePath, team)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating project: %v\n", err)
				os.Exit(1)
			}

			// Add shared paths if specified
			if len(sharedPaths) > 0 {
				ws.Config.Source.SharedPaths = sharedPaths
				configPath := ws.Path + "/project.yaml"
				if err := ws.Config.Save(configPath); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to save shared paths: %v\n", err)
				}
			}

			fmt.Printf("Project '%s' created successfully.\n", projectName)
			fmt.Printf("  Path:   %s\n", ws.Path)
			fmt.Printf("  Source: %s\n", ws.Config.Source.Path)
			fmt.Printf("  Team:   %s\n", team)
			fmt.Println()
			fmt.Println("Start working with:")
			fmt.Printf("  ugudu team start %s --project %s\n", team, projectName)
		},
	}

	cmd.Flags().StringVarP(&sourcePath, "source", "s", "", "path to source code (required)")
	cmd.Flags().StringVarP(&team, "team", "t", "", "team template to use (required)")
	cmd.Flags().StringSliceVar(&sharedPaths, "shared", nil, "additional readable paths")
	cmd.MarkFlagRequired("source")
	cmd.MarkFlagRequired("team")

	return cmd
}

func projectListCmd() *cobra.Command {
	var outputJSON bool

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all projects",
		Run: func(cmd *cobra.Command, args []string) {
			projects, err := workspace.ListProjects()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error listing projects: %v\n", err)
				os.Exit(1)
			}

			if len(projects) == 0 {
				fmt.Println("No projects found.")
				fmt.Println("\nCreate one with: ugudu project create <name> --source <path> --team <team>")
				return
			}

			if outputJSON {
				data, _ := json.MarshalIndent(projects, "", "  ")
				fmt.Println(string(data))
				return
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tSOURCE PATH\tTEAM\tCREATED")
			fmt.Fprintln(w, "────\t───────────\t────\t───────")

			for _, p := range projects {
				created := p.CreatedAt.Format("2006-01-02 15:04")
				sourcePath := p.SourcePath
				if len(sourcePath) > 40 {
					sourcePath = "..." + sourcePath[len(sourcePath)-37:]
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.Name, sourcePath, p.Team, created)
			}
			w.Flush()
		},
	}

	cmd.Flags().BoolVar(&outputJSON, "json", false, "output as JSON")

	return cmd
}

func projectShowCmd() *cobra.Command {
	var outputJSON bool

	cmd := &cobra.Command{
		Use:   "show [project-name]",
		Short: "Show project details",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			projectName := args[0]

			ws, err := workspace.New(projectName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			if outputJSON {
				data, _ := json.MarshalIndent(ws.Config, "", "  ")
				fmt.Println(string(data))
				return
			}

			fmt.Printf("Project: %s\n\n", ws.Config.Metadata.Name)

			fmt.Println("Configuration:")
			fmt.Printf("  API Version: %s\n", ws.Config.APIVersion)
			fmt.Printf("  Kind:        %s\n", ws.Config.Kind)
			fmt.Printf("  Created:     %s\n", ws.Config.Metadata.CreatedAt.Format(time.RFC3339))
			fmt.Println()

			fmt.Println("Source:")
			fmt.Printf("  Path: %s\n", ws.Config.Source.Path)
			if len(ws.Config.Source.SharedPaths) > 0 {
				fmt.Println("  Shared Paths:")
				for _, p := range ws.Config.Source.SharedPaths {
					fmt.Printf("    - %s\n", p)
				}
			}
			fmt.Println()

			fmt.Println("Team:")
			fmt.Printf("  Template: %s\n", ws.Config.Team)
			fmt.Println()

			fmt.Println("Workspace:")
			fmt.Printf("  Isolation:          %s\n", ws.Config.Workspace.Isolation)
			fmt.Printf("  Artifact Retention: %s\n", ws.Config.Workspace.ArtifactRetention)
			fmt.Println()

			fmt.Println("Paths:")
			fmt.Printf("  Project:   %s\n", ws.Path)
			fmt.Printf("  Tasks:     %s\n", ws.TasksPath())
			fmt.Printf("  Artifacts: %s\n", ws.ArtifactPath(""))
		},
	}

	cmd.Flags().BoolVar(&outputJSON, "json", false, "output as JSON")

	return cmd
}

func projectDeleteCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete [project-name]",
		Short: "Delete a project workspace",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			projectName := args[0]

			// Confirm deletion
			if !force {
				fmt.Printf("Delete project '%s'? This cannot be undone.\n", projectName)
				fmt.Print("Type 'yes' to confirm: ")
				var confirm string
				fmt.Scanln(&confirm)
				if confirm != "yes" {
					fmt.Println("Cancelled.")
					return
				}
			}

			if err := workspace.Delete(projectName); err != nil {
				fmt.Fprintf(os.Stderr, "Error deleting project: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Project '%s' deleted.\n", projectName)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation")

	return cmd
}

// projectAskCmd extends the ask command to support --project flag
func enhanceAskCmdWithProject(askCmd *cobra.Command) {
	var project string
	askCmd.Flags().StringVar(&project, "project", "", "project workspace to use")

	// The original Run function will be wrapped to handle project context
	// This is handled in the team start command which now accepts --project
}

// Helper for commands that need to check if daemon supports projects
func projectsSupported(client interface{}) bool {
	// Check if the daemon supports project operations
	// This will be implemented when we add the API endpoints
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = ctx
	return true
}
