package provider

import (
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func TestRateLimitState_BasicOperations(t *testing.T) {
	state := NewRateLimitState()

	// Initially not rate limited
	if state.IsRateLimited() {
		t.Error("Should not be rate limited initially")
	}

	// Record a rate limit
	info := RateLimitInfo{
		Type:    RateLimitMinute,
		ResetAt: time.Now().Add(1 * time.Minute),
		HitAt:   time.Now(),
		Message: "Too many requests",
	}
	state.RecordRateLimit(info)

	// Should be rate limited now
	if !state.IsRateLimited() {
		t.Error("Should be rate limited after recording")
	}

	// Clear rate limit
	state.ClearRateLimit()

	if state.IsRateLimited() {
		t.Error("Should not be rate limited after clearing")
	}
}

func TestRateLimitState_ResumeTime(t *testing.T) {
	state := NewRateLimitState()

	resetTime := time.Now().Add(5 * time.Minute)
	info := RateLimitInfo{
		Type:    RateLimitDaily,
		ResetAt: resetTime,
	}
	state.RecordRateLimit(info)

	resumeTime := state.GetResumeTime()
	if !resumeTime.Equal(resetTime) {
		t.Errorf("Expected resume time %v, got %v", resetTime, resumeTime)
	}

	remaining := state.TimeUntilResume()
	if remaining < 4*time.Minute || remaining > 6*time.Minute {
		t.Errorf("Expected ~5 minutes remaining, got %v", remaining)
	}
}

func TestRateLimitState_Callbacks(t *testing.T) {
	state := NewRateLimitState()

	var rateLimitedCalled atomic.Bool
	var resumeCalled atomic.Bool

	state.OnRateLimited(func(info RateLimitInfo) {
		rateLimitedCalled.Store(true)
	})

	state.OnResume(func() {
		resumeCalled.Store(true)
	})

	// Trigger rate limit
	state.RecordRateLimit(RateLimitInfo{
		Type:    RateLimitMinute,
		ResetAt: time.Now().Add(1 * time.Second),
	})

	// Wait for callback
	time.Sleep(50 * time.Millisecond)
	if !rateLimitedCalled.Load() {
		t.Error("Rate limited callback should have been called")
	}

	// Clear and check resume callback
	state.ClearRateLimit()
	time.Sleep(50 * time.Millisecond)
	if !resumeCalled.Load() {
		t.Error("Resume callback should have been called")
	}
}

func TestRateLimitState_AutoClear(t *testing.T) {
	state := NewRateLimitState()

	// Set rate limit that expires immediately
	info := RateLimitInfo{
		Type:    RateLimitMinute,
		ResetAt: time.Now().Add(-1 * time.Second), // Already expired
	}
	state.RecordRateLimit(info)

	// Should not be rate limited since reset time passed
	if state.IsRateLimited() {
		t.Error("Should not be rate limited when reset time has passed")
	}
}

func TestParseAnthropicRateLimitError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		headers    http.Header
		body       []byte
		wantType   RateLimitType
		wantNil    bool
	}{
		{
			name:       "not rate limited",
			statusCode: http.StatusOK,
			wantNil:    true,
		},
		{
			name:       "rate limited with retry-after",
			statusCode: http.StatusTooManyRequests,
			headers: http.Header{
				"Retry-After": []string{"60"},
			},
			body:     []byte(`{"error":{"type":"rate_limit_error","message":"Rate limit exceeded"}}`),
			wantType: RateLimitMinute,
		},
		{
			name:       "daily rate limit",
			statusCode: http.StatusTooManyRequests,
			headers:    http.Header{},
			body:       []byte(`{"error":{"type":"rate_limit_error","message":"Daily limit exceeded"}}`),
			wantType:   RateLimitDaily,
		},
		{
			name:       "weekly rate limit",
			statusCode: http.StatusTooManyRequests,
			headers:    http.Header{},
			body:       []byte(`{"error":{"type":"rate_limit_error","message":"Weekly limit exceeded"}}`),
			wantType:   RateLimitWeekly,
		},
		{
			name:       "monthly rate limit",
			statusCode: http.StatusTooManyRequests,
			headers:    http.Header{},
			body:       []byte(`{"error":{"type":"rate_limit_error","message":"Monthly limit exceeded"}}`),
			wantType:   RateLimitMonthly,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := ParseAnthropicRateLimitError(tt.statusCode, tt.headers, tt.body)

			if tt.wantNil {
				if info != nil {
					t.Error("Expected nil info")
				}
				return
			}

			if info == nil {
				t.Fatal("Expected non-nil info")
			}

			if info.Type != tt.wantType {
				t.Errorf("Expected type %s, got %s", tt.wantType, info.Type)
			}

			if info.ResetAt.IsZero() {
				t.Error("ResetAt should not be zero")
			}
		})
	}
}

