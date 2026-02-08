package tools

// ToolCategory represents a category of tools
type ToolCategory string

const (
	// CategoryFileSystem includes file read/write/edit/list operations
	CategoryFileSystem ToolCategory = "filesystem"
	// CategoryCommand includes command execution
	CategoryCommand ToolCategory = "command"
	// CategoryGit includes git operations
	CategoryGit ToolCategory = "git"
	// CategoryPlanning includes task/project management
	CategoryPlanning ToolCategory = "planning"
	// CategoryTesting includes test execution and bug reporting
	CategoryTesting ToolCategory = "testing"
	// CategoryDocumentation includes documentation and spec creation
	CategoryDocumentation ToolCategory = "documentation"
	// CategoryCommunication includes team communication tools
	CategoryCommunication ToolCategory = "communication"
	// CategoryHTTP includes HTTP/API operations
	CategoryHTTP ToolCategory = "http"
)

// RoleToolMapping defines which tool categories each role can access
var RoleToolMapping = map[string][]ToolCategory{
	"engineer": {
		CategoryFileSystem,
		CategoryCommand,
		CategoryGit,
		CategoryCommunication,
	},
	"pm": {
		CategoryPlanning,
		CategoryCommunication,
	},
	"qa": {
		CategoryTesting,
		CategoryCommand,
		CategoryCommunication,
	},
	"ba": {
		CategoryDocumentation,
		CategoryCommunication,
	},
	// Default allows all categories
	"default": {
		CategoryFileSystem,
		CategoryCommand,
		CategoryGit,
		CategoryPlanning,
		CategoryTesting,
		CategoryDocumentation,
		CategoryCommunication,
		CategoryHTTP,
	},
}

// ToolCategoryMapping maps tool names to their categories
var ToolCategoryMapping = map[string]ToolCategory{
	// FileSystem tools
	"read_file":    CategoryFileSystem,
	"write_file":   CategoryFileSystem,
	"edit_file":    CategoryFileSystem,
	"list_files":   CategoryFileSystem,
	"search_files": CategoryFileSystem,

	// Command tools
	"run_command": CategoryCommand,

	// Git tools
	"git_status": CategoryGit,
	"git_diff":   CategoryGit,
	"git_commit": CategoryGit,
	"git_log":    CategoryGit,
	"git_branch": CategoryGit,

	// Planning tools
	"create_task":   CategoryPlanning,
	"update_task":   CategoryPlanning,
	"list_tasks":    CategoryPlanning,
	"assign_task":   CategoryPlanning,
	"create_report": CategoryPlanning,
	"delegate_task": CategoryPlanning,

	// Testing tools
	"run_tests":        CategoryTesting,
	"create_bug_report": CategoryTesting,
	"verify_fix":       CategoryTesting,
	"list_test_results": CategoryTesting,

	// Documentation tools
	"create_doc":         CategoryDocumentation,
	"create_requirement": CategoryDocumentation,
	"create_spec":        CategoryDocumentation,

	// Communication tools
	"ask_colleague":    CategoryCommunication,
	"report_progress":  CategoryCommunication,

	// HTTP tools
	"http_request": CategoryHTTP,
}

// GetRoleCategories returns the tool categories allowed for a role
func GetRoleCategories(role string) []ToolCategory {
	if categories, ok := RoleToolMapping[role]; ok {
		return categories
	}
	return RoleToolMapping["default"]
}

// IsToolAllowedForRole checks if a specific tool is allowed for a role
func IsToolAllowedForRole(toolName string, role string) bool {
	category, ok := ToolCategoryMapping[toolName]
	if !ok {
		// Unknown tools are allowed by default
		return true
	}

	allowedCategories := GetRoleCategories(role)
	for _, c := range allowedCategories {
		if c == category {
			return true
		}
	}

	return false
}

// GetAllowedToolsForRole returns a list of tool names allowed for a role
func GetAllowedToolsForRole(role string) []string {
	allowedCategories := GetRoleCategories(role)
	categorySet := make(map[ToolCategory]bool)
	for _, c := range allowedCategories {
		categorySet[c] = true
	}

	var allowedTools []string
	for tool, category := range ToolCategoryMapping {
		if categorySet[category] {
			allowedTools = append(allowedTools, tool)
		}
	}

	return allowedTools
}
