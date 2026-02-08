package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// TestResult represents a test run result
type TestResult struct {
	ID        string    `json:"id"`
	Suite     string    `json:"suite"`
	Name      string    `json:"name"`
	Status    string    `json:"status"` // passed, failed, skipped, error
	Duration  float64   `json:"duration_ms"`
	Error     string    `json:"error,omitempty"`
	Output    string    `json:"output,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// BugReport represents a bug report
type BugReport struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Severity    string                 `json:"severity"` // critical, high, medium, low
	Status      string                 `json:"status"`   // open, in_progress, fixed, verified, closed
	ReportedBy  string                 `json:"reported_by"`
	AssignedTo  string                 `json:"assigned_to,omitempty"`
	Steps       []string               `json:"steps,omitempty"`
	Expected    string                 `json:"expected,omitempty"`
	Actual      string                 `json:"actual,omitempty"`
	Environment map[string]interface{} `json:"environment,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	RelatedTask string                 `json:"related_task,omitempty"`
}

// ============================================================================
// Testing Tools
// ============================================================================

// RunTestsTool executes tests
type RunTestsTool struct {
	WorkingDir   string
	ArtifactPath string
}

func (t *RunTestsTool) Name() string        { return "run_tests" }
func (t *RunTestsTool) Description() string { return "Run tests and capture results" }

func (t *RunTestsTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	// Determine test command
	command := "go test ./..."
	if cmd, ok := args["command"].(string); ok {
		command = cmd
	}

	// Get test pattern/filter
	if pattern, ok := args["pattern"].(string); ok && pattern != "" {
		command = fmt.Sprintf("%s -run '%s'", command, pattern)
	}

	// Add verbose flag if requested
	if verbose, ok := args["verbose"].(bool); ok && verbose {
		if strings.Contains(command, "go test") {
			command = strings.Replace(command, "go test", "go test -v", 1)
		}
	}

	// Set timeout
	timeout := 300 * time.Second
	if to, ok := args["timeout"].(float64); ok {
		timeout = time.Duration(to) * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = t.WorkingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startTime := time.Now()
	err := cmd.Run()
	duration := time.Since(startTime).Milliseconds()

	// Parse output for results
	output := stdout.String()
	if output == "" {
		output = stderr.String()
	}

	// Basic result structure
	result := map[string]interface{}{
		"command":     command,
		"duration_ms": duration,
		"output":      output,
		"passed":      err == nil,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result["exit_code"] = exitErr.ExitCode()
		}
		result["error"] = stderr.String()
	}

	// Try to parse JSON test output if available
	if strings.Contains(command, "-json") {
		var testResults []TestResult
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "{") {
				var tr TestResult
				if json.Unmarshal([]byte(line), &tr) == nil {
					testResults = append(testResults, tr)
				}
			}
		}
		if len(testResults) > 0 {
			result["results"] = testResults
		}
	}

	// Save results to artifacts
	if t.ArtifactPath != "" {
		timestamp := time.Now().Format("2006-01-02-150405")
		resultsPath := filepath.Join(t.ArtifactPath, "tests", fmt.Sprintf("test-run-%s.json", timestamp))
		os.MkdirAll(filepath.Dir(resultsPath), 0755)
		data, _ := json.MarshalIndent(result, "", "  ")
		os.WriteFile(resultsPath, data, 0644)
		result["artifact_path"] = resultsPath
	}

	return result, nil
}

// CreateBugReportTool creates a bug report
type CreateBugReportTool struct {
	ArtifactPath string
	ReportedBy   string
}

func (t *CreateBugReportTool) Name() string        { return "create_bug_report" }
func (t *CreateBugReportTool) Description() string { return "Create a bug report" }

