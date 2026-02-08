package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const openaiAPIURL = "https://api.openai.com/v1"

// OpenAI implements the Provider interface for OpenAI's API
type OpenAI struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewOpenAI creates a new OpenAI provider
func NewOpenAI(apiKey, baseURL string) *OpenAI {
	if baseURL == "" {
		baseURL = openaiAPIURL
	}
	return &OpenAI{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{},
	}
}

func (o *OpenAI) ID() string   { return "openai" }
func (o *OpenAI) Name() string { return "OpenAI" }

func (o *OpenAI) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	openaiReq := o.convertRequest(req)

	body, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var openaiResp openaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return o.convertResponse(&openaiResp), nil
}

func (o *OpenAI) Stream(ctx context.Context, req *ChatRequest) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk)

	openaiReq := o.convertRequest(req)
	openaiReq["stream"] = true

	body, err := json.Marshal(openaiReq)
	if err != nil {
		close(ch)
		return ch, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		close(ch)
		return ch, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)

	go func() {
		defer close(ch)

		resp, err := o.client.Do(httpReq)
		if err != nil {
			ch <- StreamChunk{Error: err}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			ch <- StreamChunk{Error: fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes))}
			return
		}

		// Parse SSE stream
		buf := make([]byte, 1024)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				n, err := resp.Body.Read(buf)
				if err != nil {
					if err == io.EOF {
						ch <- StreamChunk{Done: true}
						return
					}
					ch <- StreamChunk{Error: err}
					return
				}

				// Parse "data: " prefixed lines
				data := string(buf[:n])
				if data == "data: [DONE]\n\n" {
					ch <- StreamChunk{Done: true}
					return
				}

				// Simple extraction of content delta
				var chunk openaiStreamChunk
				if err := json.Unmarshal(buf[:n], &chunk); err == nil {
					if len(chunk.Choices) > 0 {
						ch <- StreamChunk{
							Content: chunk.Choices[0].Delta.Content,
						}
					}
				}
			}
		}
	}()

	return ch, nil
}

func (o *OpenAI) ListModels(ctx context.Context) ([]ModelInfo, error) {
	return []ModelInfo{
		{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", MaxTokens: 128000},
		{ID: "gpt-4o-mini", Name: "GPT-4o Mini", Provider: "openai", MaxTokens: 128000},
		{ID: "gpt-4-turbo", Name: "GPT-4 Turbo", Provider: "openai", MaxTokens: 128000},
		{ID: "o1", Name: "o1", Provider: "openai", MaxTokens: 200000},
		{ID: "o1-mini", Name: "o1 Mini", Provider: "openai", MaxTokens: 128000},
	}, nil
}

func (o *OpenAI) Ping(ctx context.Context) error {
	req := &ChatRequest{
		Model:    "gpt-4o-mini",
		Messages: []Message{{Role: "user", Content: "ping"}},
	}
	maxTokens := 1
	req.MaxTokens = &maxTokens

	_, err := o.Chat(ctx, req)
	return err
}

// OpenAI-specific types
type openaiResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []openaiChoice `json:"choices"`
	Usage   openaiUsage    `json:"usage"`
}

type openaiChoice struct {
	Index        int           `json:"index"`
	Message      openaiMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type openaiMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content"`
	ToolCalls  []openaiToolCall `json:"tool_calls,omitempty"`
}

type openaiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type openaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openaiStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

func (o *OpenAI) convertRequest(req *ChatRequest) map[string]interface{} {
	messages := make([]map[string]interface{}, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		}
	}

	result := map[string]interface{}{
		"model":    req.Model,
		"messages": messages,
	}

	if req.MaxTokens != nil {
		result["max_tokens"] = *req.MaxTokens
	}

	if req.Temperature != nil {
		result["temperature"] = *req.Temperature
	}

	if len(req.Stop) > 0 {
		result["stop"] = req.Stop
	}

	if len(req.Tools) > 0 {
		tools := make([]map[string]interface{}, len(req.Tools))
		for i, t := range req.Tools {
			tools[i] = map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        t.Name,
					"description": t.Description,
					"parameters": map[string]interface{}{
						"type":       "object",
						"properties": t.Parameters,
					},
				},
			}
		}
		result["tools"] = tools
	}

	return result
}

func (o *OpenAI) convertResponse(resp *openaiResponse) *ChatResponse {
	if len(resp.Choices) == 0 {
		return &ChatResponse{Provider: "openai", Model: resp.Model}
	}

	choice := resp.Choices[0]
	var toolCalls []ToolCall
	for _, tc := range choice.Message.ToolCalls {
		toolCalls = append(toolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}

	return &ChatResponse{
		Content:      choice.Message.Content,
		ToolCalls:    toolCalls,
		Model:        resp.Model,
		Provider:     "openai",
		FinishReason: choice.FinishReason,
		Usage: Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}
}
