package templates

import (
	"strings"
	"testing"
)

func TestList(t *testing.T) {
	names, err := List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(names) == 0 {
		t.Fatal("Expected at least one template")
	}

	// Check for expected templates
	expected := []string{"dev-team", "trading-team", "research-team"}
	for _, exp := range expected {
		found := false
		for _, name := range names {
			if name == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected template '%s' not found", exp)
		}
	}
}

func TestGet(t *testing.T) {
	content, err := Get("dev-team")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if len(content) == 0 {
		t.Fatal("Template content is empty")
	}

	// Check for expected content
	contentStr := string(content)
	if !strings.Contains(contentStr, "apiVersion: ugudu/v1") {
		t.Error("Template missing apiVersion")
	}

	if !strings.Contains(contentStr, "kind: Team") {
		t.Error("Template missing kind")
	}

	if !strings.Contains(contentStr, "name: dev-team") {
		t.Error("Template missing name")
	}
}

func TestGetNonExistent(t *testing.T) {
	_, err := Get("non-existent-template")
	if err == nil {
		t.Error("Expected error for non-existent template")
	}
}

func TestExists(t *testing.T) {
	if !Exists("dev-team") {
		t.Error("dev-team should exist")
	}

	if Exists("non-existent") {
		t.Error("non-existent should not exist")
	}
}

func TestGetFS(t *testing.T) {
	fs := GetFS()
	if fs == nil {
		t.Fatal("GetFS returned nil")
	}
}

func TestTemplateContent(t *testing.T) {
	templates := []string{"dev-team", "trading-team", "research-team"}

	for _, name := range templates {
		t.Run(name, func(t *testing.T) {
			content, err := Get(name)
			if err != nil {
				t.Fatalf("Failed to get template %s: %v", name, err)
			}

			contentStr := string(content)

			// All templates should have these fields
			requiredFields := []string{
				"apiVersion:",
				"kind: Team",
				"metadata:",
				"name:",
				"client_facing:",
				"roles:",
			}

			for _, field := range requiredFields {
				if !strings.Contains(contentStr, field) {
					t.Errorf("Template %s missing required field: %s", name, field)
				}
			}
		})
	}
}