func TestRequestQueue(t *testing.T) {
	queue := NewRequestQueue(3)

	// Add requests
	for i := 0; i < 3; i++ {
		req := &PendingRequest{
			ID:        string(rune('a' + i)),
			CreatedAt: time.Now(),
		}
		if err := queue.Add(req); err != nil {
			t.Errorf("Failed to add request %d: %v", i, err)
		}
	}

	// Queue should be at capacity
	if queue.Len() != 3 {
		t.Errorf("Expected length 3, got %d", queue.Len())
	}

	// Adding another should fail
	if err := queue.Add(&PendingRequest{ID: "d"}); err == nil {
		t.Error("Expected error when queue is full")
	}

	// Pop should return oldest first
	req := queue.Pop()
	if req.ID != "a" {
		t.Errorf("Expected 'a', got '%s'", req.ID)
	}

	// Length should decrease
	if queue.Len() != 2 {
		t.Errorf("Expected length 2, got %d", queue.Len())
	}

	// Clear should remove all
	reqs := queue.Clear()
	if len(reqs) != 2 {
		t.Errorf("Expected 2 cleared requests, got %d", len(reqs))
	}

	if queue.Len() != 0 {
		t.Error("Queue should be empty after clear")
	}
}

func TestNextWeeklyReset(t *testing.T) {
	reset := nextWeeklyReset()

	// Should be a Monday
	if reset.Weekday() != time.Monday {
		t.Errorf("Expected Monday, got %s", reset.Weekday())
	}

	// Should be in the future
	if !reset.After(time.Now()) {
		t.Error("Reset time should be in the future")
	}

	// Should be within 7 days
	if reset.Sub(time.Now()) > 8*24*time.Hour {
		t.Error("Reset time should be within 7 days")
	}
}

func TestNextMonthlyReset(t *testing.T) {
	reset := nextMonthlyReset()

	// Should be the 1st of the month
	if reset.Day() != 1 {
		t.Errorf("Expected day 1, got %d", reset.Day())
	}

	// Should be in the future
	if !reset.After(time.Now()) {
		t.Error("Reset time should be in the future")
	}

	// Should be within ~31 days
	if reset.Sub(time.Now()) > 32*24*time.Hour {
		t.Error("Reset time should be within 32 days")
	}
}

func TestRateLimitError(t *testing.T) {
	info := &RateLimitInfo{
		Type:    RateLimitWeekly,
		ResetAt: time.Now().Add(3 * 24 * time.Hour),
	}

	err := &RateLimitError{
		Info:    info,
		Message: "Weekly limit exceeded",
	}

	errStr := err.Error()
	if errStr == "" {
		t.Error("Error string should not be empty")
	}

	// Should contain type
	if !contains(errStr, "weekly") {
		t.Errorf("Error should contain 'weekly': %s", errStr)
	}

	// Test IsRateLimitError
	extractedInfo, ok := IsRateLimitError(err)
	if !ok {
		t.Error("Should detect rate limit error")
	}
	if extractedInfo.Type != RateLimitWeekly {
		t.Error("Should extract correct info")
	}

	// Test with non-rate-limit error
	_, ok = IsRateLimitError(http.ErrAbortHandler)
	if ok {
		t.Error("Should not detect non-rate-limit error")
	}
}

func TestRateLimitState_MultipleLimits(t *testing.T) {
	state := NewRateLimitState()

	// Record multiple limits
	state.RecordRateLimit(RateLimitInfo{
		Type:    RateLimitMinute,
		ResetAt: time.Now().Add(1 * time.Minute),
	})

	state.RecordRateLimit(RateLimitInfo{
		Type:    RateLimitDaily,
		ResetAt: time.Now().Add(24 * time.Hour),
	})

	// Should have both limits
	limits := state.GetLimits()
	if len(limits) != 2 {
		t.Errorf("Expected 2 limits, got %d", len(limits))
	}

	// Resume time should be the latest
	resumeTime := state.GetResumeTime()
	if resumeTime.Before(time.Now().Add(23 * time.Hour)) {
		t.Error("Resume time should be based on the longest limit")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
