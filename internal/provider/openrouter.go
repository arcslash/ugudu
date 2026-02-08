package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const openrouterAPIURL = "https://openrouter.ai/api/v1"

// OpenRouter implements the Provider interface for OpenRouter's API
// OpenRouter provides access to many models including Claude, GPT, Gemini, Mistral, DeepSeek, etc.
type OpenRouter struct {
	apiKey   string
	baseURL  string
	client   *http.Client
	siteName string // For OpenRouter attribution
	siteURL  string // For OpenRouter attribution
}

// NewOpenRouter creates a new OpenRouter provider
func NewOpenRouter(apiKey, siteName, siteURL string) *OpenRouter {
	if siteName == "" {
		siteName = "Ugudu"
	}
	return &OpenRouter{
		apiKey:   apiKey,
		baseURL:  openrouterAPIURL,
		client:   &http.Client{},
		siteName: siteName,
		siteURL:  siteURL,
	}
}

func (o *OpenRouter) ID() string   { return "openrouter" }
func (o *OpenRouter) Name() string { return "OpenRouter" }

func (o *OpenRouter) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
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
	httpReq.Header.Set("HTTP-Referer", o.siteURL)
	httpReq.Header.Set("X-Title", o.siteName)

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

func (o *OpenRouter) Stream(ctx context.Context, req *ChatRequest) (<-chan StreamChunk, error) {
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
	httpReq.Header.Set("HTTP-Referer", o.siteURL)
	httpReq.Header.Set("X-Title", o.siteName)

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

				data := string(buf[:n])
				if data == "data: [DONE]\n\n" {
					ch <- StreamChunk{Done: true}
					return
				}

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

func (o *OpenRouter) ListModels(ctx context.Context) ([]ModelInfo, error) {
	// OpenRouter supports many models - here are the most popular ones
	return []ModelInfo{
		// Anthropic Claude
		{ID: "anthropic/claude-3.5-sonnet", Name: "Claude 3.5 Sonnet", Provider: "openrouter", MaxTokens: 200000},
		{ID: "anthropic/claude-3-opus", Name: "Claude 3 Opus", Provider: "openrouter", MaxTokens: 200000},
		{ID: "anthropic/claude-3-sonnet", Name: "Claude 3 Sonnet", Provider: "openrouter", MaxTokens: 200000},
		{ID: "anthropic/claude-3-haiku", Name: "Claude 3 Haiku", Provider: "openrouter", MaxTokens: 200000},
		// OpenAI
		{ID: "openai/gpt-4o", Name: "GPT-4o", Provider: "openrouter", MaxTokens: 128000},
		{ID: "openai/gpt-4o-mini", Name: "GPT-4o Mini", Provider: "openrouter", MaxTokens: 128000},
		{ID: "openai/o1", Name: "o1", Provider: "openrouter", MaxTokens: 200000},
		// Google
		{ID: "google/gemini-pro-1.5", Name: "Gemini 1.5 Pro", Provider: "openrouter", MaxTokens: 1000000},
		{ID: "google/gemini-flash-1.5", Name: "Gemini 1.5 Flash", Provider: "openrouter", MaxTokens: 1000000},
		// Mistral
		{ID: "mistralai/mistral-large", Name: "Mistral Large", Provider: "openrouter", MaxTokens: 128000},
		{ID: "mistralai/mistral-medium", Name: "Mistral Medium", Provider: "openrouter", MaxTokens: 32000},
		{ID: "mistralai/mixtral-8x7b-instruct", Name: "Mixtral 8x7B", Provider: "openrouter", MaxTokens: 32000},
		// DeepSeek
		{ID: "deepseek/deepseek-coder", Name: "DeepSeek Coder", Provider: "openrouter", MaxTokens: 16000},
		{ID: "deepseek/deepseek-chat", Name: "DeepSeek Chat", Provider: "openrouter", MaxTokens: 32000},
		// Meta Llama
		{ID: "meta-llama/llama-3.1-405b-instruct", Name: "Llama 3.1 405B", Provider: "openrouter", MaxTokens: 128000},
		{ID: "meta-llama/llama-3.1-70b-instruct", Name: "Llama 3.1 70B", Provider: "openrouter", MaxTokens: 128000},
		// Cohere
		{ID: "cohere/command-r-plus", Name: "Command R+", Provider: "openrouter", MaxTokens: 128000},
	}, nil
}

func (o *OpenRouter) Ping(ctx context.Context) error {
	req := &ChatRequest{
		Model:    "openai/gpt-4o-mini",
		Messages: []Message{{Role: "user", Content: "ping"}},
	}
	maxTokens := 1
	req.MaxTokens = &maxTokens

	_, err := o.Chat(ctx, req)
	return err
}

func (o *OpenRouter) convertRequest(req *ChatRequest) map[string]interface{} {
	messages := make([]map[string]interface{}, len(req.Messages))
	for i, msg := range req.Messages {
		m := map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		}

		// Handle tool results
		if msg.Role == "tool" && msg.ToolCallID != "" {
			m["role"] = "tool"
			m["tool_call_id"] = msg.ToolCallID
		}

		// Handle assistant messages with tool calls
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			toolCalls := make([]map[string]interface{}, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				toolCalls[j] = map[string]interface{}{
					"id":   tc.ID,
					"type": "function",
					"function": map[string]interface{}{
						"name":      tc.Name,
						"arguments": tc.Arguments,
					},
				}
			}
			m["tool_calls"] = toolCalls
		}

		messages[i] = m
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

func (o *OpenRouter) convertResponse(resp *openaiResponse) *ChatResponse {
	if len(resp.Choices) == 0 {
		return &ChatResponse{Provider: "openrouter", Model: resp.Model}
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
		Provider:     "openrouter",
		FinishReason: choice.FinishReason,
		Usage: Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}
}
