package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/arcslash/ugudu/internal/config"
	"github.com/arcslash/ugudu/internal/daemon"
	"github.com/arcslash/ugudu/internal/logger"
	"github.com/arcslash/ugudu/internal/mcp"
	"github.com/arcslash/ugudu/internal/templates"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

var (
	socketPath string // --socket flag
	remoteAddr string // --host flag for remote daemon
)

func main() {
	// Load .env if present
	_ = godotenv.Load()

	// Load Ugudu config and apply to environment
	if cfg, err := config.Load(); err == nil {
		cfg.ApplyToEnvironment()
	}

	root := &cobra.Command{
		Use:   "ugudu",
		Short: "Ugudu - AI Team Orchestration System",
		Long: `Ugudu is an AI agent orchestration system that lets you create
and manage teams of AI agents working together.

Quick Start:
  ugudu config init                        # Set up API keys
  ugudu daemon                             # Start the daemon
  ugudu spec new dev-team                  # Create a spec (blueprint)
  ugudu team create alpha --spec dev-team  # Create team from spec
  ugudu ask alpha "Hello!"                 # Talk to your team

Specs are reusable blueprints - create multiple teams from the same spec.

Examples:
  ugudu spec new dev-team -y               # Quick create spec
  ugudu spec list                          # List available specs
  ugudu team create alpha --spec dev-team  # Create "alpha" team
  ugudu team create beta --spec dev-team   # Create "beta" from same spec
  ugudu team create gamma -t dev-team      # Use built-in template`,
	}

	// Global flags
	root.PersistentFlags().StringVar(&socketPath, "socket", "", "daemon socket path (default: auto-detect)")
	root.PersistentFlags().StringVar(&remoteAddr, "host", "", "remote daemon address (e.g., localhost:8080)")

	// Add commands
	root.AddCommand(specCmd())
	root.AddCommand(configCmd())
	root.AddCommand(daemonCmd())
	root.AddCommand(teamCmd())
	root.AddCommand(projectCmd())
	root.AddCommand(standupCmd())
	root.AddCommand(activityCmd())
	root.AddCommand(askCmd())
	root.AddCommand(statusCmd())
	root.AddCommand(providerCmd())
	root.AddCommand(initCmd())
	root.AddCommand(versionCmd())
	root.AddCommand(debugCmd())
	root.AddCommand(templatesCmd())
	root.AddCommand(conversationCmd())
	root.AddCommand(createCmd()) // Easy entry point for beginners
	root.AddCommand(mcpCmd())    // MCP server for AI assistants

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// getClient returns a daemon client
func getClient() (*daemon.Client, error) {
	if remoteAddr != "" {
		return daemon.NewRemoteClient(remoteAddr), nil
	}
	return daemon.NewClient(socketPath)
}

// requireDaemon ensures daemon is running before executing a command
func requireDaemon() (*daemon.Client, error) {
	client, err := getClient()
	if err != nil {
		return nil, fmt.Errorf("daemon not running.\n\nStart it with: ugudu daemon\n\nError: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		return nil, fmt.Errorf("daemon not responding.\n\nStart it with: ugudu daemon\n\nError: %w", err)
	}

	return client, nil
}

// ============================================================================
// Daemon Command
// ============================================================================

func daemonCmd() *cobra.Command {
	var dataDir string
	var tcpAddr string
	var foreground bool

	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Start the Ugudu daemon",
		Long: `Start the Ugudu daemon (ugudud) which runs in the background
and manages all teams and agents.

The daemon:
- Listens on a Unix socket for local CLI connections
- Optionally listens on TCP for remote/UI connections
- Keeps teams running even when CLI disconnects
- Persists state across restarts

Examples:
  ugudu daemon                    # Start daemon with web UI on :9741
  ugudu daemon --tcp :3000        # Use custom port
  ugudu daemon --data ~/.ugudu    # Custom data directory`,
		Run: func(cmd *cobra.Command, args []string) {
			if dataDir == "" {
				home, _ := os.UserHomeDir()
				dataDir = filepath.Join(home, ".ugudu", "data")
			}

			cfg := daemon.Config{
				DataDir:    dataDir,
				SocketPath: socketPath,
				TCPAddr:    tcpAddr,
				LogLevel:   "info",
			}

			d, err := daemon.New(cfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			fmt.Println("╔══════════════════════════════════════════╗")
			fmt.Println("║           U G U D U                      ║")
			fmt.Println("║   AI Team Orchestration System           ║")
			fmt.Println("╚══════════════════════════════════════════╝")
			fmt.Println()
			fmt.Printf("Daemon starting...\n")
			fmt.Printf("  Socket:  %s\n", d.GetSocketPath())
			fmt.Printf("  Web UI:  http://localhost%s\n", tcpAddr)
			fmt.Printf("  Data:    %s\n", dataDir)
			fmt.Println()
			fmt.Println("Press Ctrl+C to stop.")

			if err := d.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	home, _ := os.UserHomeDir()
	defaultDataDir := filepath.Join(home, ".ugudu", "data")

	cmd.Flags().StringVar(&dataDir, "data", defaultDataDir, "data directory")
	cmd.Flags().StringVar(&tcpAddr, "tcp", ":9741", "TCP address for HTTP/Web UI (default :9741)")
	cmd.Flags().BoolVar(&foreground, "foreground", true, "run in foreground (default)")

	return cmd
}

// ============================================================================
// Team Commands
// ============================================================================

func teamCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "team",
		Short: "Manage teams",
		Long:  "Create, start, stop, and manage AI agent teams",
	}

	cmd.AddCommand(teamCreateCmd())
	cmd.AddCommand(teamStartCmd())
	cmd.AddCommand(teamStopCmd())
	cmd.AddCommand(teamDeleteCmd())
	cmd.AddCommand(teamListCmd())
	cmd.AddCommand(teamPsCmd())

	return cmd
}

func teamCreateCmd() *cobra.Command {
	var fromSpec string
	var fromTemplate string

	cmd := &cobra.Command{
		Use:   "create <team-name>",
		Short: "Create a new team instance",
		Long: `Create a new team instance from a spec (blueprint).

A spec is a reusable blueprint - you can create multiple teams from the same spec.

Examples:
  ugudu team create alpha --spec dev-team      # Create "alpha" team from dev-team spec
  ugudu team create beta --spec dev-team       # Create "beta" team from same spec
  ugudu team create gamma --template dev-team  # Create from built-in template

List available specs with: ugudu spec list
List templates with: ugudu templates list`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			teamName := args[0]

			client, err := requireDaemon()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			var specPath string
			var specContent []byte

			if fromTemplate != "" {
				// Use embedded template
				if !templates.Exists(fromTemplate) {
					fmt.Fprintf(os.Stderr, "Template not found: %s\n", fromTemplate)
					fmt.Println("\nAvailable templates:")
					names, _ := templates.List()
					for _, n := range names {
						fmt.Printf("  - %s\n", n)
					}
					os.Exit(1)
				}
				specContent, _ = templates.Get(fromTemplate)
			} else if fromSpec != "" {
				// Use spec from ~/.ugudu/teams/
				specPath = resolveSpecPath(fromSpec)
				if _, err := os.Stat(specPath); os.IsNotExist(err) {
					fmt.Fprintf(os.Stderr, "Spec not found: %s\n", fromSpec)
					fmt.Println("\nAvailable specs:")
					listAvailableSpecs()
					os.Exit(1)
				}
				specContent, _ = os.ReadFile(specPath)
			} else {
				// No spec specified - show help
				fmt.Println("Usage: ugudu team create <team-name> --spec <spec-name>")
				fmt.Println("\nAvailable specs:")
				listAvailableSpecs()
				fmt.Println("\nBuilt-in templates:")
				names, _ := templates.List()
				for _, n := range names {
					fmt.Printf("  - %s (use: --template %s)\n", n, n)
				}
				os.Exit(1)
			}

			// Modify the spec to use the provided team name
			modifiedSpec := replaceTeamName(string(specContent), teamName)

			// Write to persistent spec file in ~/.ugudu/specs/
			specFile := filepath.Join(config.SpecsDir(), teamName+".yaml")
			if err := os.WriteFile(specFile, []byte(modifiedSpec), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing spec file: %v\n", err)
				os.Exit(1)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			result, err := client.CreateTeam(ctx, specFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating team: %v\n", err)
				os.Exit(1)
			}

			name := result["name"].(string)
			members := int(result["members"].(float64))
			fmt.Printf("Team '%s' created successfully.\n", name)
			fmt.Printf("  Spec: %s\n", fromSpec+fromTemplate)
			fmt.Printf("  Members: %d\n", members)
			fmt.Println("\nTalk to it: ugudu ask", name, "\"Hello team!\"")
		},
	}

	cmd.Flags().StringVarP(&fromSpec, "spec", "s", "", "spec name (from ~/.ugudu/specs/)")
	cmd.Flags().StringVarP(&fromTemplate, "template", "t", "", "built-in template name")

	return cmd
}

// replaceTeamName modifies the spec YAML to use the given team name
func replaceTeamName(specContent, teamName string) string {
	lines := strings.Split(specContent, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "name:") {
			// Find indentation
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			lines[i] = indent + "name: " + teamName
			break
		}
	}
	return strings.Join(lines, "\n")
}

// resolveSpecPath resolves a spec name to its file path
func resolveSpecPath(name string) string {
	// If it's already a path, use it
	if strings.Contains(name, "/") || strings.HasSuffix(name, ".yaml") {
		abs, _ := filepath.Abs(name)
		return abs
	}
	// Otherwise, look in ~/.ugudu/specs/
	return filepath.Join(config.SpecsDir(), name+".yaml")
}

// listAvailableSpecs prints available specs from ~/.ugudu/specs/
func listAvailableSpecs() {
	specsDir := config.SpecsDir()
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		fmt.Println("  (none - create one with: ugudu spec new <name>)")
		return
	}

	found := false
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
			name := strings.TrimSuffix(e.Name(), ".yaml")
			fmt.Printf("  - %s\n", name)
			found = true
		}
	}

	if !found {
		fmt.Println("  (none - create one with: ugudu spec new <name>)")
	}
}

func teamStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start [team-name]",
		Short: "Start a team",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client, err := requireDaemon()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if err := client.StartTeam(ctx, args[0]); err != nil {
				fmt.Fprintf(os.Stderr, "Error starting team: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Team '%s' started.\n", args[0])
		},
	}
}

func teamStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop [team-name]",
		Short: "Stop a team",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client, err := requireDaemon()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if err := client.StopTeam(ctx, args[0]); err != nil {
				fmt.Fprintf(os.Stderr, "Error stopping team: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Team '%s' stopped.\n", args[0])
		},
	}
}

func teamDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete [team-name]",
		Short: "Delete a team",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client, err := requireDaemon()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if err := client.DeleteTeam(ctx, args[0]); err != nil {
				fmt.Fprintf(os.Stderr, "Error deleting team: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Team '%s' deleted.\n", args[0])
		},
	}
}

func teamListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all teams",
		Run: func(cmd *cobra.Command, args []string) {
			client, err := requireDaemon()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			teams, err := client.ListTeams(ctx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			if len(teams) == 0 {
				fmt.Println("No teams found.")
				fmt.Println("\nCreate one with: ugudu team create <spec.yaml>")
				return
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tSTATUS\tMEMBERS\tDESCRIPTION")
			fmt.Fprintln(w, "────\t──────\t───────\t───────────")

			for _, t := range teams {
				name, _ := t["name"].(string)
				status, _ := t["status"].(string)
				members := int(t["member_count"].(float64))
				desc, _ := t["description"].(string)
				if len(desc) > 40 {
					desc = desc[:40] + "..."
				}
				fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", name, status, members, desc)
			}
			w.Flush()
		},
	}
}

func teamPsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ps [team-name]",
		Short: "Show team members and their status",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client, err := requireDaemon()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			members, err := client.TeamMembers(ctx, args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "Team: %s\n\n", args[0])
			fmt.Fprintln(w, "NAME\tROLE\tSTATUS\tVISIBILITY")
			fmt.Fprintln(w, "────\t────\t──────\t──────────")

			for _, m := range members {
				name, _ := m["name"].(string)
				title, _ := m["title"].(string)
				status, _ := m["status"].(string)
				visibility, _ := m["visibility"].(string)

				// Use display_name if available, otherwise format name + title
				displayName := name
				if displayName == "" || displayName == title {
					displayName = title
				} else {
					displayName = fmt.Sprintf("%s (%s)", name, title)
				}

				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", displayName, title, status, visibility)
			}
			w.Flush()
		},
	}
}

// ============================================================================
// Ask Command
// ============================================================================

func askCmd() *cobra.Command {
	var toMember string
	var timeout int
	var lowToken bool
	var minimalToken bool

	cmd := &cobra.Command{
		Use:   "ask [team-name] [message]",
		Short: "Send a message to a team",
		Long: `Send a message to a team. The message goes to the primary
client-facing member (usually a PM or lead).

Use --to to send to a specific role.
Use --low-token to reduce token consumption (shorter prompts, cheaper models).
Use --minimal-token for bare minimum token usage.`,
		Args: cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			client, err := requireDaemon()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			teamName := args[0]
			message := strings.Join(args[1:], " ")

			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
			defer cancel()

			// Start team if not running
			_ = client.StartTeam(ctx, teamName)

			// Set token mode if specified
			if minimalToken {
				_ = client.SetTokenMode(ctx, teamName, "minimal")
			} else if lowToken {
				_ = client.SetTokenMode(ctx, teamName, "low")
			}

			responses, err := client.Chat(ctx, teamName, message, toMember)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			for _, resp := range responses {
				from, _ := resp["from"].(string)
				content, _ := resp["content"].(string)
				fmt.Printf("\n%s: %s\n", from, content)
			}
		},
	}

	cmd.Flags().StringVar(&toMember, "to", "", "send to specific role")
	cmd.Flags().IntVar(&timeout, "timeout", 600, "timeout in seconds (default: 10 minutes for complex tasks)")
	cmd.Flags().BoolVar(&lowToken, "low-token", false, "use low token mode (condensed prompts, reduced context)")
	cmd.Flags().BoolVar(&minimalToken, "minimal-token", false, "use minimal token mode (bare minimum)")

	return cmd
}

