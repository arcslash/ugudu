package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"text/template"
	"time"

	"github.com/arcslash/ugudu/internal/config"
	"github.com/arcslash/ugudu/internal/provider"
	"github.com/arcslash/ugudu/internal/specgen"
	"github.com/spf13/cobra"
)

func specCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spec",
		Short: "Manage team specifications",
		Long: `Create and manage team specification files.

Specs are saved to ~/.ugudu/specs/ and can be used with 'ugudu team create'.

Examples:
  ugudu spec new                 # Interactive wizard
  ugudu spec new my-team -y      # Quick create with defaults
  ugudu spec ai                  # AI-powered spec creation (conversational)
  ugudu spec ai "mobile app"     # Start with your idea
  ugudu spec list                # List available specs
  ugudu spec show my-team        # Show spec contents
  ugudu spec delete my-team      # Delete a spec`,
	}

	cmd.AddCommand(specNewCmd())
	cmd.AddCommand(specAICmd())
	cmd.AddCommand(specListCmd())
	cmd.AddCommand(specShowCmd())
	cmd.AddCommand(specDeleteCmd())

	return cmd
}

func specNewCmd() *cobra.Command {
	var useDefaults bool

	cmd := &cobra.Command{
		Use:   "new [spec-name]",
		Short: "Create a new team specification",
		Long: `Create a new team specification with an interactive wizard.

Similar to 'npm init', this command guides you through creating
a team configuration step by step.

Examples:
  ugudu spec new                 # Interactive mode
  ugudu spec new my-team         # Start with a name
  ugudu spec new my-team -y      # Use defaults`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			reader := bufio.NewReader(os.Stdin)
			teamCfg := &TeamConfig{
				APIVersion: "ugudu/v1",
				Kind:       "Team",
			}

			fmt.Println("╔══════════════════════════════════════════╗")
			fmt.Println("║       U G U D U   S P E C                ║")
			fmt.Println("║      Team Specification Wizard           ║")
			fmt.Println("╚══════════════════════════════════════════╝")
			fmt.Println()
			fmt.Println("This utility will walk you through creating a team spec.")
			fmt.Println("Press ^C at any time to quit.")
			fmt.Println()

			// Team name
			if len(args) > 0 {
				teamCfg.Name = args[0]
			} else {
				teamCfg.Name = prompt(reader, "Spec name", "my-team")
			}

			// Check if spec already exists
			specPath := filepath.Join(config.SpecsDir(), teamCfg.Name+".yaml")
			if _, err := os.Stat(specPath); err == nil {
				fmt.Printf("Spec '%s' already exists at %s\n", teamCfg.Name, specPath)
				overwrite := prompt(reader, "Overwrite? [y/N]", "n")
				if strings.ToLower(overwrite) != "y" {
					fmt.Println("Aborted.")
					return
				}
			}

			if useDefaults {
				// Use sensible defaults
				teamCfg.Description = "AI agent team"
				teamCfg.TeamType = "custom"
				teamCfg.Roles = []RoleConfig{
					{
						ID:         "lead",
						Title:      "Team Lead",
						Visibility: "client",
						Provider:   "anthropic",
						Model:      "claude-sonnet-4-20250514",
						Persona:    "You are the team lead. Coordinate work and communicate with clients.",
					},
					{
						ID:         "worker",
						Title:      "Worker",
						Visibility: "internal",
						Provider:   "anthropic",
						Model:      "claude-sonnet-4-20250514",
						Persona:    "You are a worker. Complete tasks assigned by the lead.",
						ReportsTo:  "lead",
					},
				}
				teamCfg.ClientFacing = []string{"lead"}
			} else {
				// Interactive mode
				teamCfg.Description = prompt(reader, "Description", "AI agent team")

				// Team type
				fmt.Println()
				fmt.Println("Team type:")
				fmt.Println("  1) dev        - Software development (PM, BA, Engineers, QA)")
				fmt.Println("  2) trading    - Trading/finance (Analyst, Risk, Executor)")
				fmt.Println("  3) research   - Research/analysis (Researchers, Writer)")
				fmt.Println("  4) support    - Customer support (Triage, Specialists)")
				fmt.Println("  5) custom     - Build from scratch")
				fmt.Println()

				typeChoice := prompt(reader, "Choose type [1-5]", "5")
				teamCfg.TeamType = parseTeamType(typeChoice)

				if teamCfg.TeamType == "custom" {
					teamCfg.Roles = buildCustomRoles(reader)
				} else {
					teamCfg.Roles = getPresetRoles(teamCfg.TeamType)
				}

				// Determine client-facing roles
				teamCfg.ClientFacing = []string{}
				for _, role := range teamCfg.Roles {
					if role.Visibility == "client" {
						teamCfg.ClientFacing = append(teamCfg.ClientFacing, role.ID)
					}
				}

				// Provider configuration
				fmt.Println()
				fmt.Println("Default AI provider:")
				fmt.Println("  1) anthropic  - Claude models (recommended)")
				fmt.Println("  2) openai     - GPT models")
				fmt.Println("  3) ollama     - Local models")
				fmt.Println("  4) groq       - Fast inference")
				fmt.Println()

				providerChoice := prompt(reader, "Choose provider [1-4]", "1")
				defaultProvider := parseProvider(providerChoice)
				defaultModel := getDefaultModel(defaultProvider)

				// Apply provider to all roles
				for i := range teamCfg.Roles {
					if teamCfg.Roles[i].Provider == "" {
						teamCfg.Roles[i].Provider = defaultProvider
						teamCfg.Roles[i].Model = defaultModel
					}
				}
			}

			// Generate the YAML
			yaml := generateTeamYAML(teamCfg)

			// Show preview
			fmt.Println()
			fmt.Println("─────────────────────────────────────────────")
			fmt.Println("Preview:")
			fmt.Println("─────────────────────────────────────────────")
			fmt.Println(yaml)
			fmt.Println("─────────────────────────────────────────────")
			fmt.Println()

			if !useDefaults {
				confirm := prompt(reader, fmt.Sprintf("Save to %s? [Y/n]", specPath), "Y")
				if strings.ToLower(confirm) == "n" {
					fmt.Println("Aborted.")
					return
				}
			}

			// Create directory if needed
			if err := os.MkdirAll(config.SpecsDir(), 0755); err != nil {
				fmt.Fprintf(os.Stderr, "Error creating directory: %v\n", err)
				os.Exit(1)
			}

			// Write file
			if err := os.WriteFile(specPath, []byte(yaml), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("\nSpec '%s' created successfully!\n", teamCfg.Name)
			fmt.Println()
			fmt.Println("Next steps:")
			fmt.Println("  1. Start the daemon:  ugudu daemon")
			fmt.Println("  2. Create the team:   ugudu team create", teamCfg.Name)
			fmt.Println("  3. Talk to it:        ugudu ask", teamCfg.Name, "\"Hello team!\"")
		},
	}

	cmd.Flags().BoolVarP(&useDefaults, "yes", "y", false, "use defaults (non-interactive)")

	return cmd
}

