package team

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSpec(t *testing.T) {
	// Create a temp spec file
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "test-team.yaml")

	specContent := `
apiVersion: ugudu/v1
kind: Team
metadata:
  name: test-team
  description: Test team for unit tests

client_facing:
  - lead

roles:
  lead:
    title: Team Lead
    visibility: client
    model:
      provider: anthropic
      model: claude-sonnet-4-20250514
    persona: |
      You are the team lead.
    can_delegate:
      - worker

  worker:
    title: Worker
    visibility: internal
    model:
      provider: openai
      model: gpt-4o
    persona: |
      You are a worker.
    reports_to: lead
`

	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("Failed to write test spec: %v", err)
	}

	spec, err := LoadSpec(specPath)
	if err != nil {
		t.Fatalf("LoadSpec failed: %v", err)
	}

	// Verify spec
	if spec.APIVersion != "ugudu/v1" {
		t.Errorf("Expected apiVersion 'ugudu/v1', got '%s'", spec.APIVersion)
	}

	if spec.Kind != "Team" {
		t.Errorf("Expected kind 'Team', got '%s'", spec.Kind)
	}

	if spec.Metadata.Name != "test-team" {
		t.Errorf("Expected name 'test-team', got '%s'", spec.Metadata.Name)
	}

	if len(spec.ClientFacing) != 1 || spec.ClientFacing[0] != "lead" {
		t.Errorf("Expected client_facing ['lead'], got %v", spec.ClientFacing)
	}

	if len(spec.Roles) != 2 {
		t.Errorf("Expected 2 roles, got %d", len(spec.Roles))
	}

	// Check lead role
	lead, ok := spec.Roles["lead"]
	if !ok {
		t.Fatal("Missing 'lead' role")
	}

	if lead.Title != "Team Lead" {
		t.Errorf("Expected title 'Team Lead', got '%s'", lead.Title)
	}

	if lead.Visibility != "client" {
		t.Errorf("Expected visibility 'client', got '%s'", lead.Visibility)
	}

	if lead.Model.Provider != "anthropic" {
		t.Errorf("Expected provider 'anthropic', got '%s'", lead.Model.Provider)
	}

	// Check worker role
	worker, ok := spec.Roles["worker"]
	if !ok {
		t.Fatal("Missing 'worker' role")
	}

	if worker.ReportsTo != "lead" {
		t.Errorf("Expected reports_to 'lead', got '%s'", worker.ReportsTo)
	}
}

func TestLoadSpecNonExistent(t *testing.T) {
	_, err := LoadSpec("/non/existent/path.yaml")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestLoadSpecInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "invalid.yaml")

	if err := os.WriteFile(specPath, []byte("invalid: yaml: content: ["), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := LoadSpec(specPath)
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestRoleDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "minimal.yaml")

	// Minimal spec with missing optional fields
	specContent := `
apiVersion: ugudu/v1
kind: Team
metadata:
  name: minimal-team

roles:
  agent:
    title: Agent
    model:
      provider: anthropic
      model: claude-sonnet-4-20250514
`

	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("Failed to write test spec: %v", err)
	}

	spec, err := LoadSpec(specPath)
	if err != nil {
		t.Fatalf("LoadSpec failed: %v", err)
	}

	agent := spec.Roles["agent"]

	// Default visibility should be empty (internal by default in team logic)
	if agent.Visibility != "" {
		t.Logf("Visibility: %s (empty is valid)", agent.Visibility)
	}

	// Count should default to 0 (treated as 1)
	if agent.Count != 0 {
		t.Logf("Count: %d (0 is valid, treated as 1)", agent.Count)
	}
}

func TestMessageTypes(t *testing.T) {
	msg := Message{
		Type:    MsgClientResponse,
		From:    "lead",
		To:      "client",
		Content: "Hello, world!",
	}

	if msg.Type != MsgClientResponse {
		t.Errorf("Expected type '%s', got '%s'", MsgClientResponse, msg.Type)
	}

	if msg.From != "lead" {
		t.Errorf("Expected from 'lead', got '%s'", msg.From)
	}
}

func TestTaskStatus(t *testing.T) {
	task := Task{
		ID:      "task-1",
		Content: "Test task",
		Status:  TaskPending,
		From:    "client",
		To:      "worker",
	}

	if task.Status != TaskPending {
		t.Errorf("Expected status pending, got %s", task.Status)
	}

	task.Status = TaskInProgress
	if task.Status != TaskInProgress {
		t.Errorf("Expected status in_progress, got %s", task.Status)
	}

	task.Status = TaskCompleted
	if task.Status != TaskCompleted {
		t.Errorf("Expected status completed, got %s", task.Status)
	}
}

func TestMemberStatus(t *testing.T) {
	statuses := []MemberStatus{
		MemberIdle,
		MemberWorking,
		MemberWaiting,
		MemberBlocked,
		MemberOffline,
	}

	expected := []string{"idle", "working", "waiting", "blocked", "offline"}

	for i, status := range statuses {
		if string(status) != expected[i] {
			t.Errorf("Expected status '%s', got '%s'", expected[i], status)
		}
	}
}

func TestTaskStatusConstants(t *testing.T) {
	// Verify all task status constants
	if TaskPending != "pending" {
		t.Errorf("TaskPending should be 'pending'")
	}
	if TaskAssigned != "assigned" {
		t.Errorf("TaskAssigned should be 'assigned'")
	}
	if TaskInProgress != "in_progress" {
		t.Errorf("TaskInProgress should be 'in_progress'")
	}
	if TaskCompleted != "completed" {
		t.Errorf("TaskCompleted should be 'completed'")
	}
	if TaskFailed != "failed" {
		t.Errorf("TaskFailed should be 'failed'")
	}
	if TaskBlocked != "blocked" {
		t.Errorf("TaskBlocked should be 'blocked'")
	}
}

func TestMessageTypeConstants(t *testing.T) {
	// Verify key message type constants
	if MsgClientRequest != "client_request" {
		t.Errorf("MsgClientRequest should be 'client_request'")
	}
	if MsgClientResponse != "client_response" {
		t.Errorf("MsgClientResponse should be 'client_response'")
	}
	if MsgDelegation != "delegation" {
		t.Errorf("MsgDelegation should be 'delegation'")
	}
}

func TestWorkflowSpec(t *testing.T) {
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "workflow-team.yaml")

	specContent := `
apiVersion: ugudu/v1
kind: Team
metadata:
  name: workflow-team

roles:
  lead:
    title: Lead
    model:
      provider: anthropic
      model: claude-sonnet-4-20250514

workflow:
  pattern: pipeline
  auto_assign: true
  stages:
    - name: analysis
      owner: analyst
    - name: execution
      owner: executor
`

	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("Failed to write test spec: %v", err)
	}

	spec, err := LoadSpec(specPath)
	if err != nil {
		t.Fatalf("LoadSpec failed: %v", err)
	}

	if spec.Workflow.Pattern != "pipeline" {
		t.Errorf("Expected pattern 'pipeline', got '%s'", spec.Workflow.Pattern)
	}

	if !spec.Workflow.AutoAssign {
		t.Error("Expected auto_assign to be true")
	}

	if len(spec.Workflow.Stages) != 2 {
		t.Errorf("Expected 2 stages, got %d", len(spec.Workflow.Stages))
	}
}
