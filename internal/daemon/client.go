// Package daemon provides the client for connecting to the Ugudu daemon
package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Client connects to the Ugudu daemon
type Client struct {
	socketPath string
	httpClient *http.Client
	baseURL    string
}

// NewClient creates a new daemon client
func NewClient(socketPath string) (*Client, error) {
	// Determine socket path
	if socketPath == "" {
		socketPath = findSocket()
	}

	if socketPath == "" {
		return nil, fmt.Errorf("daemon not running (no socket found)")
	}

	// Create HTTP client that uses Unix socket - long timeout for multi-agent tasks
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
		Timeout: 600 * time.Second,
	}

	return &Client{
		socketPath: socketPath,
		httpClient: httpClient,
		baseURL:    "http://unix", // Fake host, routed via Unix socket
	}, nil
}

// NewRemoteClient creates a client that connects via TCP
func NewRemoteClient(addr string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 600 * time.Second},
		baseURL:    "http://" + addr,
	}
}

// findSocket looks for the daemon socket in common locations
func findSocket() string {
	// Check environment variable first
	if sock := os.Getenv("UGUDU_SOCKET"); sock != "" {
		if _, err := os.Stat(sock); err == nil {
			return sock
		}
	}

	// Check system path
	if _, err := os.Stat(DefaultSocketPath); err == nil {
		return DefaultSocketPath
	}

	// Check user home
	if home, err := os.UserHomeDir(); err == nil {
		userSock := filepath.Join(home, UserSocketPath)
		if _, err := os.Stat(userSock); err == nil {
			return userSock
		}
	}

	return ""
}

