package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RateLimitType represents the type of rate limit
type RateLimitType string

const (
	RateLimitMinute  RateLimitType = "minute"
	RateLimitDaily   RateLimitType = "daily"
	RateLimitWeekly  RateLimitType = "weekly"
	RateLimitMonthly RateLimitType = "monthly"
)

// RateLimitInfo holds information about a rate limit
type RateLimitInfo struct {
	Type           RateLimitType `json:"type"`
	Limit          int           `json:"limit"`
	Remaining      int           `json:"remaining"`
	ResetAt        time.Time     `json:"reset_at"`
	RetryAfter     time.Duration `json:"retry_after"`
	HitAt          time.Time     `json:"hit_at"`
	Message        string        `json:"message"`
}

// RateLimitState tracks rate limit status for a provider
type RateLimitState struct {
	mu             sync.RWMutex
	limits         map[RateLimitType]*RateLimitInfo
	isRateLimited  bool
	resumeAt       time.Time
	callbacks      []func(RateLimitInfo)
	resumeCallback func()
}

// NewRateLimitState creates a new rate limit state tracker
func NewRateLimitState() *RateLimitState {
	return &RateLimitState{
		limits: make(map[RateLimitType]*RateLimitInfo),
	}
}

// OnRateLimited registers a callback for when rate limit is hit
func (r *RateLimitState) OnRateLimited(cb func(RateLimitInfo)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.callbacks = append(r.callbacks, cb)
}

// OnResume registers a callback for when rate limit clears
func (r *RateLimitState) OnResume(cb func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.resumeCallback = cb
}

// IsRateLimited returns true if currently rate limited
func (r *RateLimitState) IsRateLimited() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.isRateLimited {
		return false
	}

	// Check if we've passed the resume time
	if time.Now().After(r.resumeAt) {
		return false
	}

	return true
}

// GetResumeTime returns when the rate limit will reset
func (r *RateLimitState) GetResumeTime() time.Time {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.resumeAt
}

// TimeUntilResume returns duration until rate limit resets
func (r *RateLimitState) TimeUntilResume() time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.isRateLimited {
		return 0
	}

	remaining := time.Until(r.resumeAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// RecordRateLimit records a rate limit hit
func (r *RateLimitState) RecordRateLimit(info RateLimitInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.isRateLimited = true
	r.limits[info.Type] = &info

	// Set resume time to the latest reset time
	if info.ResetAt.After(r.resumeAt) {
		r.resumeAt = info.ResetAt
	}

	// Notify callbacks
	for _, cb := range r.callbacks {
		go cb(info)
	}
}

// ClearRateLimit marks rate limit as cleared
func (r *RateLimitState) ClearRateLimit() {
	r.mu.Lock()
	wasLimited := r.isRateLimited
	r.isRateLimited = false
	r.limits = make(map[RateLimitType]*RateLimitInfo)
	cb := r.resumeCallback
	r.mu.Unlock()

	if wasLimited && cb != nil {
		go cb()
	}
}

// GetLimits returns current rate limit info
func (r *RateLimitState) GetLimits() map[RateLimitType]*RateLimitInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[RateLimitType]*RateLimitInfo)
	for k, v := range r.limits {
		result[k] = v
	}
	return result
}

