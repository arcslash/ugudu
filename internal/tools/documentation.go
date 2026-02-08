package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Document represents a documentation artifact
type Document struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Type      string    `json:"type"` // doc, requirement, spec
	Content   string    `json:"content"`
	Author    string    `json:"author"`
	Version   string    `json:"version"`
	Status    string    `json:"status"` // draft, review, approved
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Requirement represents a business requirement
type Requirement struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Type        string    `json:"type"` // functional, non-functional
	Priority    string    `json:"priority"`
	Status      string    `json:"status"` // proposed, approved, implemented, verified
	Author      string    `json:"author"`
	Rationale   string    `json:"rationale,omitempty"`
	Criteria    []string  `json:"acceptance_criteria,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Spec represents a technical specification
type Spec struct {
	ID           string                 `json:"id"`
	Title        string                 `json:"title"`
	Description  string                 `json:"description"`
	Author       string                 `json:"author"`
	Version      string                 `json:"version"`
	Status       string                 `json:"status"` // draft, review, approved
	Requirements []string               `json:"requirements,omitempty"`
	Components   []string               `json:"components,omitempty"`
	Interfaces   map[string]interface{} `json:"interfaces,omitempty"`
	DataModels   map[string]interface{} `json:"data_models,omitempty"`
	Path         string                 `json:"path"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// ============================================================================
// Documentation Tools
// ============================================================================

// CreateDocTool creates a documentation file
type CreateDocTool struct {
	ArtifactPath string
	Author       string
}

func (t *CreateDocTool) Name() string        { return "create_doc" }
func (t *CreateDocTool) Description() string { return "Create a documentation file" }

func (t *CreateDocTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	title, ok := args["title"].(string)
	if !ok || title == "" {
		return nil, fmt.Errorf("title is required")
	}

	content, ok := args["content"].(string)
	if !ok || content == "" {
		return nil, fmt.Errorf("content is required")
	}

	docType := "doc"
	if dt, ok := args["type"].(string); ok {
		docType = dt
	}

	// Generate filename from title
	filename := sanitizeFilename(title) + ".md"
	docPath := filepath.Join(t.ArtifactPath, "docs", filename)

	doc := &Document{
		ID:        fmt.Sprintf("DOC-%d", time.Now().Unix()),
		Title:     title,
		Type:      docType,
		Content:   content,
		Author:    t.Author,
		Version:   "1.0",
		Status:    "draft",
		Path:      docPath,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Create markdown document
	mdContent := fmt.Sprintf(`# %s

*Author: %s*
*Version: %s*
*Status: %s*
*Created: %s*

---

%s
`, doc.Title, doc.Author, doc.Version, doc.Status, doc.CreatedAt.Format(time.RFC3339), content)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(docPath), 0755); err != nil {
		return nil, fmt.Errorf("create docs directory: %w", err)
	}

	if err := os.WriteFile(docPath, []byte(mdContent), 0644); err != nil {
		return nil, fmt.Errorf("write document: %w", err)
	}

	// Also save metadata
	metaPath := filepath.Join(t.ArtifactPath, "docs", ".metadata", doc.ID+".json")
	os.MkdirAll(filepath.Dir(metaPath), 0755)
	metaData, _ := json.MarshalIndent(doc, "", "  ")
	os.WriteFile(metaPath, metaData, 0644)

	return map[string]interface{}{
		"id":         doc.ID,
		"title":      doc.Title,
		"path":       doc.Path,
		"type":       doc.Type,
		"status":     doc.Status,
		"created_at": doc.CreatedAt,
	}, nil
}

// CreateRequirementTool creates a requirement document
type CreateRequirementTool struct {
	ArtifactPath string
	Author       string
}

func (t *CreateRequirementTool) Name() string        { return "create_requirement" }
func (t *CreateRequirementTool) Description() string { return "Create a business requirement" }