func (t *CreateBugReportTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	title, ok := args["title"].(string)
	if !ok || title == "" {
		return nil, fmt.Errorf("title is required")
	}

	description, ok := args["description"].(string)
	if !ok || description == "" {
		return nil, fmt.Errorf("description is required")
	}

	bug := &BugReport{
		ID:          fmt.Sprintf("BUG-%d", time.Now().Unix()),
		Title:       title,
		Description: description,
		Severity:    "medium",
		Status:      "open",
		ReportedBy:  t.ReportedBy,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if sev, ok := args["severity"].(string); ok {
		bug.Severity = sev
	}
	if expected, ok := args["expected"].(string); ok {
		bug.Expected = expected
	}
	if actual, ok := args["actual"].(string); ok {
		bug.Actual = actual
	}
	if steps, ok := args["steps"].([]interface{}); ok {
		for _, step := range steps {
			if s, ok := step.(string); ok {
				bug.Steps = append(bug.Steps, s)
			}
		}
	}
	if env, ok := args["environment"].(map[string]interface{}); ok {
		bug.Environment = env
	}
	if taskID, ok := args["related_task"].(string); ok {
		bug.RelatedTask = taskID
	}

	// Save bug report
	bugPath := filepath.Join(t.ArtifactPath, "bugs", bug.ID+".json")
	os.MkdirAll(filepath.Dir(bugPath), 0755)
	data, _ := json.MarshalIndent(bug, "", "  ")
	if err := os.WriteFile(bugPath, data, 0644); err != nil {
		return nil, fmt.Errorf("save bug report: %w", err)
	}

	return map[string]interface{}{
		"id":          bug.ID,
		"title":       bug.Title,
		"severity":    bug.Severity,
		"status":      bug.Status,
		"path":        bugPath,
		"reported_by": bug.ReportedBy,
	}, nil
}

// VerifyFixTool verifies a bug fix
type VerifyFixTool struct {
	ArtifactPath string
	VerifiedBy   string
}

func (t *VerifyFixTool) Name() string        { return "verify_fix" }
func (t *VerifyFixTool) Description() string { return "Verify a bug fix" }

func (t *VerifyFixTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	bugID, ok := args["bug_id"].(string)
	if !ok || bugID == "" {
		return nil, fmt.Errorf("bug_id is required")
	}

	verified, ok := args["verified"].(bool)
	if !ok {
		return nil, fmt.Errorf("verified is required (true/false)")
	}

	// Load bug report
	bugPath := filepath.Join(t.ArtifactPath, "bugs", bugID+".json")
	data, err := os.ReadFile(bugPath)
	if err != nil {
		return nil, fmt.Errorf("bug not found: %s", bugID)
	}

	var bug BugReport
	if err := json.Unmarshal(data, &bug); err != nil {
		return nil, fmt.Errorf("parse bug report: %w", err)
	}

	// Update status
	if verified {
		bug.Status = "verified"
	} else {
		bug.Status = "open" // Reopen if not verified
	}
	bug.UpdatedAt = time.Now()

	// Save updated bug
	data, _ = json.MarshalIndent(bug, "", "  ")
	if err := os.WriteFile(bugPath, data, 0644); err != nil {
		return nil, fmt.Errorf("save bug report: %w", err)
	}

	notes := ""
	if n, ok := args["notes"].(string); ok {
		notes = n
	}

	return map[string]interface{}{
		"bug_id":      bug.ID,
		"status":      bug.Status,
		"verified":    verified,
		"verified_by": t.VerifiedBy,
		"notes":       notes,
	}, nil
}

// ListTestResultsTool lists test results
type ListTestResultsTool struct {
	ArtifactPath string
}

func (t *ListTestResultsTool) Name() string        { return "list_test_results" }
func (t *ListTestResultsTool) Description() string { return "List recent test results" }

func (t *ListTestResultsTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	testsDir := filepath.Join(t.ArtifactPath, "tests")

	entries, err := os.ReadDir(testsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]interface{}{
				"results": []interface{}{},
				"count":   0,
			}, nil
		}
		return nil, fmt.Errorf("read tests directory: %w", err)
	}

	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	var results []map[string]interface{}
	for i := len(entries) - 1; i >= 0 && len(results) < limit; i-- {
		entry := entries[i]
		if entry.IsDir() {
			continue
		}

		path := filepath.Join(testsDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			continue
		}

		result["filename"] = entry.Name()
		results = append(results, result)
	}

	return map[string]interface{}{
		"results": results,
		"count":   len(results),
	}, nil
}
