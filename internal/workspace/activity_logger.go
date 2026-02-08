package workspace

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ActivityLogger handles logging activities to JSONL files
type ActivityLogger struct {
	workspace   *Workspace
	role        string
	logPath     string
	mu          sync.Mutex
	file        *os.File
	writer      *bufio.Writer
	entryCount  int64
}

// NewActivityLogger creates a new activity logger for a role
func NewActivityLogger(ws *Workspace, role string) (*ActivityLogger, error) {
	logPath := ws.ActivityPath(role)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return nil, fmt.Errorf("create activity directory: %w", err)
	}

	// Open log file in append mode
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open activity log: %w", err)
	}

	return &ActivityLogger{
		workspace: ws,
		role:      role,
		logPath:   logPath,
		file:      file,
		writer:    bufio.NewWriter(file),
	}, nil
}

// Log writes an activity entry to the log
func (l *ActivityLogger) Log(entry *ActivityEntry) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Ensure entry has the correct role
	if entry.AgentRole == "" {
		entry.AgentRole = l.role
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal activity entry: %w", err)
	}

	if _, err := l.writer.Write(data); err != nil {
		return fmt.Errorf("write activity entry: %w", err)
	}

	if _, err := l.writer.WriteString("\n"); err != nil {
		return fmt.Errorf("write newline: %w", err)
	}

	l.entryCount++

	// Flush periodically
	if l.entryCount%10 == 0 {
		if err := l.writer.Flush(); err != nil {
			return fmt.Errorf("flush activity log: %w", err)
		}
	}

	return nil
}

// Flush flushes buffered data to disk
func (l *ActivityLogger) Flush() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.writer.Flush(); err != nil {
		return fmt.Errorf("flush activity log: %w", err)
	}
	return l.file.Sync()
}

// Close closes the activity logger
func (l *ActivityLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.writer.Flush(); err != nil {
		return fmt.Errorf("flush on close: %w", err)
	}
	return l.file.Close()
}

// Query returns activity entries matching the given criteria
func (l *ActivityLogger) Query(opts QueryOptions) ([]ActivityEntry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Flush before querying
	l.writer.Flush()

	return QueryActivityLog(l.logPath, opts)
}

// QueryOptions defines options for querying activity logs
type QueryOptions struct {
	Since      time.Time    // Only entries after this time
	Until      time.Time    // Only entries before this time
	Types      []ActivityType // Filter by activity types
	TaskID     string       // Filter by task ID
	Limit      int          // Maximum number of entries
	Offset     int          // Skip first N entries
}

// QueryActivityLog queries a single activity log file
func QueryActivityLog(logPath string, opts QueryOptions) ([]ActivityEntry, error) {
	file, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []ActivityEntry{}, nil
		}
		return nil, fmt.Errorf("open activity log: %w", err)
	}
	defer file.Close()

	var entries []ActivityEntry
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer

	skipped := 0
	for scanner.Scan() {
		var entry ActivityEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue // Skip malformed entries
		}

		// Apply filters
		if !opts.Since.IsZero() && entry.Timestamp.Before(opts.Since) {
			continue
		}
		if !opts.Until.IsZero() && entry.Timestamp.After(opts.Until) {
			continue
		}
		if len(opts.Types) > 0 {
			found := false
			for _, t := range opts.Types {
				if entry.Type == t {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		if opts.TaskID != "" && entry.TaskID != opts.TaskID {
			continue
		}

		// Apply offset
		if opts.Offset > 0 && skipped < opts.Offset {
			skipped++
			continue
		}

		entries = append(entries, entry)

		// Apply limit
		if opts.Limit > 0 && len(entries) >= opts.Limit {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan activity log: %w", err)
	}

	return entries, nil
}

// QueryProjectActivity queries all activity logs for a project
func QueryProjectActivity(ws *Workspace, opts QueryOptions) ([]ActivityEntry, error) {
	activityDir := filepath.Join(ws.Path, "activity")

	entries, err := os.ReadDir(activityDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []ActivityEntry{}, nil
		}
		return nil, fmt.Errorf("read activity directory: %w", err)
	}

	var allEntries []ActivityEntry
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".log" {
			continue
		}

		logPath := filepath.Join(activityDir, entry.Name())
		logEntries, err := QueryActivityLog(logPath, opts)
		if err != nil {
			continue // Skip problematic files
		}

		allEntries = append(allEntries, logEntries...)
	}

	// Sort by timestamp (most recent first)
	sortActivityEntries(allEntries)

	// Apply global limit
	if opts.Limit > 0 && len(allEntries) > opts.Limit {
		allEntries = allEntries[:opts.Limit]
	}

	return allEntries, nil
}

// sortActivityEntries sorts entries by timestamp (most recent first)
func sortActivityEntries(entries []ActivityEntry) {
	// Simple bubble sort for now (could use sort.Slice for larger datasets)
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].Timestamp.After(entries[i].Timestamp) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
}

// ActivitySummary provides a summary of activity
type ActivitySummary struct {
	TotalEntries   int                     `json:"total_entries"`
	ByType         map[ActivityType]int    `json:"by_type"`
	ByRole         map[string]int          `json:"by_role"`
	SuccessRate    float64                 `json:"success_rate"`
	FirstEntry     time.Time               `json:"first_entry"`
	LastEntry      time.Time               `json:"last_entry"`
	AverageDuration int64                  `json:"average_duration_ms"`
}

// SummarizeActivity generates a summary of activity entries
func SummarizeActivity(entries []ActivityEntry) *ActivitySummary {
	if len(entries) == 0 {
		return &ActivitySummary{
			ByType: make(map[ActivityType]int),
			ByRole: make(map[string]int),
		}
	}

	summary := &ActivitySummary{
		TotalEntries: len(entries),
		ByType:       make(map[ActivityType]int),
		ByRole:       make(map[string]int),
		FirstEntry:   entries[0].Timestamp,
		LastEntry:    entries[0].Timestamp,
	}

	successCount := 0
	totalDuration := int64(0)
	durationCount := 0

	for _, entry := range entries {
		summary.ByType[entry.Type]++
		summary.ByRole[entry.AgentRole]++

		if entry.Success {
			successCount++
		}

		if entry.Duration > 0 {
			totalDuration += entry.Duration
			durationCount++
		}

		if entry.Timestamp.Before(summary.FirstEntry) {
			summary.FirstEntry = entry.Timestamp
		}
		if entry.Timestamp.After(summary.LastEntry) {
			summary.LastEntry = entry.Timestamp
		}
	}

	summary.SuccessRate = float64(successCount) / float64(len(entries)) * 100

	if durationCount > 0 {
		summary.AverageDuration = totalDuration / int64(durationCount)
	}

	return summary
}
