package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	anthropicAPIURL  = "https://api.anthropic.com/v1"
	anthropicVersion = "2023-06-01"
)

// Anthropic implements the Provider interface for Anthropic's Claude API
type Anthropic struct {
	apiKey  string
	baseURL string
	client  *http.Client

	// Rate limiting
	rateLimitState *RateLimitState
	requestQueue   *RequestQueue
	autoResume     bool
	resumeWorker   bool
	workerMu       sync.Mutex
	stopWorker     chan struct{}

	// Callbacks
	onRateLimited func(RateLimitInfo)
	onResume      func()
}

// AnthropicOption configures the Anthropic provider
type AnthropicOption func(*Anthropic)

// WithAutoResume enables automatic retry when rate limits clear
func WithAutoResume(enabled bool) AnthropicOption {
	return func(a *Anthropic) {
		a.autoResume = enabled
	}
}

// WithRateLimitCallback sets a callback for rate limit events
func WithRateLimitCallback(cb func(RateLimitInfo)) AnthropicOption {
	return func(a *Anthropic) {
		a.onRateLimited = cb
	}
}

// WithResumeCallback sets a callback for when rate limits clear
func WithResumeCallback(cb func()) AnthropicOption {
	return func(a *Anthropic) {
		a.onResume = cb
	}
}

// NewAnthropic creates a new Anthropic provider
func NewAnthropic(apiKey, baseURL string, opts ...AnthropicOption) *Anthropic {
	if baseURL == "" {
		baseURL = anthropicAPIURL
	}

	a := &Anthropic{
		apiKey:         apiKey,
		baseURL:        baseURL,
		client:         &http.Client{Timeout: 120 * time.Second},
		rateLimitState: NewRateLimitState(),
		requestQueue:   NewRequestQueue(100),
		autoResume:     true, // Enabled by default
		stopWorker:     make(chan struct{}),
	}

	for _, opt := range opts {
		opt(a)
	}

	// Set up callbacks
	if a.onRateLimited != nil {
		a.rateLimitState.OnRateLimited(a.onRateLimited)
	}
	if a.onResume != nil {
		a.rateLimitState.OnResume(a.onResume)
	}

	return a
}

func (a *Anthropic) ID() string   { return "anthropic" }
func (a *Anthropic) Name() string { return "Anthropic Claude" }

// RateLimitStatus returns current rate limit information
func (a *Anthropic) RateLimitStatus() *RateLimitState {
	return a.rateLimitState
}

// IsRateLimited returns true if currently rate limited
func (a *Anthropic) IsRateLimited() bool {
	return a.rateLimitState.IsRateLimited()
}

// TimeUntilResume returns duration until rate limit resets
func (a *Anthropic) TimeUntilResume() time.Duration {
	return a.rateLimitState.TimeUntilResume()
}

// PendingRequests returns the number of queued requests
func (a *Anthropic) PendingRequests() int {
	return a.requestQueue.Len()
}

func (a *Anthropic) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	// Check if we're currently rate limited
	if a.rateLimitState.IsRateLimited() {
		if a.autoResume {
			// Queue the request for later processing
			return a.queueAndWait(ctx, req)
		}
		return nil, &RateLimitError{
			Info:    a.getActiveRateLimit(),
			Message: "rate limited - retry after " + a.rateLimitState.GetResumeTime().Format(time.RFC3339),
		}
	}

	// Try the request
	resp, err := a.doChat(ctx, req)
	if err != nil {
		// Check if it's a rate limit error
		if info, ok := IsRateLimitError(err); ok {
			a.rateLimitState.RecordRateLimit(*info)
			a.startResumeWorker()

			if a.autoResume {
				// Queue and wait for resume
				return a.queueAndWait(ctx, req)
			}
		}
		return nil, err
	}

	// Clear rate limit if request succeeded
	if a.rateLimitState.IsRateLimited() {
		a.rateLimitState.ClearRateLimit()
	}

	return resp, nil
}

// doChat performs the actual API call
func (a *Anthropic) doChat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	anthropicReq := a.convertRequest(req)

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	// Check for rate limit
	if resp.StatusCode == http.StatusTooManyRequests {
		info := ParseAnthropicRateLimitError(resp.StatusCode, resp.Header, bodyBytes)
		return nil, &RateLimitError{
			Info:    info,
			Message: string(bodyBytes),
		}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var anthropicResp anthropicResponse
	if err := json.Unmarshal(bodyBytes, &anthropicResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return a.convertResponse(&anthropicResp, req.Model), nil
}

// queueAndWait queues a request and waits for the response
func (a *Anthropic) queueAndWait(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	pending := &PendingRequest{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Request:   req,
		Context:   ctx,
		Response:  make(chan *ChatResponse, 1),
		Error:     make(chan error, 1),
		CreatedAt: time.Now(),
	}

	if err := a.requestQueue.Add(pending); err != nil {
		return nil, fmt.Errorf("queue full: %w", err)
	}

	// Start resume worker if not running
	a.startResumeWorker()

	// Wait for result or context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-pending.Response:
		return resp, nil
	case err := <-pending.Error:
		return nil, err
	}
}

