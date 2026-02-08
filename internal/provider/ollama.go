package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Ollama implements the Provider interface for local Ollama models
type Ollama struct {
	baseURL string
	client  *http.Client
}

// NewOllama creates a new Ollama provider
func NewOllama(baseURL string) *Ollama {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &Ollama{
		baseURL: baseURL,
		client:  &http.Client{},
	}
}

func (o *Ollama) ID() string   { return "ollama" }
func (o *Ollama) Name() string { return "Ollama (Local)" }

func (o *Ollama) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	ollamaReq := o.convertRequest(req)
	ollamaReq["stream"] = false

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var ollamaResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return o.convertResponse(&ollamaResp, req.Model), nil
}

func (o *Ollama) Stream(ctx context.Context, req *ChatRequest) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk)

	ollamaReq := o.convertRequest(req)
	ollamaReq["stream"] = true

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		close(ch)
		return ch, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		close(ch)
		return ch, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

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

		decoder := json.NewDecoder(resp.Body)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				var chunk ollamaStreamChunk
				if err := decoder.Decode(&chunk); err != nil {
					if err == io.EOF {
						ch <- StreamChunk{Done: true}
						return
					}
					ch <- StreamChunk{Error: err}
					return
				}

				ch <- StreamChunk{
					Content: chunk.Message.Content,
					Done:    chunk.Done,
				}

				if chunk.Done {
					return
				}
			}
		}
	}()

	return ch, nil
}

func (o *Ollama) ListModels(ctx context.Context) ([]ModelInfo, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", o.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Ollama might not be running, return empty list
		return []ModelInfo{}, nil
	}

	var tagsResp struct {
		Models []struct {
			Name string `json:"name"`
			Size int64  `json:"size"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	models := make([]ModelInfo, len(tagsResp.Models))
	for i, m := range tagsResp.Models {
		models[i] = ModelInfo{
			ID:       m.Name,
			Name:     m.Name,
			Provider: "ollama",
		}
	}

	return models, nil
}

func (o *Ollama) Ping(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", o.baseURL+"/api/tags", nil)
	if err != nil {
		return err
	}

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("ollama not reachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	return nil
}

// Ollama-specific types
type ollamaResponse struct {
	Model     string        `json:"model"`
	CreatedAt string        `json:"created_at"`
	Message   ollamaMessage `json:"message"`
	Done      bool          `json:"done"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaStreamChunk struct {
	Model   string        `json:"model"`
	Message ollamaMessage `json:"message"`
	Done    bool          `json:"done"`
}

func (o *Ollama) convertRequest(req *ChatRequest) map[string]interface{} {
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

	if req.Temperature != nil {
		result["options"] = map[string]interface{}{
			"temperature": *req.Temperature,
		}
	}

	return result
}

func (o *Ollama) convertResponse(resp *ollamaResponse, model string) *ChatResponse {
	return &ChatResponse{
		Content:      resp.Message.Content,
		Model:        model,
		Provider:     "ollama",
		FinishReason: "stop",
	}
}