func specAICmd() *cobra.Command {
	var specName string
	var providerID string
	var model string

	cmd := &cobra.Command{
		Use:   "ai [description]",
		Short: "Create a spec using AI (conversational)",
		Long: `Create a team specification through a conversational AI interview.

Just describe what you want to build and the AI will:
- Ask clarifying questions about your project
- Suggest an appropriate team structure
- Generate a complete specification

This is perfect if you're new to Ugudu or unsure what team structure you need.

Examples:
  ugudu spec ai                              # Start a conversation
  ugudu spec ai "mobile app for fitness"     # Start with your idea
  ugudu spec ai "e-commerce site" --name shop-team`,
		Run: func(cmd *cobra.Command, args []string) {
			reader := bufio.NewReader(os.Stdin)

			fmt.Println("╔══════════════════════════════════════════╗")
			fmt.Println("║       U G U D U   S P E C   A I          ║")
			fmt.Println("║     AI-Powered Team Design               ║")
			fmt.Println("╚══════════════════════════════════════════╝")
			fmt.Println()

			// Load config to get API keys
			cfg, err := config.Load()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
				os.Exit(1)
			}
			cfg.ApplyToEnvironment()

			// Get provider
			var llmProvider provider.Provider
			apiKey := os.Getenv("ANTHROPIC_API_KEY")
			if apiKey == "" {
				apiKey = os.Getenv("OPENAI_API_KEY")
				if apiKey == "" {
					fmt.Fprintln(os.Stderr, "Error: No API key configured.")
					fmt.Fprintln(os.Stderr, "\nRun 'ugudu config init' to set up your API keys.")
					os.Exit(1)
				}
				llmProvider = provider.NewOpenAI(apiKey, "")
				if model == "" {
					model = "gpt-4o"
				}
				providerID = "openai"
			} else {
				llmProvider = provider.NewAnthropic(apiKey, "")
				if model == "" {
					model = "claude-sonnet-4-20250514"
				}
				providerID = "anthropic"
			}

			generator := specgen.NewGenerator(llmProvider, model)

			// Initial input
			initialInput := ""
			if len(args) > 0 {
				initialInput = strings.Join(args, " ")
				fmt.Printf("You: %s\n\n", initialInput)
			}

			fmt.Println("Tell me about what you want to build, and I'll help you")
			fmt.Println("design the perfect AI team. Type 'done' when ready to generate,")
			fmt.Println("or 'quit' to cancel.")
			fmt.Println()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			// Start conversation
			state, response, err := generator.StartConversation(ctx, initialInput)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("🤖 Ugudu: %s\n\n", response)

			// Continue conversation until complete
			for !state.IsComplete {
				fmt.Print("You: ")
				input, _ := reader.ReadString('\n')
				input = strings.TrimSpace(input)
				fmt.Println()

				if input == "" {
					continue
				}

				if strings.ToLower(input) == "quit" || strings.ToLower(input) == "exit" {
					fmt.Println("Cancelled.")
					return
				}

				if strings.ToLower(input) == "done" || strings.ToLower(input) == "generate" {
					fmt.Println("🤖 Generating your team specification...")
					fmt.Println()

					spec, err := generator.ForceGenerate(ctx, state)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error generating spec: %v\n", err)
						os.Exit(1)
					}

					showAndSaveSpec(reader, spec, specName, providerID, model)
					return
				}

				response, err = generator.ContinueConversation(ctx, state, input)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}

				fmt.Printf("🤖 Ugudu: %s\n\n", response)
			}

			// Conversation complete, extract and save spec
			spec, err := generator.GetGeneratedSpec(state)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			showAndSaveSpec(reader, spec, specName, providerID, model)
		},
	}

	cmd.Flags().StringVarP(&specName, "name", "n", "", "spec name (default: derived from project)")
	cmd.Flags().StringVar(&providerID, "provider", "", "AI provider (anthropic, openai)")
	cmd.Flags().StringVar(&model, "model", "", "model to use")

	return cmd
}