// ============================================================================
// Status Command
// ============================================================================

func statusCmd() *cobra.Command {
	var outputJSON bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show overall status",
		Run: func(cmd *cobra.Command, args []string) {
			client, err := requireDaemon()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			status, err := client.Status(ctx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			if outputJSON {
				data, _ := json.MarshalIndent(status, "", "  ")
				fmt.Println(string(data))
				return
			}

			fmt.Println("╔══════════════════════════════════════════╗")
			fmt.Println("║           U G U D U                      ║")
			fmt.Println("║   AI Team Orchestration System           ║")
			fmt.Println("╚══════════════════════════════════════════╝")
			fmt.Println()
			fmt.Println("Daemon: running")
			fmt.Printf("Socket: %s\n", client.GetSocketPath())
			fmt.Println()

			// Teams
			if teams, ok := status["teams"].([]interface{}); ok {
				fmt.Printf("Teams: %d\n", len(teams))
				for _, t := range teams {
					if tm, ok := t.(map[string]interface{}); ok {
						fmt.Printf("  - %s (%v members)\n", tm["name"], tm["member_count"])
					}
				}
			}
			fmt.Println()

			// Providers
			if providers, ok := status["providers"].([]interface{}); ok {
				fmt.Printf("Providers: %d configured\n", len(providers))
				for _, p := range providers {
					if pm, ok := p.(map[string]interface{}); ok {
						fmt.Printf("  - %s\n", pm["name"])
					}
				}
			}
		},
	}

	cmd.Flags().BoolVar(&outputJSON, "json", false, "output as JSON")

	return cmd
}

// ============================================================================
// Provider Commands
// ============================================================================

func providerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provider",
		Short: "Manage AI providers",
	}

	cmd.AddCommand(providerListCmd())
	cmd.AddCommand(providerTestCmd())
	cmd.AddCommand(providerModelsCmd())

	return cmd
}

func providerListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List configured providers",
		Run: func(cmd *cobra.Command, args []string) {
			client, err := requireDaemon()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			providers, err := client.ListProviders(ctx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			if len(providers) == 0 {
				fmt.Println("No providers configured.")
				fmt.Println("\nSet environment variables to configure providers:")
				fmt.Println("  ANTHROPIC_API_KEY    - for Claude models")
				fmt.Println("  OPENAI_API_KEY       - for GPT models")
				fmt.Println("  GROQ_API_KEY         - for Groq models")
				fmt.Println("  OLLAMA_URL           - for local Ollama (default: http://localhost:11434)")
				return
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME")
			fmt.Fprintln(w, "──\t────")
			for _, p := range providers {
				id, _ := p["id"].(string)
				name, _ := p["name"].(string)
				fmt.Fprintf(w, "%s\t%s\n", id, name)
			}
			w.Flush()
		},
	}
}

func providerTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test [provider-id]",
		Short: "Test connectivity to a provider",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client, err := requireDaemon()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Testing %s...", args[0])

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if err := client.TestProvider(ctx, args[0]); err != nil {
				fmt.Printf(" FAILED\n")
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf(" OK\n")
		},
	}
}

func providerModelsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "models [provider-id]",
		Short: "List models for a provider",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client, err := requireDaemon()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			models, err := client.ProviderModels(ctx, args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			if len(models) == 0 {
				fmt.Println("No models found.")
				return
			}

			fmt.Println("Models:")
			for _, m := range models {
				fmt.Printf("  - %s\n", m)
			}
		},
	}
}

// ============================================================================
// Templates Command
// ============================================================================

func templatesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "templates",
		Short: "Manage team templates",
	}

	cmd.AddCommand(&cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List available templates",
		Run: func(cmd *cobra.Command, args []string) {
			names, err := templates.List()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			fmt.Println("Available templates:")
			for _, n := range names {
				fmt.Printf("  - %s\n", n)
			}
			fmt.Println("\nUse with: ugudu team create --template <name>")
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "show [name]",
		Short: "Show template content",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			content, err := templates.Get(args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Template not found: %s\n", args[0])
				os.Exit(1)
			}
			fmt.Println(string(content))
		},
	})

	return cmd
}

// ============================================================================
// Init Command
// ============================================================================

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize Ugudu with example templates",
		Run: func(cmd *cobra.Command, args []string) {
			// Create teams directory
			teamsDir := "teams"
			if err := os.MkdirAll(teamsDir, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "Error creating teams directory: %v\n", err)
				os.Exit(1)
			}

			// Export embedded templates
			names, _ := templates.List()
			for _, name := range names {
				path := filepath.Join(teamsDir, name+".yaml")
				if _, err := os.Stat(path); err == nil {
					fmt.Printf("  Skipping %s (already exists)\n", path)
					continue
				}

				content, _ := templates.Get(name)
				if err := os.WriteFile(path, content, 0644); err != nil {
					fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", path, err)
					continue
				}
				fmt.Printf("  Created %s\n", path)
			}

			fmt.Println("\nDone! Example team templates created in ./teams/")
			fmt.Println("\nNext steps:")
			fmt.Println("  1. Start the daemon: ugudu daemon --tcp :8080")
			fmt.Println("  2. Create a team: ugudu team create teams/dev-team.yaml")
			fmt.Println("  3. Start it: ugudu team start dev-team")
			fmt.Println("  4. Ask it something: ugudu ask dev-team \"Hello team!\"")
		},
	}
}

// ============================================================================
// Create Command (Easy entry point)
// ============================================================================

func createCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create [description]",
		Short: "Create a new AI team (easiest way to start)",
		Long: `The easiest way to create a new AI team.

Just describe what you want to build - no YAML knowledge required!
An AI assistant will interview you and design the perfect team.

Examples:
  ugudu create                           # Start with guided interview
  ugudu create "mobile fitness app"      # Start with your idea
  ugudu create "e-commerce website"      # Jump right in

This is equivalent to 'ugudu spec ai' but easier to discover.

For more control, use:
  ugudu spec new      # Step-by-step wizard
  ugudu spec ai       # AI-powered (same as this)`,
		Run: func(cmd *cobra.Command, args []string) {
			// Delegate to spec ai command
			specAI := specAICmd()
			specAI.Run(specAI, args)
		},
	}
}

// ============================================================================
// Version Command
// ============================================================================

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Ugudu v0.1.0")
			fmt.Println("AI Team Orchestration System")
			fmt.Println()
			fmt.Println("Architecture: daemon + client (like Docker)")
		},
	}
}

// ============================================================================
// MCP Command (Model Context Protocol)
// ============================================================================

func mcpCmd() *cobra.Command {
	var logLevel string

	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Start MCP server for AI assistant integration",
		Long: `Start the Model Context Protocol (MCP) server.

MCP allows AI assistants like Claude Desktop to interact with Ugudu directly.
The server runs on stdin/stdout and exposes Ugudu's functionality as tools.

Available MCP tools:
  ugudu_list_teams      List all teams
  ugudu_create_team     Create a team from a spec
  ugudu_ask             Send message to a team
  ugudu_team_status     Get team member status
  ugudu_list_specs      List available specs
  ugudu_daemon_status   Check daemon status
  ugudu_start_team      Start a stopped team
  ugudu_stop_team       Stop a running team
  ugudu_delete_team     Delete a team

Setup for Claude Desktop:

1. Add to ~/.claude/claude_desktop_config.json:

{
  "mcpServers": {
    "ugudu": {
      "command": "ugudu",
      "args": ["mcp"]
    }
  }
}

2. Restart Claude Desktop

Now Claude can manage your AI teams directly!

Note: The Ugudu daemon must be running for most tools to work.
Start it with: ugudu daemon`,
		Run: func(cmd *cobra.Command, args []string) {
			log := logger.New(logLevel)

			server, err := mcp.NewServer(log)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating MCP server: %v\n", err)
				os.Exit(1)
			}

			if err := server.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringVar(&logLevel, "log-level", "warn", "log level (debug, info, warn, error)")

	return cmd
}
