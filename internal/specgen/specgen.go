// Package specgen provides LLM-powered team specification generation
package specgen

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/arcslash/ugudu/internal/provider"
)

// Generator uses an LLM to generate team specifications
type Generator struct {
	provider provider.Provider
	model    string
}

// NewGenerator creates a new spec generator
func NewGenerator(p provider.Provider, model string) *Generator {
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}
	return &Generator{
		provider: p,
		model:    model,
	}
}

// ConversationState tracks the interview progress
type ConversationState struct {
	Messages     []provider.Message
	ProjectIdea  string
	Technologies []string
	TeamSize     string
	Complexity   string
	Features     []string
	IsComplete   bool
}

// TeamSpec represents the generated specification
type TeamSpec struct {
	Name         string     `json:"name"`
	Description  string     `json:"description"`
	Roles        []RoleSpec `json:"roles"`
	ClientFacing []string   `json:"client_facing"`
}

// RoleSpec represents a role in the team
type RoleSpec struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Name        string   `json:"name,omitempty"`  // Personal name for single member
	Names       []string `json:"names,omitempty"` // Names for multiple members
	Visibility  string   `json:"visibility"`
	Count       int      `json:"count,omitempty"`
	Persona     string   `json:"persona"`
	CanDelegate []string `json:"can_delegate,omitempty"`
	ReportsTo   string   `json:"reports_to,omitempty"`
}

const systemPrompt = `You are a helpful assistant that helps users create AI team specifications for the Ugudu platform.

Ugudu allows users to create teams of AI agents that work together. Each team has:
- Roles (e.g., PM, Engineer, QA, Analyst)
- Each role has a name, persona, visibility (client-facing or internal), and relationships
- Teams can handle software development, research, trading, support, and custom tasks

Your job is to:
1. Interview the user to understand what they want to build
2. Ask clarifying questions to understand their needs
3. When you have enough information, generate a complete team specification

INTERVIEW GUIDELINES:
- Ask 3-5 focused questions total, not too many
- Be conversational and friendly
- Understand: what they're building, technologies, complexity, timeline
- Suggest appropriate team structures based on their needs

When you're ready to generate the spec, respond with a JSON block in this exact format:

` + "```json" + `
{
  "ready": true,
  "spec": {
    "name": "team-name",
    "description": "Team description",
    "roles": [
      {
        "id": "pm",
        "title": "Product Manager",
        "name": "Sarah",
        "visibility": "client",
        "persona": "You are Sarah, the Product Manager...",
        "can_delegate": ["engineer", "qa"]
      },
      {
        "id": "engineer",
        "title": "Software Engineer",
        "names": ["Alex", "Jordan"],
        "visibility": "internal",
        "count": 2,
        "persona": "You are a Software Engineer...",
        "reports_to": "pm"
      }
    ],
    "client_facing": ["pm"]
  }
}
` + "```" + `

IMPORTANT:
- Always have at least one client-facing role
- Give each role member a unique, friendly name (use "name" for single roles, "names" array for multiple)
- Personas should be detailed and specific to the project - include the name in the persona
- Include appropriate delegation chains
- Scale team size based on project complexity
- For simple projects: 2-3 roles
- For medium projects: 3-5 roles
- For complex projects: 5-8 roles
- Use diverse, professional names that feel natural

If you need more information, just ask the question directly without the JSON block.`

// StartConversation begins a new spec generation conversation
func (g *Generator) StartConversation(ctx context.Context, initialInput string) (*ConversationState, string, error) {
	state := &ConversationState{
		Messages: []provider.Message{
			{Role: "system", Content: systemPrompt},
		},
	}

	// Add initial user message
	userMsg := initialInput
	if userMsg == "" {
		userMsg = "Hi, I'd like to create a new AI team. Can you help me figure out what kind of team I need?"
	}
	state.Messages = append(state.Messages, provider.Message{Role: "user", Content: userMsg})

	// Get LLM response
	resp, err := g.chat(ctx, state.Messages)
	if err != nil {
		return nil, "", err
	}

	state.Messages = append(state.Messages, provider.Message{Role: "assistant", Content: resp})

	// Check if response contains a complete spec
	if spec, ok := g.extractSpec(resp); ok {
		state.IsComplete = true
		return state, g.formatSpecPreview(spec), nil
	}

	return state, resp, nil
}

// ContinueConversation continues the interview
func (g *Generator) ContinueConversation(ctx context.Context, state *ConversationState, userInput string) (string, error) {
	state.Messages = append(state.Messages, provider.Message{Role: "user", Content: userInput})

	resp, err := g.chat(ctx, state.Messages)
	if err != nil {
		return "", err
	}

	state.Messages = append(state.Messages, provider.Message{Role: "assistant", Content: resp})

	// Check if response contains a complete spec
	if spec, ok := g.extractSpec(resp); ok {
		state.IsComplete = true
		return g.formatSpecPreview(spec), nil
	}

	return resp, nil
}

// GetGeneratedSpec extracts the final spec from a completed conversation
func (g *Generator) GetGeneratedSpec(state *ConversationState) (*TeamSpec, error) {
	if !state.IsComplete {
		return nil, fmt.Errorf("conversation not complete")
	}

	// Find the last assistant message with the spec
	for i := len(state.Messages) - 1; i >= 0; i-- {
		if state.Messages[i].Role == "assistant" {
			if spec, ok := g.extractSpec(state.Messages[i].Content); ok {
				return spec, nil
			}
		}
	}

	return nil, fmt.Errorf("no spec found in conversation")
}

