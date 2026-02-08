package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestAnthropic_RateLimitDetection(t *testing.T) {
	callCount := int32(0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&callCount, 1)

		if count == 1 {
			// First call: return rate limit
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]string{
					"type":    "rate_limit_error",
					"message": "Rate limit exceeded",
				},
			})
			return
		}

		// Subsequent calls: success
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   "msg_123",
			"type": "message",
			"role": "assistant",
			"content": []map[string]string{
				{"type": "text", "text": "Hello!"},
			},
			"stop_reason": "end_turn",
			"usage":       map[string]int{"input_tokens": 10, "output_tokens": 5},
		})
	}))
	defer server.Close()

	provider := NewAnthropic("test-key", server.URL, WithAutoResume(false))

	ctx := context.Background()
	req := &ChatRequest{
		Model:    "claude-3-5-haiku-20241022",
		Messages: []Message{{Role: "user", Content: "Hello"}},
	}

	// First call should hit rate limit
	_, err := provider.Chat(ctx, req)
	if err == nil {
		t.Fatal("Expected rate limit error")
	}

	info, ok := IsRateLimitError(err)
	if !ok {
		t.Fatal("Should be a rate limit error")
	}
	if info == nil {
		t.Fatal("Rate limit info should not be nil")
	}

	// Provider should report as rate limited
	if !provider.IsRateLimited() {
		t.Error("Provider should report as rate limited")
	}
}

func TestAnthropic_AutoResume(t *testing.T) {
	callCount := int32(0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&callCount, 1)

		if count == 1 {
			// First call: return rate limit with short retry
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]string{
					"type":    "rate_limit_error",
					"message": "Rate limit exceeded",
				},
			})
			return
		}

		// Subsequent calls: success
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   "msg_123",
			"type": "message",
			"role": "assistant",
			"content": []map[string]string{
				{"type": "text", "text": "Hello after resume!"},
			},
			"stop_reason": "end_turn",
			"usage":       map[string]int{"input_tokens": 10, "output_tokens": 5},
		})
	}))
	defer server.Close()

	resumeCalled := false
	provider := NewAnthropic("test-key", server.URL,
		WithAutoResume(true),
		WithResumeCallback(func() {
			resumeCalled = true
		}),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &ChatRequest{
		Model:    "claude-3-5-haiku-20241022",
		Messages: []Message{{Role: "user", Content: "Hello"}},
	}

	// Call should queue and eventually succeed after rate limit clears
	resp, err := provider.Chat(ctx, req)
	if err != nil {
		t.Fatalf("Expected success after auto-resume, got: %v", err)
	}

	if resp.Content != "Hello after resume!" {
		t.Errorf("Unexpected response: %s", resp.Content)
	}

	// Wait a bit for callback
	time.Sleep(100 * time.Millisecond)
	if !resumeCalled {
		t.Error("Resume callback should have been called")
	}
}

func TestAnthropic_RateLimitCallbacks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"type":    "rate_limit_error",
				"message": "Weekly limit exceeded",
			},
		})
	}))
	defer server.Close()

	rateLimitedCalled := false
	var capturedInfo RateLimitInfo

	provider := NewAnthropic("test-key", server.URL,
		WithAutoResume(false),
		WithRateLimitCallback(func(info RateLimitInfo) {
			rateLimitedCalled = true
			capturedInfo = info
		}),
	)

	ctx := context.Background()
	req := &ChatRequest{
		Model:    "claude-3-5-haiku-20241022",
		Messages: []Message{{Role: "user", Content: "Hello"}},
	}

	_, _ = provider.Chat(ctx, req)

	// Wait for async callback
	time.Sleep(100 * time.Millisecond)

	if !rateLimitedCalled {
		t.Error("Rate limited callback should have been called")
	}

	if capturedInfo.Type != RateLimitWeekly {
		t.Errorf("Expected weekly limit, got %s", capturedInfo.Type)
	}
}

func TestAnthropic_PendingRequests(t *testing.T) {
	callCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&callCount, 1)
		if count == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]string{"message": "Rate limit"},
			})
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":          "msg_123",
			"type":        "message",
			"role":        "assistant",
			"content":     []map[string]string{{"type": "text", "text": "OK"}},
			"stop_reason": "end_turn",
			"usage":       map[string]int{"input_tokens": 1, "output_tokens": 1},
		})
	}))
	defer server.Close()

	provider := NewAnthropic("test-key", server.URL, WithAutoResume(true))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req := &ChatRequest{
		Model:    "claude-3-5-haiku-20241022",
		Messages: []Message{{Role: "user", Content: "Hello"}},
	}

	// This should get rate limited, queue, wait, and succeed
	resp, err := provider.Chat(ctx, req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if resp.Content != "OK" {
		t.Errorf("Unexpected content: %s", resp.Content)
	}
}

func TestAnthropic_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always rate limit
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{"message": "Rate limit"},
		})
	}))
	defer server.Close()

	provider := NewAnthropic("test-key", server.URL, WithAutoResume(true))

	// Short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	req := &ChatRequest{
		Model:    "claude-3-5-haiku-20241022",
		Messages: []Message{{Role: "user", Content: "Hello"}},
	}

	_, err := provider.Chat(ctx, req)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected deadline exceeded, got: %v", err)
	}
}

func TestAnthropic_TimeUntilResume(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{"message": "Rate limit"},
		})
	}))
	defer server.Close()

	provider := NewAnthropic("test-key", server.URL, WithAutoResume(false))

	ctx := context.Background()
	req := &ChatRequest{
		Model:    "claude-3-5-haiku-20241022",
		Messages: []Message{{Role: "user", Content: "Hello"}},
	}

	_, _ = provider.Chat(ctx, req)

	remaining := provider.TimeUntilResume()
	if remaining < 1*time.Minute || remaining > 3*time.Minute {
		t.Errorf("Expected ~2 minutes remaining, got %v", remaining)
	}
}

func TestAnthropic_SuccessfulRequestClearsRateLimit(t *testing.T) {
	callCount := int32(0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&callCount, 1)

		if count == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Header().Set("Retry-After", "0") // Immediate retry
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]string{"message": "Rate limit"},
			})
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":          "msg_123",
			"type":        "message",
			"role":        "assistant",
			"content":     []map[string]string{{"type": "text", "text": "OK"}},
			"stop_reason": "end_turn",
			"usage":       map[string]int{"input_tokens": 1, "output_tokens": 1},
		})
	}))
	defer server.Close()

	provider := NewAnthropic("test-key", server.URL, WithAutoResume(true))

	ctx := context.Background()
	req := &ChatRequest{
		Model:    "claude-3-5-haiku-20241022",
		Messages: []Message{{Role: "user", Content: "Hello"}},
	}

	// First call gets rate limited but auto-resumes
	_, err := provider.Chat(ctx, req)
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}

	// Should no longer be rate limited after successful request
	time.Sleep(100 * time.Millisecond)
	if provider.IsRateLimited() {
		t.Error("Should not be rate limited after successful request")
	}
}