// Ping checks if daemon is responsive
func (c *Client) Ping(ctx context.Context) error {
	resp, err := c.get(ctx, "/api/health")
	if err != nil {
		return fmt.Errorf("daemon not responding: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("daemon returned status %d", resp.StatusCode)
	}
	return nil
}

// Status returns daemon and manager status
func (c *Client) Status(ctx context.Context) (map[string]interface{}, error) {
	resp, err := c.get(ctx, "/api/status")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

// ListTeams returns all teams
func (c *Client) ListTeams(ctx context.Context) ([]map[string]interface{}, error) {
	resp, err := c.get(ctx, "/api/teams")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Teams []map[string]interface{} `json:"teams"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Teams, nil
}

// CreateTeam creates a team from a spec file
func (c *Client) CreateTeam(ctx context.Context, specPath string) (map[string]interface{}, error) {
	body := map[string]string{"spec_path": specPath}
	resp, err := c.post(ctx, "/api/teams", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if errMsg, ok := result["error"].(string); ok && errMsg != "" {
		return nil, fmt.Errorf("%s", errMsg)
	}

	return result, nil
}

// GetTeam returns a team by name
func (c *Client) GetTeam(ctx context.Context, name string) (map[string]interface{}, error) {
	resp, err := c.get(ctx, "/api/teams/"+name)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if errMsg, ok := result["error"].(string); ok && errMsg != "" {
		return nil, fmt.Errorf("%s", errMsg)
	}

	return result, nil
}

// StartTeam starts a team
func (c *Client) StartTeam(ctx context.Context, name string) error {
	resp, err := c.post(ctx, "/api/teams/"+name+"/start", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if errMsg, ok := result["error"].(string); ok && errMsg != "" {
		return fmt.Errorf("%s", errMsg)
	}

	return nil
}

// StopTeam stops a team
func (c *Client) StopTeam(ctx context.Context, name string) error {
	resp, err := c.post(ctx, "/api/teams/"+name+"/stop", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if errMsg, ok := result["error"].(string); ok && errMsg != "" {
		return fmt.Errorf("%s", errMsg)
	}

	return nil
}

// DeleteTeam deletes a team
func (c *Client) DeleteTeam(ctx context.Context, name string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", c.baseURL+"/api/teams/"+name, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if errMsg, ok := result["error"].(string); ok && errMsg != "" {
		return fmt.Errorf("%s", errMsg)
	}

	return nil
}

// TeamMembers returns members of a team
func (c *Client) TeamMembers(ctx context.Context, name string) ([]map[string]interface{}, error) {
	resp, err := c.get(ctx, "/api/teams/"+name+"/members")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Members []map[string]interface{} `json:"members"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Members, nil
}

// Chat sends a message to a team
func (c *Client) Chat(ctx context.Context, team, message, to string) ([]map[string]interface{}, error) {
	body := map[string]string{
		"team":    team,
		"message": message,
	}
	if to != "" {
		body["to"] = to
	}

	resp, err := c.post(ctx, "/api/chat", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Responses []map[string]interface{} `json:"responses"`
		Error     string                   `json:"error,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Error != "" {
		return nil, fmt.Errorf("%s", result.Error)
	}

	return result.Responses, nil
}

// ListProviders returns available providers
func (c *Client) ListProviders(ctx context.Context) ([]map[string]interface{}, error) {
	resp, err := c.get(ctx, "/api/providers")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Providers []map[string]interface{} `json:"providers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Providers, nil
}

// TestProvider tests provider connectivity
func (c *Client) TestProvider(ctx context.Context, id string) error {
	resp, err := c.get(ctx, "/api/providers/"+id+"/test")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if status, ok := result["status"].(string); ok && status == "error" {
		if errMsg, ok := result["error"].(string); ok {
			return fmt.Errorf("%s", errMsg)
		}
	}

	return nil
}

// ProviderModels returns available models for a provider
func (c *Client) ProviderModels(ctx context.Context, id string) ([]string, error) {
	resp, err := c.get(ctx, "/api/providers/"+id+"/models")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Models []string `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Models, nil
}

// GetSocketPath returns the socket path being used
func (c *Client) GetSocketPath() string {
	return c.socketPath
}

// ============================================================================
// Project Methods
// ============================================================================

// ListProjects returns all projects
func (c *Client) ListProjects(ctx context.Context) ([]map[string]interface{}, error) {
	resp, err := c.get(ctx, "/api/projects")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Projects []map[string]interface{} `json:"projects"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Projects, nil
}

// CreateProject creates a new project
func (c *Client) CreateProject(ctx context.Context, name, sourcePath, team string, sharedPaths []string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"name":        name,
		"source_path": sourcePath,
		"team":        team,
	}
	if len(sharedPaths) > 0 {
		body["shared_paths"] = sharedPaths
	}

	resp, err := c.post(ctx, "/api/projects", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if errMsg, ok := result["error"].(string); ok && errMsg != "" {
		return nil, fmt.Errorf("%s", errMsg)
	}

	return result, nil
}

// GetProject returns a project by name
func (c *Client) GetProject(ctx context.Context, name string) (map[string]interface{}, error) {
	resp, err := c.get(ctx, "/api/projects/"+name)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if errMsg, ok := result["error"].(string); ok && errMsg != "" {
		return nil, fmt.Errorf("%s", errMsg)
	}

	return result, nil
}

// DeleteProject deletes a project
func (c *Client) DeleteProject(ctx context.Context, name string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", c.baseURL+"/api/projects/"+name, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if errMsg, ok := result["error"].(string); ok && errMsg != "" {
		return fmt.Errorf("%s", errMsg)
	}

	return nil
}

// ProjectStandup gets a standup report for a project
func (c *Client) ProjectStandup(ctx context.Context, name, period string) (map[string]interface{}, error) {
	path := "/api/projects/" + name + "/standup"
	if period != "" {
		path += "?period=" + period
	}

	resp, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if errMsg, ok := result["error"].(string); ok && errMsg != "" {
		return nil, fmt.Errorf("%s", errMsg)
	}

	return result, nil
}

// ProjectActivity gets activity for a project
func (c *Client) ProjectActivity(ctx context.Context, name string, limit int, since string, actType string) ([]map[string]interface{}, error) {
	path := "/api/projects/" + name + "/activity?"
	if limit > 0 {
		path += fmt.Sprintf("limit=%d&", limit)
	}
	if since != "" {
		path += "since=" + since + "&"
	}
	if actType != "" {
		path += "type=" + actType
	}

	resp, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Entries []map[string]interface{} `json:"entries"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Entries, nil
}

// ProjectTasks gets tasks for a project
func (c *Client) ProjectTasks(ctx context.Context, name string) ([]map[string]interface{}, error) {
	resp, err := c.get(ctx, "/api/projects/"+name+"/tasks")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Tasks []map[string]interface{} `json:"tasks"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Tasks, nil
}

// ============================================================================
// Conversation Methods
// ============================================================================

// ListConversations returns recent conversations for a team
func (c *Client) ListConversations(ctx context.Context, teamName string, limit int) ([]map[string]interface{}, error) {
	path := fmt.Sprintf("/api/teams/%s/conversations?limit=%d", teamName, limit)
	resp, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Conversations []map[string]interface{} `json:"conversations"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Conversations, nil
}

// GetConversationHistory returns messages from a conversation
func (c *Client) GetConversationHistory(ctx context.Context, conversationID string) ([]map[string]interface{}, error) {
	resp, err := c.get(ctx, "/api/conversations/"+conversationID)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Messages []map[string]interface{} `json:"messages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Messages, nil
}

// ClearConversation clears conversation history for a team
func (c *Client) ClearConversation(ctx context.Context, teamName string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", c.baseURL+"/api/teams/"+teamName+"/conversations", nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if errMsg, ok := result["error"].(string); ok && errMsg != "" {
		return fmt.Errorf("%s", errMsg)
	}

	return nil
}

// SetTokenMode sets the token consumption mode for a team
func (c *Client) SetTokenMode(ctx context.Context, teamName, mode string) error {
	body := map[string]interface{}{
		"mode": mode,
	}

	resp, err := c.post(ctx, "/api/teams/"+teamName+"/token-mode", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if errMsg, ok := result["error"].(string); ok && errMsg != "" {
		return fmt.Errorf("%s", errMsg)
	}

	return nil
}

// ============================================================================
// Project Workflow Methods (Orchestrator)
// ============================================================================

// StartProject starts a new collaborative project with a team
func (c *Client) StartProject(ctx context.Context, team, request string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"request": request,
	}

	resp, err := c.post(ctx, "/api/teams/"+team+"/project/start", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if errMsg, ok := result["error"].(string); ok && errMsg != "" {
		return nil, fmt.Errorf("%s", errMsg)
	}

	return result, nil
}

// ProjectStatus returns the status of a team's active project
func (c *Client) ProjectStatus(ctx context.Context, team string) (map[string]interface{}, error) {
	resp, err := c.get(ctx, "/api/teams/"+team+"/project/status")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if errMsg, ok := result["error"].(string); ok && errMsg != "" {
		return nil, fmt.Errorf("%s", errMsg)
	}

	return result, nil
}

// PendingQuestions returns questions awaiting client response
func (c *Client) PendingQuestions(ctx context.Context, team string) ([]interface{}, error) {
	resp, err := c.get(ctx, "/api/teams/"+team+"/project/questions")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Questions []interface{} `json:"questions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Questions, nil
}

// AnswerQuestion provides an answer to a pending question
func (c *Client) AnswerQuestion(ctx context.Context, team, questionID, answer string) error {
	body := map[string]interface{}{
		"question_id": questionID,
		"answer":      answer,
	}

	resp, err := c.post(ctx, "/api/teams/"+team+"/project/answer", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if errMsg, ok := result["error"].(string); ok && errMsg != "" {
		return fmt.Errorf("%s", errMsg)
	}

	return nil
}

// ProjectCommunications returns the communication log for a project
func (c *Client) ProjectCommunications(ctx context.Context, team string, limit int) ([]interface{}, error) {
	path := fmt.Sprintf("/api/teams/%s/project/communications?limit=%d", team, limit)
	resp, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Communications []interface{} `json:"communications"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Communications, nil
}

// ============================================================================
// HTTP Helpers
// ============================================================================

func (c *Client) get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	return c.httpClient.Do(req)
}

func (c *Client) post(ctx context.Context, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.httpClient.Do(req)
}
