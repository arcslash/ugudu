package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ============================================================================
// Git Tools
// ============================================================================

// GitStatusTool shows git status
type GitStatusTool struct {
	WorkingDir string
}

func (t *GitStatusTool) Name() string        { return "git_status" }
func (t *GitStatusTool) Description() string { return "Show the working tree status" }

func (t *GitStatusTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain", "-b")
	cmd.Dir = t.WorkingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git status: %s", stderr.String())
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")

	result := map[string]interface{}{
		"branch":   "",
		"modified": []string{},
		"added":    []string{},
		"deleted":  []string{},
		"untracked": []string{},
	}

	modified := []string{}
	added := []string{}
	deleted := []string{}
	untracked := []string{}

	for _, line := range lines {
		if len(line) < 3 {
			continue
		}

		if strings.HasPrefix(line, "##") {
			// Branch info
			parts := strings.Split(line[3:], "...")
			result["branch"] = parts[0]
			continue
		}

		status := line[:2]
		file := strings.TrimSpace(line[3:])

		switch {
		case status[0] == 'M' || status[1] == 'M':
			modified = append(modified, file)
		case status[0] == 'A':
			added = append(added, file)
		case status[0] == 'D' || status[1] == 'D':
			deleted = append(deleted, file)
		case status == "??":
			untracked = append(untracked, file)
		}
	}

	result["modified"] = modified
	result["added"] = added
	result["deleted"] = deleted
	result["untracked"] = untracked
	result["clean"] = len(modified) == 0 && len(added) == 0 && len(deleted) == 0 && len(untracked) == 0

	return result, nil
}

// GitDiffTool shows git diff
type GitDiffTool struct {
	WorkingDir string
}

func (t *GitDiffTool) Name() string        { return "git_diff" }
func (t *GitDiffTool) Description() string { return "Show changes between commits, commit and working tree, etc" }

func (t *GitDiffTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	cmdArgs := []string{"diff"}

	// Add optional arguments
	if staged, ok := args["staged"].(bool); ok && staged {
		cmdArgs = append(cmdArgs, "--staged")
	}
	if file, ok := args["file"].(string); ok && file != "" {
		cmdArgs = append(cmdArgs, "--", file)
	}
	if commit, ok := args["commit"].(string); ok && commit != "" {
		cmdArgs = append(cmdArgs, commit)
	}

	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	cmd.Dir = t.WorkingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git diff: %s", stderr.String())
	}

	diff := stdout.String()

	// Parse diff stats
	statsCmd := exec.CommandContext(ctx, "git", append(cmdArgs, "--stat")...)
	statsCmd.Dir = t.WorkingDir
	var statsOut bytes.Buffer
	statsCmd.Stdout = &statsOut
	statsCmd.Run()

	return map[string]interface{}{
		"diff":  diff,
		"stats": statsOut.String(),
		"empty": len(diff) == 0,
	}, nil
}

// GitCommitTool creates a git commit
type GitCommitTool struct {
	WorkingDir string
	Author     string
}

func (t *GitCommitTool) Name() string        { return "git_commit" }
func (t *GitCommitTool) Description() string { return "Record changes to the repository" }

func (t *GitCommitTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	message, ok := args["message"].(string)
	if !ok || message == "" {
		return nil, fmt.Errorf("message is required")
	}

	// Stage files if specified
	if files, ok := args["files"].([]interface{}); ok && len(files) > 0 {
		addArgs := []string{"add"}
		for _, f := range files {
			if s, ok := f.(string); ok {
				addArgs = append(addArgs, s)
			}
		}
		addCmd := exec.CommandContext(ctx, "git", addArgs...)
		addCmd.Dir = t.WorkingDir
		if err := addCmd.Run(); err != nil {
			return nil, fmt.Errorf("git add: %w", err)
		}
	}

	// Stage all if requested
	if all, ok := args["all"].(bool); ok && all {
		addCmd := exec.CommandContext(ctx, "git", "add", "-A")
		addCmd.Dir = t.WorkingDir
		if err := addCmd.Run(); err != nil {
			return nil, fmt.Errorf("git add: %w", err)
		}
	}

	// Create commit
	commitArgs := []string{"commit", "-m", message}
	cmd := exec.CommandContext(ctx, "git", commitArgs...)
	cmd.Dir = t.WorkingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git commit: %s", stderr.String())
	}

	// Get commit hash
	hashCmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	hashCmd.Dir = t.WorkingDir
	var hashOut bytes.Buffer
	hashCmd.Stdout = &hashOut
	hashCmd.Run()

	return map[string]interface{}{
		"message":    message,
		"commit":     strings.TrimSpace(hashOut.String()),
		"output":     stdout.String(),
		"created_at": time.Now(),
	}, nil
}

// GitLogTool shows git log
type GitLogTool struct {
	WorkingDir string
}

func (t *GitLogTool) Name() string        { return "git_log" }
func (t *GitLogTool) Description() string { return "Show commit logs" }

func (t *GitLogTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	format := "--format=%H|%an|%ae|%s|%ci"
	cmdArgs := []string{"log", format, fmt.Sprintf("-n%d", limit)}

	if file, ok := args["file"].(string); ok && file != "" {
		cmdArgs = append(cmdArgs, "--", file)
	}

	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	cmd.Dir = t.WorkingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git log: %s", stderr.String())
	}

	var commits []map[string]interface{}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 5)
		if len(parts) >= 5 {
			commits = append(commits, map[string]interface{}{
				"hash":    parts[0],
				"author":  parts[1],
				"email":   parts[2],
				"message": parts[3],
				"date":    parts[4],
			})
		}
	}

	return map[string]interface{}{
		"commits": commits,
		"count":   len(commits),
	}, nil
}

// GitBranchTool manages git branches
type GitBranchTool struct {
	WorkingDir string
}

func (t *GitBranchTool) Name() string        { return "git_branch" }
func (t *GitBranchTool) Description() string { return "List, create, or delete branches" }

func (t *GitBranchTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	action := "list"
	if a, ok := args["action"].(string); ok {
		action = a
	}

	switch action {
	case "list":
		cmd := exec.CommandContext(ctx, "git", "branch", "-a")
		cmd.Dir = t.WorkingDir
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("git branch: %w", err)
		}

		var branches []string
		var current string
		for _, line := range strings.Split(stdout.String(), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "* ") {
				current = line[2:]
				branches = append(branches, current)
			} else {
				branches = append(branches, line)
			}
		}

		return map[string]interface{}{
			"branches": branches,
			"current":  current,
			"count":    len(branches),
		}, nil

	case "create":
		name, ok := args["name"].(string)
		if !ok || name == "" {
			return nil, fmt.Errorf("name is required for create action")
		}

		cmd := exec.CommandContext(ctx, "git", "branch", name)
		cmd.Dir = t.WorkingDir
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("git branch create: %w", err)
		}

		return map[string]interface{}{
			"created": name,
		}, nil

	case "checkout":
		name, ok := args["name"].(string)
		if !ok || name == "" {
			return nil, fmt.Errorf("name is required for checkout action")
		}

		cmd := exec.CommandContext(ctx, "git", "checkout", name)
		cmd.Dir = t.WorkingDir
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("git checkout: %s", stderr.String())
		}

		return map[string]interface{}{
			"checked_out": name,
		}, nil

	default:
		return nil, fmt.Errorf("unknown action: %s", action)
	}
}