// ParseAnthropicRateLimitError parses rate limit info from Anthropic error response
func ParseAnthropicRateLimitError(statusCode int, headers http.Header, body []byte) *RateLimitInfo {
	if statusCode != http.StatusTooManyRequests {
		return nil
	}

	info := &RateLimitInfo{
		HitAt:   time.Now(),
		Type:    RateLimitMinute, // Default
	}

	// Parse Retry-After header
	if retryAfter := headers.Get("Retry-After"); retryAfter != "" {
		if seconds, err := strconv.Atoi(retryAfter); err == nil {
			info.RetryAfter = time.Duration(seconds) * time.Second
			info.ResetAt = time.Now().Add(info.RetryAfter)
		}
	}

	// Parse rate limit headers
	if limit := headers.Get("X-RateLimit-Limit-Requests"); limit != "" {
		info.Limit, _ = strconv.Atoi(limit)
	}
	if remaining := headers.Get("X-RateLimit-Remaining-Requests"); remaining != "" {
		info.Remaining, _ = strconv.Atoi(remaining)
	}
	if resetStr := headers.Get("X-RateLimit-Reset-Requests"); resetStr != "" {
		// Parse duration like "1m30s" or timestamp
		if d, err := time.ParseDuration(resetStr); err == nil {
			info.ResetAt = time.Now().Add(d)
		}
	}

	// Parse error body for more details
	var errorResp struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &errorResp); err == nil {
		info.Message = errorResp.Error.Message

		// Detect limit type from message
		msg := strings.ToLower(errorResp.Error.Message)
		if strings.Contains(msg, "daily") {
			info.Type = RateLimitDaily
			// Daily limits reset at midnight UTC
			now := time.Now().UTC()
			midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
			info.ResetAt = midnight
		} else if strings.Contains(msg, "weekly") {
			info.Type = RateLimitWeekly
			// Weekly limits typically reset on Monday
			info.ResetAt = nextWeeklyReset()
		} else if strings.Contains(msg, "monthly") {
			info.Type = RateLimitMonthly
			// Monthly limits reset on the 1st
			info.ResetAt = nextMonthlyReset()
		}
	}

	// If no reset time determined, use retry-after or default
	if info.ResetAt.IsZero() {
		if info.RetryAfter > 0 {
			info.ResetAt = time.Now().Add(info.RetryAfter)
		} else {
			// Default: 1 minute for unknown limits
			info.ResetAt = time.Now().Add(1 * time.Minute)
		}
	}

	return info
}

// nextWeeklyReset calculates the next weekly reset time (Monday 00:00 UTC)
func nextWeeklyReset() time.Time {
	now := time.Now().UTC()
	daysUntilMonday := (8 - int(now.Weekday())) % 7
	if daysUntilMonday == 0 && now.Hour() > 0 {
		daysUntilMonday = 7
	}
	return time.Date(now.Year(), now.Month(), now.Day()+daysUntilMonday, 0, 0, 0, 0, time.UTC)
}

// nextMonthlyReset calculates the next monthly reset time (1st of next month 00:00 UTC)
func nextMonthlyReset() time.Time {
	now := time.Now().UTC()
	year := now.Year()
	month := now.Month() + 1
	if month > 12 {
		month = 1
		year++
	}
	return time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
}

// RateLimitError is an error that includes rate limit information
type RateLimitError struct {
	Info    *RateLimitInfo
	Message string
}

func (e *RateLimitError) Error() string {
	if e.Info != nil {
		return fmt.Sprintf("rate limited (%s): %s - retry after %v",
			e.Info.Type, e.Message, time.Until(e.Info.ResetAt).Round(time.Second))
	}
	return fmt.Sprintf("rate limited: %s", e.Message)
}

// IsRateLimitError checks if an error is a rate limit error
func IsRateLimitError(err error) (*RateLimitInfo, bool) {
	if rle, ok := err.(*RateLimitError); ok {
		return rle.Info, true
	}
	return nil, false
}

// PendingRequest represents a queued request waiting for rate limit to clear
type PendingRequest struct {
	ID        string
	Request   *ChatRequest
	Context   context.Context
	Response  chan *ChatResponse
	Error     chan error
	CreatedAt time.Time
}

// RequestQueue manages pending requests during rate limiting
type RequestQueue struct {
	mu       sync.Mutex
	requests []*PendingRequest
	maxSize  int
}

// NewRequestQueue creates a new request queue
func NewRequestQueue(maxSize int) *RequestQueue {
	if maxSize <= 0 {
		maxSize = 100
	}
	return &RequestQueue{
		maxSize: maxSize,
	}
}

// Add adds a request to the queue
func (q *RequestQueue) Add(req *PendingRequest) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.requests) >= q.maxSize {
		return fmt.Errorf("request queue full")
	}

	q.requests = append(q.requests, req)
	return nil
}

// Pop removes and returns the oldest request
func (q *RequestQueue) Pop() *PendingRequest {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.requests) == 0 {
		return nil
	}

	req := q.requests[0]
	q.requests = q.requests[1:]
	return req
}

// Len returns the number of pending requests
func (q *RequestQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.requests)
}

// Clear removes all pending requests
func (q *RequestQueue) Clear() []*PendingRequest {
	q.mu.Lock()
	defer q.mu.Unlock()

	reqs := q.requests
	q.requests = nil
	return reqs
}