func showAndSaveSpec(reader *bufio.Reader, spec *specgen.TeamSpec, specName, providerID, model string) {
	// Use provided name or spec's name
	name := specName
	if name == "" {
		name = spec.Name
	}
	if name == "" {
		name = prompt(reader, "Spec name", "my-team")
	}
	spec.Name = name

	// Generate YAML
	yaml := spec.ToYAML(providerID, model)

	fmt.Println()
	fmt.Println("─────────────────────────────────────────────")
	fmt.Println("Generated Specification:")
	fmt.Println("─────────────────────────────────────────────")
	fmt.Println(yaml)
	fmt.Println("─────────────────────────────────────────────")
	fmt.Println()

	specPath := filepath.Join(config.SpecsDir(), name+".yaml")

	// Check if exists
	if _, err := os.Stat(specPath); err == nil {
		overwrite := prompt(reader, fmt.Sprintf("Spec '%s' exists. Overwrite? [y/N]", name), "n")
		if strings.ToLower(overwrite) != "y" {
			// Ask for new name
			name = prompt(reader, "New spec name", name+"-new")
			spec.Name = name
			specPath = filepath.Join(config.SpecsDir(), name+".yaml")
			yaml = spec.ToYAML(providerID, model)
		}
	}

	confirm := prompt(reader, fmt.Sprintf("Save to %s? [Y/n]", specPath), "Y")
	if strings.ToLower(confirm) == "n" {
		fmt.Println("Not saved. You can copy the YAML above manually.")
		return
	}

	// Create directory if needed
	if err := os.MkdirAll(config.SpecsDir(), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating directory: %v\n", err)
		os.Exit(1)
	}

	// Write file
	if err := os.WriteFile(specPath, []byte(yaml), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✅ Spec '%s' created successfully!\n", name)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Start the daemon:  ugudu daemon")
	fmt.Println("  2. Create the team:   ugudu team create", name, "--spec", name)
	fmt.Println("  3. Talk to it:        ugudu ask", name, "\"Hello team!\"")
}

func specListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List available team specifications",
		Run: func(cmd *cobra.Command, args []string) {
			specsDir := config.SpecsDir()

			entries, err := os.ReadDir(specsDir)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Println("No specs found.")
					fmt.Println("\nCreate one with: ugudu spec new <name>")
					return
				}
				fmt.Fprintf(os.Stderr, "Error reading specs: %v\n", err)
				os.Exit(1)
			}

			var specs []string
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
					specs = append(specs, strings.TrimSuffix(e.Name(), ".yaml"))
				}
			}

			if len(specs) == 0 {
				fmt.Println("No specs found.")
				fmt.Println("\nCreate one with: ugudu spec new <name>")
				return
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tPATH")
			fmt.Fprintln(w, "────\t────")

			for _, name := range specs {
				path := filepath.Join(specsDir, name+".yaml")
				fmt.Fprintf(w, "%s\t%s\n", name, path)
			}
			w.Flush()

			fmt.Println()
			fmt.Println("Use 'ugudu team create <name>' to create a team from a spec.")
		},
	}
}

func specShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show [spec-name]",
		Short: "Show spec contents",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			specPath := resolveSpecPath(args[0])

			content, err := os.ReadFile(specPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Spec not found: %s\n", args[0])
				os.Exit(1)
			}

			fmt.Println(string(content))
		},
	}
}

func specDeleteCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete [spec-name]",
		Short: "Delete a spec",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			specPath := resolveSpecPath(args[0])

			if _, err := os.Stat(specPath); os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "Spec not found: %s\n", args[0])
				os.Exit(1)
			}

			if !force {
				reader := bufio.NewReader(os.Stdin)
				confirm := prompt(reader, fmt.Sprintf("Delete spec '%s'? [y/N]", args[0]), "n")
				if strings.ToLower(confirm) != "y" {
					fmt.Println("Aborted.")
					return
				}
			}

			if err := os.Remove(specPath); err != nil {
				fmt.Fprintf(os.Stderr, "Error deleting spec: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Spec '%s' deleted.\n", args[0])
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation")

	return cmd
}

// TeamConfig holds the configuration for generating a team
type TeamConfig struct {
	APIVersion   string
	Kind         string
	Name         string
	Description  string
	TeamType     string
	ClientFacing []string
	Roles        []RoleConfig
}

// RoleConfig holds configuration for a single role
type RoleConfig struct {
	ID          string
	Title       string
	Name        string   // Personal name for single member
	Names       []string // Names for multiple members
	Visibility  string
	Provider    string
	Model       string
	Persona     string
	CanDelegate []string
	ReportsTo   string
	Count       int
}

func prompt(reader *bufio.Reader, question, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("%s (%s): ", question, defaultVal)
	} else {
		fmt.Printf("%s: ", question)
	}

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return defaultVal
	}
	return input
}

func parseTeamType(choice string) string {
	switch choice {
	case "1":
		return "dev"
	case "2":
		return "trading"
	case "3":
		return "research"
	case "4":
		return "support"
	default:
		return "custom"
	}
}

func parseProvider(choice string) string {
	switch choice {
	case "1":
		return "anthropic"
	case "2":
		return "openai"
	case "3":
		return "ollama"
	case "4":
		return "groq"
	default:
		return "anthropic"
	}
}

func getDefaultModel(provider string) string {
	switch provider {
	case "anthropic":
		return "claude-sonnet-4-20250514"
	case "openai":
		return "gpt-4o"
	case "ollama":
		return "llama3.2"
	case "groq":
		return "llama-3.1-70b-versatile"
	default:
		return "claude-sonnet-4-20250514"
	}
}