// startResumeWorker starts the background worker that processes queued requests
func (a *Anthropic) startResumeWorker() {
	a.workerMu.Lock()
	defer a.workerMu.Unlock()

	if a.resumeWorker {
		return // Already running
	}

	a.resumeWorker = true
	go a.resumeWorkerLoop()
}

// resumeWorkerLoop processes queued requests when rate limit clears
func (a *Anthropic) resumeWorkerLoop() {
	defer func() {
		a.workerMu.Lock()
		a.resumeWorker = false
		a.workerMu.Unlock()
	}()

	for {
		// Check if still rate limited
		if a.rateLimitState.IsRateLimited() {
			waitTime := a.rateLimitState.TimeUntilResume()
			if waitTime > 0 {
				// Add small buffer
				waitTime += 5 * time.Second

				select {
				case <-time.After(waitTime):
					// Check again
				case <-a.stopWorker:
					return
				}
				continue
			}
		}

		// Rate limit should be cleared now
		a.rateLimitState.ClearRateLimit()

		// Process pending requests
		for {
			pending := a.requestQueue.Pop()
			if pending == nil {
				break // Queue empty
			}

			// Check if context is still valid
			if pending.Context.Err() != nil {
				pending.Error <- pending.Context.Err()
				continue
			}

			// Try the request
			resp, err := a.doChat(pending.Context, pending.Request)
			if err != nil {
				if info, ok := IsRateLimitError(err); ok {
					// Rate limited again - requeue and wait
					a.rateLimitState.RecordRateLimit(*info)
					a.requestQueue.Add(pending)
					break // Exit inner loop, wait for rate limit to clear
				}
				pending.Error <- err
				continue
			}

			pending.Response <- resp
		}

		// If queue is empty and not rate limited, exit worker
		if a.requestQueue.Len() == 0 && !a.rateLimitState.IsRateLimited() {
			return
		}
	}
}

// getActiveRateLimit returns the most restrictive active rate limit
func (a *Anthropic) getActiveRateLimit() *RateLimitInfo {
	limits := a.rateLimitState.GetLimits()
	var latest *RateLimitInfo
	for _, info := range limits {
		if latest == nil || info.ResetAt.After(latest.ResetAt) {
			latest = info
		}
	}
	return latest
}

// Stop stops the resume worker
func (a *Anthropic) Stop() {
	select {
	case a.stopWorker <- struct{}{}:
	default:
	}
}

func (a *Anthropic) Stream(ctx context.Context, req *ChatRequest) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk)

	// Check if rate limited
	if a.rateLimitState.IsRateLimited() {
		close(ch)
		return ch, &RateLimitError{
			Info:    a.getActiveRateLimit(),
			Message: "rate limited - streaming not supported during rate limit",
		}
	}

	anthropicReq := a.convertRequest(req)
	anthropicReq["stream"] = true

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		close(ch)
		return ch, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		close(ch)
		return ch, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	go func() {
		defer close(ch)

		resp, err := a.client.Do(httpReq)
		if err != nil {
			ch <- StreamChunk{Error: err}
			return
		}
		defer resp.Body.Close()

		// Check for rate limit
		if resp.StatusCode == http.StatusTooManyRequests {
			bodyBytes, _ := io.ReadAll(resp.Body)
			info := ParseAnthropicRateLimitError(resp.StatusCode, resp.Header, bodyBytes)
			a.rateLimitState.RecordRateLimit(*info)
			a.startResumeWorker()
			ch <- StreamChunk{Error: &RateLimitError{Info: info, Message: string(bodyBytes)}}
			return
		}

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			ch <- StreamChunk{Error: fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes))}
			return
		}

		// Parse SSE stream
		decoder := json.NewDecoder(resp.Body)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				var event map[string]interface{}
				if err := decoder.Decode(&event); err != nil {
					if err == io.EOF {
						ch <- StreamChunk{Done: true}
						return
					}
					ch <- StreamChunk{Error: err}
					return
				}

				if content, ok := event["delta"].(map[string]interface{}); ok {
					if text, ok := content["text"].(string); ok {
						ch <- StreamChunk{Content: text}
					}
				}

				if event["type"] == "message_stop" {
					ch <- StreamChunk{Done: true}
					return
				}
			}
		}
	}()

	return ch, nil
}

