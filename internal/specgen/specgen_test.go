package specgen

import (
	"strings"
	"testing"
)

func TestExtractSpec(t *testing.T) {
	g := &Generator{}

	tests := []struct {
		name     string
		content  string
		wantSpec bool
		wantName string
	}{
		{
			name: "valid wrapped spec",
			content: `Here's your team:

` + "```json" + `
{
  "ready": true,
  "spec": {
    "name": "fitness-team",
    "description": "Team for fitness app",
    "roles": [
      {
        "id": "pm",
        "title": "Product Manager",
        "visibility": "client",
        "persona": "You are the PM"
      }
    ],
    "client_facing": ["pm"]
  }
}
` + "```",
			wantSpec: true,
			wantName: "fitness-team",
		},
		{
			name: "no json block",
			content: `I need more information about your project.
What technologies are you using?`,
			wantSpec: false,
		},
		{
			name:     "empty content",
			content:  "",
			wantSpec: false,
		},
		{
			name: "invalid json",
			content: "```json\n{invalid}\n```",
			wantSpec: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, ok := g.extractSpec(tt.content)

			if ok != tt.wantSpec {
				t.Errorf("extractSpec() ok = %v, want %v", ok, tt.wantSpec)
			}

			if tt.wantSpec && spec != nil {
				if spec.Name != tt.wantName {
					t.Errorf("spec.Name = %v, want %v", spec.Name, tt.wantName)
				}
			}
		})
	}
}

func TestTeamSpecToYAML(t *testing.T) {
	spec := &TeamSpec{
		Name:        "test-team",
		Description: "Test team description",
		Roles: []RoleSpec{
			{
				ID:          "pm",
				Title:       "Product Manager",
				Visibility:  "client",
				Persona:     "You are the PM.",
				CanDelegate: []string{"engineer"},
			},
			{
				ID:         "engineer",
				Title:      "Engineer",
				Visibility: "internal",
				Count:      2,
				Persona:    "You are an engineer.",
				ReportsTo:  "pm",
			},
		},
		ClientFacing: []string{"pm"},
	}

	yaml := spec.ToYAML("anthropic", "claude-sonnet-4-20250514")

	// Check key elements are present
	checks := []string{
		"apiVersion: ugudu/v1",
		"kind: Team",
		"name: test-team",
		"description: Test team description",
		"client_facing:",
		"- pm",
		"pm:",
		"title: Product Manager",
		"visibility: client",
		"provider: anthropic",
		"model: claude-sonnet-4-20250514",
		"can_delegate:",
		"- engineer",
		"engineer:",
		"count: 2",
		"reports_to: pm",
	}

	for _, check := range checks {
		if !strings.Contains(yaml, check) {
			t.Errorf("YAML missing expected content: %s", check)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"this is a long string", 10, "this is..."},
		{"exactly10!", 10, "exactly10!"},
		{"", 10, ""},
	}

	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}