func getPresetRoles(teamType string) []RoleConfig {
	switch teamType {
	case "dev":
		return []RoleConfig{
			{
				ID:          "pm",
				Title:       "Product Manager",
				Name:        "Sarah",
				Visibility:  "client",
				Persona:     "You are Sarah, the Product Manager. Coordinate the team, gather requirements, break down tasks, and keep clients informed of progress.",
				CanDelegate: []string{"ba", "engineer", "qa"},
			},
			{
				ID:          "ba",
				Title:       "Business Analyst",
				Name:        "Michael",
				Visibility:  "client",
				Persona:     "You are Michael, the Business Analyst. Clarify requirements, document specifications, and translate business needs to technical specs.",
				ReportsTo:   "pm",
				CanDelegate: []string{"engineer"},
			},
			{
				ID:         "engineer",
				Title:      "Software Engineer",
				Names:      []string{"Alex", "Jordan"},
				Visibility: "internal",
				Persona:    "You are a Software Engineer. Implement features, write clean code, and follow best practices. Report progress to Sarah (PM).",
				ReportsTo:  "pm",
				Count:      2,
			},
			{
				ID:         "qa",
				Title:      "QA Engineer",
				Name:       "Taylor",
				Visibility: "internal",
				Persona:    "You are Taylor, the QA Engineer. Review code, identify bugs, design test cases, and verify implementations meet requirements.",
				ReportsTo:  "pm",
			},
		}

	case "trading":
		return []RoleConfig{
			{
				ID:          "lead",
				Title:       "Trading Lead",
				Name:        "Marcus",
				Visibility:  "client",
				Persona:     "You are Marcus, the Trading Lead. Receive trading requests, coordinate analysis, risk checks, and execution. Report results clearly.",
				CanDelegate: []string{"analyst", "risk", "executor"},
			},
			{
				ID:         "analyst",
				Title:      "Market Analyst",
				Name:       "Elena",
				Visibility: "internal",
				Persona:    "You are Elena, the Market Analyst. Analyze market conditions, identify opportunities, and provide research-backed recommendations.",
				ReportsTo:  "lead",
			},
			{
				ID:         "risk",
				Title:      "Risk Manager",
				Name:       "David",
				Visibility: "internal",
				Persona:    "You are David, the Risk Manager. Evaluate trade risk, check portfolio exposure, and approve or reject trades based on risk criteria.",
				ReportsTo:  "lead",
			},
			{
				ID:         "executor",
				Title:      "Trade Executor",
				Name:       "Nina",
				Visibility: "internal",
				Persona:    "You are Nina, the Trade Executor. Execute approved trades, monitor status, and report execution results.",
				ReportsTo:  "lead",
			},
		}

	case "research":
		return []RoleConfig{
			{
				ID:          "lead",
				Title:       "Research Lead",
				Name:        "Dr. Chen",
				Visibility:  "client",
				Persona:     "You are Dr. Chen, the Research Lead. Receive research requests, break them down, assign to researchers, and compile findings.",
				CanDelegate: []string{"researcher", "writer"},
			},
			{
				ID:         "researcher",
				Title:      "Researcher",
				Names:      []string{"Priya", "James"},
				Visibility: "internal",
				Persona:    "You are a Researcher. Investigate topics thoroughly, gather information, analyze data, and report findings to Dr. Chen.",
				ReportsTo:  "lead",
				Count:      2,
			},
			{
				ID:         "writer",
				Title:      "Technical Writer",
				Name:       "Emma",
				Visibility: "internal",
				Persona:    "You are Emma, the Technical Writer. Compile research into clear, professional reports and documentation.",
				ReportsTo:  "lead",
			},
		}

	case "support":
		return []RoleConfig{
			{
				ID:          "triage",
				Title:       "Support Triage",
				Name:        "Chris",
				Visibility:  "client",
				Persona:     "You are Chris, Support Triage. Receive customer inquiries, categorize issues, and route to appropriate specialists.",
				CanDelegate: []string{"technical", "billing", "general"},
			},
			{
				ID:         "technical",
				Title:      "Technical Specialist",
				Name:       "Maya",
				Visibility: "internal",
				Persona:    "You are Maya, a Technical Specialist. Resolve technical issues, debug problems, and provide technical solutions.",
				ReportsTo:  "triage",
			},
			{
				ID:         "billing",
				Title:      "Billing Specialist",
				Name:       "Ryan",
				Visibility: "internal",
				Persona:    "You are Ryan, the Billing Specialist. Handle billing inquiries, process refunds, and resolve payment issues.",
				ReportsTo:  "triage",
			},
			{
				ID:         "general",
				Title:      "General Support",
				Name:       "Kim",
				Visibility: "internal",
				Persona:    "You are Kim, General Support. Handle general inquiries, provide information, and escalate complex issues.",
				ReportsTo:  "triage",
			},
		}

	default:
		return []RoleConfig{}
	}
}