// ForceGenerate asks the LLM to generate a spec with current information
func (g *Generator) ForceGenerate(ctx context.Context, state *ConversationState) (*TeamSpec, error) {
	prompt := `Based on our conversation so far, please generate the team specification now.
If you don't have enough information, make reasonable assumptions based on what was discussed.
Respond with the JSON spec block as described in your instructions.`

	state.Messages = append(state.Messages, provider.Message{Role: "user", Content: prompt})

	resp, err := g.chat(ctx, state.Messages)
	if err != nil {
		return nil, err
	}

	state.Messages = append(state.Messages, provider.Message{Role: "assistant", Content: resp})

	if spec, ok := g.extractSpec(resp); ok {
		state.IsComplete = true
		return spec, nil
	}

	return nil, fmt.Errorf("failed to generate spec")
}

func (g *Generator) chat(ctx context.Context, messages []provider.Message) (string, error) {
	req := &provider.ChatRequest{
		Model:    g.model,
		Messages: messages,
	}

	resp, err := g.provider.Chat(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.Content, nil
}

func (g *Generator) extractSpec(content string) (*TeamSpec, bool) {
	// Look for JSON block
	start := strings.Index(content, "```json")
	if start == -1 {
		start = strings.Index(content, "```")
	}
	if start == -1 {
		return nil, false
	}

	// Find the JSON content
	jsonStart := strings.Index(content[start:], "{")
	if jsonStart == -1 {
		return nil, false
	}
	jsonStart += start

	// Find the end of JSON
	end := strings.LastIndex(content, "}")
	if end == -1 || end < jsonStart {
		return nil, false
	}

	jsonContent := content[jsonStart : end+1]

	// Try to parse as wrapped response
	var wrapped struct {
		Ready bool      `json:"ready"`
		Spec  *TeamSpec `json:"spec"`
	}

	if err := json.Unmarshal([]byte(jsonContent), &wrapped); err == nil && wrapped.Spec != nil {
		return wrapped.Spec, true
	}

	// Try to parse directly as TeamSpec
	var spec TeamSpec
	if err := json.Unmarshal([]byte(jsonContent), &spec); err == nil && len(spec.Roles) > 0 {
		return &spec, true
	}

	return nil, false
}

func (g *Generator) formatSpecPreview(spec *TeamSpec) string {
	var sb strings.Builder

	sb.WriteString("\n✨ I've designed a team for you!\n\n")
	sb.WriteString(fmt.Sprintf("**Team: %s**\n", spec.Name))
	sb.WriteString(fmt.Sprintf("%s\n\n", spec.Description))

	sb.WriteString("**Team Members:**\n")
	for _, role := range spec.Roles {
		visibility := "🔒"
		if role.Visibility == "client" {
			visibility = "👤"
		}

		// Format name(s)
		var nameDisplay string
		if role.Count > 1 && len(role.Names) > 0 {
			nameDisplay = strings.Join(role.Names, ", ")
		} else if role.Name != "" {
			nameDisplay = role.Name
		} else {
			nameDisplay = role.Title
		}

		sb.WriteString(fmt.Sprintf("  %s %s - %s\n", visibility, nameDisplay, role.Title))
	}

	sb.WriteString("\n👤 = client-facing, 🔒 = internal\n")

	return sb.String()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// ToYAML converts the spec to YAML format
func (spec *TeamSpec) ToYAML(providerName, model string) string {
	var sb strings.Builder

	sb.WriteString("apiVersion: ugudu/v1\n")
	sb.WriteString("kind: Team\n")
	sb.WriteString("metadata:\n")
	sb.WriteString(fmt.Sprintf("  name: %s\n", spec.Name))
	sb.WriteString(fmt.Sprintf("  description: %s\n", spec.Description))
	sb.WriteString("\n")

	sb.WriteString("client_facing:\n")
	for _, cf := range spec.ClientFacing {
		sb.WriteString(fmt.Sprintf("  - %s\n", cf))
	}
	sb.WriteString("\n")

	sb.WriteString("roles:\n")
	for _, role := range spec.Roles {
		sb.WriteString(fmt.Sprintf("  %s:\n", role.ID))
		sb.WriteString(fmt.Sprintf("    title: %s\n", role.Title))

		// Add name(s) for personality
		if role.Count > 1 && len(role.Names) > 0 {
			sb.WriteString("    names:\n")
			for _, name := range role.Names {
				sb.WriteString(fmt.Sprintf("      - %s\n", name))
			}
		} else if role.Name != "" {
			sb.WriteString(fmt.Sprintf("    name: %s\n", role.Name))
		}

		sb.WriteString(fmt.Sprintf("    visibility: %s\n", role.Visibility))

		if role.Count > 1 {
			sb.WriteString(fmt.Sprintf("    count: %d\n", role.Count))
		}

		sb.WriteString("    model:\n")
		sb.WriteString(fmt.Sprintf("      provider: %s\n", providerName))
		sb.WriteString(fmt.Sprintf("      model: %s\n", model))

		sb.WriteString("    persona: |\n")
		// Indent persona properly
		lines := strings.Split(role.Persona, "\n")
		for _, line := range lines {
			sb.WriteString(fmt.Sprintf("      %s\n", line))
		}

		if len(role.CanDelegate) > 0 {
			sb.WriteString("    can_delegate:\n")
			for _, d := range role.CanDelegate {
				sb.WriteString(fmt.Sprintf("      - %s\n", d))
			}
		}

		if role.ReportsTo != "" {
			sb.WriteString(fmt.Sprintf("    reports_to: %s\n", role.ReportsTo))
		}

		sb.WriteString("\n")
	}

	return sb.String()
}
