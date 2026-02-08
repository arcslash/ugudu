package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const groqAPIURL = "https://api.groq.com/openai/v1"

// Groq implements the Provider interface for Groq's API (OpenAI-compatible)
type Groq struct {
	apiKey string
	client *http.Client
}

// NewGroq creates a new Groq provider
func NewGroq(apiKey string) *Groq {
	return &Groq{
		apiKey: apiKey,
		client: &http.Client{},
	}
}

func (g *Groq) ID() string   { return "groq" }
func (g *Groq) Name() string { return "Groq" }

func (g *Groq) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	groqReq := g.convertRequest(req)

	body, err := json.Marshal(groqReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", groqAPIURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+g.apiKey)

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var groqResp groqResponse
	if err := json.NewDecoder(resp.Body).Decode(&groqResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return g.convertResponse(&groqResp), nil
}

func (g *Groq) Stream(ctx context.Context, req *ChatRequest) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk)

	groqReq := g.convertRequest(req)
	groqReq["stream"] = true

	body, err := json.Marshal(groqReq)
	if err != nil {
		close(ch)
		return ch, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", groqAPIURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		close(ch)
		return ch, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+g.apiKey)

	go func() {
		defer close(ch)

		resp, err := g.client.Do(httpReq)
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

		buf := make([]byte, 4096)
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

				var chunk groqStreamChunk
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

func (g *Groq) ListModels(ctx context.Context) ([]ModelInfo, error) {
	return []ModelInfo{
		{ID: "llama-3.3-70b-versatile", Name: "Llama 3.3 70B", Provider: "groq", MaxTokens: 32768},
		{ID: "llama-3.1-8b-instant", Name: "Llama 3.1 8B", Provider: "groq", MaxTokens: 8192},
		{ID: "mixtral-8x7b-32768", Name: "Mixtral 8x7B", Provider: "groq", MaxTokens: 32768},
		{ID: "gemma2-9b-it", Name: "Gemma 2 9B", Provider: "groq", MaxTokens: 8192},
	}, nil
}

func (g *Groq) Ping(ctx context.Context) error {
	req := &ChatRequest{
		Model:    "llama-3.1-8b-instant",
		Messages: []Message{{Role: "user", Content: "ping"}},
	}
	maxTokens := 1
	req.MaxTokens = &maxTokens

	_, err := g.Chat(ctx, req)
	return err
}

// Groq-specific types (OpenAI-compatible)
type groqResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []groqChoice `json:"choices"`
	Usage   groqUsage    `json:"usage"`
}

type groqChoice struct {
	Index        int         `json:"index"`
	Message      groqMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type groqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type groqStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

func (g *Groq) convertRequest(req *ChatRequest) map[string]interface{} {
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

	return result
}

func (g *Groq) convertResponse(resp *groqResponse) *ChatResponse {
	if len(resp.Choices) == 0 {
		return &ChatResponse{Provider: "groq", Model: resp.Model}
	}

	choice := resp.Choices[0]

	return &ChatResponse{
		Content:      choice.Message.Content,
		Model:        resp.Model,
		Provider:     "groq",
		FinishReason: choice.FinishReason,
		Usage: Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}
}