func buildCustomRoles(reader *bufio.Reader) []RoleConfig {
	var roles []RoleConfig

	fmt.Println()
	fmt.Println("Let's define your team roles.")
	fmt.Println("You'll need at least one client-facing role.")
	fmt.Println()

	for {
		fmt.Printf("─── Role %d ───\n", len(roles)+1)

		id := prompt(reader, "Role ID (e.g., lead, worker)", "")
		if id == "" {
			if len(roles) == 0 {
				fmt.Println("You need at least one role.")
				continue
			}
			break
		}

		title := prompt(reader, "Title", strings.Title(id))

		visChoice := prompt(reader, "Visibility [1=client, 2=internal]", "1")
		visibility := "client"
		if visChoice == "2" {
			visibility = "internal"
		}

		persona := prompt(reader, "Persona (short description)", fmt.Sprintf("You are the %s.", title))

		countStr := prompt(reader, "Count (number of this role)", "1")
		count, _ := strconv.Atoi(countStr)
		if count < 1 {
			count = 1
		}

		var reportsTo string
		if visibility == "internal" && len(roles) > 0 {
			reportsTo = prompt(reader, "Reports to (role ID)", roles[0].ID)
		}

		role := RoleConfig{
			ID:         id,
			Title:      title,
			Visibility: visibility,
			Persona:    persona,
			ReportsTo:  reportsTo,
			Count:      count,
		}

		roles = append(roles, role)

		fmt.Println()
		another := prompt(reader, "Add another role? [y/N]", "n")
		if strings.ToLower(another) != "y" {
			break
		}
		fmt.Println()
	}

	// Set up delegation for client-facing roles
	for i, role := range roles {
		if role.Visibility == "client" {
			var delegates []string
			for _, r := range roles {
				if r.ID != role.ID {
					delegates = append(delegates, r.ID)
				}
			}
			roles[i].CanDelegate = delegates
		}
	}

	return roles
}

func generateTeamYAML(teamCfg *TeamConfig) string {
	tmpl := `apiVersion: {{ .APIVersion }}
kind: {{ .Kind }}
metadata:
  name: {{ .Name }}
  description: {{ .Description }}

client_facing:
{{- range .ClientFacing }}
  - {{ . }}
{{- end }}

roles:
{{- range .Roles }}
  {{ .ID }}:
    title: {{ .Title }}
{{- if and (gt .Count 1) .Names }}
    names:
{{- range .Names }}
      - {{ . }}
{{- end }}
{{- else if .Name }}
    name: {{ .Name }}
{{- end }}
    visibility: {{ .Visibility }}
{{- if gt .Count 1 }}
    count: {{ .Count }}
{{- end }}
    model:
      provider: {{ .Provider }}
      model: {{ .Model }}
    persona: |
      {{ .Persona }}
{{- if .CanDelegate }}
    can_delegate:
{{- range .CanDelegate }}
      - {{ . }}
{{- end }}
{{- end }}
{{- if .ReportsTo }}
    reports_to: {{ .ReportsTo }}
{{- end }}
{{ end }}`

	t := template.Must(template.New("team").Parse(tmpl))
	var buf strings.Builder
	t.Execute(&buf, teamCfg)
	return buf.String()
}