func (a *Anthropic) ListModels(ctx context.Context) ([]ModelInfo, error) {
	return []ModelInfo{
		{ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4", Provider: "anthropic", MaxTokens: 8192},
		{ID: "claude-opus-4-20250514", Name: "Claude Opus 4", Provider: "anthropic", MaxTokens: 8192},
		{ID: "claude-3-5-haiku-20241022", Name: "Claude 3.5 Haiku", Provider: "anthropic", MaxTokens: 8192},
	}, nil
}

func (a *Anthropic) Ping(ctx context.Context) error {
	// Simple request to verify API key
	req := &ChatRequest{
		Model:    "claude-3-5-haiku-20241022",
		Messages: []Message{{Role: "user", Content: "ping"}},
	}
	maxTokens := 1
	req.MaxTokens = &maxTokens

	_, err := a.Chat(ctx, req)
	return err
}

// Anthropic-specific types
type anthropicResponse struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"`
	Role         string            `json:"role"`
	Content      []anthropicContent `json:"content"`
	Model        string            `json:"model"`
	StopReason   string            `json:"stop_reason"`
	StopSequence string            `json:"stop_sequence"`
	Usage        anthropicUsage    `json:"usage"`
}

type anthropicContent struct {
	Type  string `json:"type"`
	Text  string `json:"text,omitempty"`
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Input any    `json:"input,omitempty"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

func (a *Anthropic) convertRequest(req *ChatRequest) map[string]interface{} {
	// Convert messages - Anthropic has different format
	messages := make([]map[string]interface{}, 0)
	var systemPrompt string

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			systemPrompt = msg.Content
			continue
		}

		// Handle tool result messages
		if msg.Role == "tool" {
			messages = append(messages, map[string]interface{}{
				"role": "user",
				"content": []map[string]interface{}{
					{
						"type":        "tool_result",
						"tool_use_id": msg.ToolCallID,
						"content":     msg.Content,
					},
				},
			})
			continue
		}

		// Handle assistant messages with tool calls
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			content := make([]map[string]interface{}, 0)

			// Add text content if present
			if msg.Content != "" {
				content = append(content, map[string]interface{}{
					"type": "text",
					"text": msg.Content,
				})
			}

			// Add tool use blocks
			for _, tc := range msg.ToolCalls {
				var input interface{}
				json.Unmarshal([]byte(tc.Arguments), &input)
				content = append(content, map[string]interface{}{
					"type":  "tool_use",
					"id":    tc.ID,
					"name":  tc.Name,
					"input": input,
				})
			}

			messages = append(messages, map[string]interface{}{
				"role":    "assistant",
				"content": content,
			})
			continue
		}

		// Standard message
		messages = append(messages, map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}

	result := map[string]interface{}{
		"model":      req.Model,
		"messages":   messages,
		"max_tokens": 4096,
	}

	if systemPrompt != "" {
		result["system"] = systemPrompt
	}

	if req.MaxTokens != nil {
		result["max_tokens"] = *req.MaxTokens
	}

	if req.Temperature != nil {
		result["temperature"] = *req.Temperature
	}

	if len(req.Tools) > 0 {
		tools := make([]map[string]interface{}, len(req.Tools))
		for i, t := range req.Tools {
			// Use Parameters directly as input_schema - it already contains the full schema
			inputSchema := t.Parameters
			if inputSchema == nil {
				inputSchema = map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				}
			}

			tools[i] = map[string]interface{}{
				"name":         t.Name,
				"description":  t.Description,
				"input_schema": inputSchema,
			}
		}
		result["tools"] = tools
	}

	return result
}

func (a *Anthropic) convertResponse(resp *anthropicResponse, model string) *ChatResponse {
	var content string
	var toolCalls []ToolCall

	for _, c := range resp.Content {
		switch c.Type {
		case "text":
			content += c.Text
		case "tool_use":
			args, _ := json.Marshal(c.Input)
			toolCalls = append(toolCalls, ToolCall{
				ID:        c.ID,
				Name:      c.Name,
				Arguments: string(args),
			})
		}
	}

	return &ChatResponse{
		Content:      content,
		ToolCalls:    toolCalls,
		Model:        model,
		Provider:     "anthropic",
		FinishReason: resp.StopReason,
		Usage: Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}
}
