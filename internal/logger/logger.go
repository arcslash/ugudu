// Package logger provides a simple structured logging interface
package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Level represents logging level
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// Logger provides structured logging
type Logger struct {
	level  Level
	output io.Writer
	fields map[string]interface{}
	mu     sync.Mutex
}

// New creates a new logger
func New(level string, output ...io.Writer) *Logger {
	var out io.Writer = os.Stdout
	if len(output) > 0 && output[0] != nil {
		out = output[0]
	}
	l := &Logger{
		level:  parseLevel(level),
		output: out,
		fields: make(map[string]interface{}),
	}
	return l
}

func parseLevel(s string) Level {
	switch strings.ToLower(s) {
	case "debug":
		return LevelDebug
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// With returns a new logger with additional fields
func (l *Logger) With(keyvals ...interface{}) *Logger {
	newLogger := &Logger{
		level:  l.level,
		output: l.output,
		fields: make(map[string]interface{}),
	}

	// Copy existing fields
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}

	// Add new fields
	for i := 0; i < len(keyvals)-1; i += 2 {
		if key, ok := keyvals[i].(string); ok {
			newLogger.fields[key] = keyvals[i+1]
		}
	}

	return newLogger
}

// Debug logs at debug level
func (l *Logger) Debug(msg string, keyvals ...interface{}) {
	if l.level <= LevelDebug {
		l.log("DEBUG", msg, keyvals...)
	}
}

// Info logs at info level
func (l *Logger) Info(msg string, keyvals ...interface{}) {
	if l.level <= LevelInfo {
		l.log("INFO", msg, keyvals...)
	}
}

// Warn logs at warn level
func (l *Logger) Warn(msg string, keyvals ...interface{}) {
	if l.level <= LevelWarn {
		l.log("WARN", msg, keyvals...)
	}
}

// Error logs at error level
func (l *Logger) Error(msg string, keyvals ...interface{}) {
	if l.level <= LevelError {
		l.log("ERROR", msg, keyvals...)
	}
}

func (l *Logger) log(level, msg string, keyvals ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("15:04:05")

	// Build fields string
	var fields strings.Builder

	// Add stored fields
	for k, v := range l.fields {
		fields.WriteString(fmt.Sprintf(" %s=%v", k, v))
	}

	// Add inline fields
	for i := 0; i < len(keyvals)-1; i += 2 {
		if key, ok := keyvals[i].(string); ok {
			fields.WriteString(fmt.Sprintf(" %s=%v", key, keyvals[i+1]))
		}
	}

	fmt.Fprintf(l.output, "%s %s %s%s\n", timestamp, level, msg, fields.String())
}