func (t *CreateRequirementTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	title, ok := args["title"].(string)
	if !ok || title == "" {
		return nil, fmt.Errorf("title is required")
	}

	description, ok := args["description"].(string)
	if !ok || description == "" {
		return nil, fmt.Errorf("description is required")
	}

	req := &Requirement{
		ID:          fmt.Sprintf("REQ-%d", time.Now().Unix()),
		Title:       title,
		Description: description,
		Type:        "functional",
		Priority:    "medium",
		Status:      "proposed",
		Author:      t.Author,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if rt, ok := args["type"].(string); ok {
		req.Type = rt
	}
	if priority, ok := args["priority"].(string); ok {
		req.Priority = priority
	}
	if rationale, ok := args["rationale"].(string); ok {
		req.Rationale = rationale
	}
	if criteria, ok := args["acceptance_criteria"].([]interface{}); ok {
		for _, c := range criteria {
			if s, ok := c.(string); ok {
				req.Criteria = append(req.Criteria, s)
			}
		}
	}

	// Save requirement
	reqPath := filepath.Join(t.ArtifactPath, "requirements", req.ID+".json")
	os.MkdirAll(filepath.Dir(reqPath), 0755)
	data, _ := json.MarshalIndent(req, "", "  ")
	if err := os.WriteFile(reqPath, data, 0644); err != nil {
		return nil, fmt.Errorf("save requirement: %w", err)
	}

	// Also create markdown version
	mdPath := filepath.Join(t.ArtifactPath, "requirements", req.ID+".md")
	mdContent := fmt.Sprintf(`# %s

**ID:** %s
**Type:** %s
**Priority:** %s
**Status:** %s
**Author:** %s

## Description

%s
`, req.Title, req.ID, req.Type, req.Priority, req.Status, req.Author, req.Description)

	if req.Rationale != "" {
		mdContent += fmt.Sprintf("\n## Rationale\n\n%s\n", req.Rationale)
	}

	if len(req.Criteria) > 0 {
		mdContent += "\n## Acceptance Criteria\n\n"
		for i, c := range req.Criteria {
			mdContent += fmt.Sprintf("%d. %s\n", i+1, c)
		}
	}

	os.WriteFile(mdPath, []byte(mdContent), 0644)

	return map[string]interface{}{
		"id":         req.ID,
		"title":      req.Title,
		"type":       req.Type,
		"priority":   req.Priority,
		"status":     req.Status,
		"path":       reqPath,
		"created_at": req.CreatedAt,
	}, nil
}

// CreateSpecTool creates a technical specification
type CreateSpecTool struct {
	ArtifactPath string
	Author       string
}

func (t *CreateSpecTool) Name() string        { return "create_spec" }
func (t *CreateSpecTool) Description() string { return "Create a technical specification" }

func (t *CreateSpecTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	title, ok := args["title"].(string)
	if !ok || title == "" {
		return nil, fmt.Errorf("title is required")
	}

	description, ok := args["description"].(string)
	if !ok || description == "" {
		return nil, fmt.Errorf("description is required")
	}

	spec := &Spec{
		ID:          fmt.Sprintf("SPEC-%d", time.Now().Unix()),
		Title:       title,
		Description: description,
		Author:      t.Author,
		Version:     "1.0",
		Status:      "draft",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if reqs, ok := args["requirements"].([]interface{}); ok {
		for _, r := range reqs {
			if s, ok := r.(string); ok {
				spec.Requirements = append(spec.Requirements, s)
			}
		}
	}
	if comps, ok := args["components"].([]interface{}); ok {
		for _, c := range comps {
			if s, ok := c.(string); ok {
				spec.Components = append(spec.Components, s)
			}
		}
	}
	if interfaces, ok := args["interfaces"].(map[string]interface{}); ok {
		spec.Interfaces = interfaces
	}
	if models, ok := args["data_models"].(map[string]interface{}); ok {
		spec.DataModels = models
	}

	// Save spec
	specPath := filepath.Join(t.ArtifactPath, "specs", spec.ID+".json")
	spec.Path = specPath
	os.MkdirAll(filepath.Dir(specPath), 0755)
	data, _ := json.MarshalIndent(spec, "", "  ")
	if err := os.WriteFile(specPath, data, 0644); err != nil {
		return nil, fmt.Errorf("save spec: %w", err)
	}

	// Create markdown version
	mdPath := filepath.Join(t.ArtifactPath, "specs", spec.ID+".md")
	mdContent := fmt.Sprintf(`# %s

**ID:** %s
**Version:** %s
**Status:** %s
**Author:** %s
**Created:** %s

## Overview

%s
`, spec.Title, spec.ID, spec.Version, spec.Status, spec.Author, spec.CreatedAt.Format(time.RFC3339), spec.Description)

	if len(spec.Requirements) > 0 {
		mdContent += "\n## Related Requirements\n\n"
		for _, r := range spec.Requirements {
			mdContent += fmt.Sprintf("- %s\n", r)
		}
	}

	if len(spec.Components) > 0 {
		mdContent += "\n## Components\n\n"
		for _, c := range spec.Components {
			mdContent += fmt.Sprintf("- %s\n", c)
		}
	}

	os.WriteFile(mdPath, []byte(mdContent), 0644)

	return map[string]interface{}{
		"id":         spec.ID,
		"title":      spec.Title,
		"version":    spec.Version,
		"status":     spec.Status,
		"path":       specPath,
		"created_at": spec.CreatedAt,
	}, nil
}

// sanitizeFilename removes unsafe characters from a filename
func sanitizeFilename(name string) string {
	// Replace spaces and special characters
	result := ""
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			result += string(r)
		} else if r == ' ' {
			result += "-"
		}
	}
	return result
}
