package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// LogLevel represents the severity of a log entry
type LogLevel string

const (
	DEBUG   LogLevel = "debug"
	INFO    LogLevel = "info"
	WARNING LogLevel = "warning"
	ERROR   LogLevel = "error"
	FATAL   LogLevel = "fatal"
)

// Entry represents a structured log entry
type Entry struct {
	Timestamp string            `json:"timestamp"`
	Level     string            `json:"level"`
	Message   string            `json:"message"`
	Error     string            `json:"error,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// Logger provides structured logging functionality
type Logger struct {
	minLevel LogLevel
}

// New creates a new logger instance
func New() *Logger {
	levelStr := os.Getenv("LOG_LEVEL")
	if levelStr == "" {
		levelStr = "info"
	}

	var minLevel LogLevel
	switch levelStr {
	case "debug":
		minLevel = DEBUG
	case "warning":
		minLevel = WARNING
	case "error":
		minLevel = ERROR
	case "fatal":
		minLevel = FATAL
	default:
		minLevel = INFO
	}

	return &Logger{
		minLevel: minLevel,
	}
}

// shouldLog determines if a message should be logged based on level
func (l *Logger) shouldLog(level LogLevel) bool {
	levels := map[LogLevel]int{
		DEBUG:   0,
		INFO:    1,
		WARNING: 2,
		ERROR:   3,
		FATAL:   4,
	}

	return levels[level] >= levels[l.minLevel]
}

// log outputs a structured log entry
func (l *Logger) log(level LogLevel, msg string, err error, labels map[string]string) {
	if !l.shouldLog(level) {
		return
	}

	entry := Entry{
		Timestamp: time.Now().Format(time.RFC3339),
		Level:     string(level),
		Message:   msg,
		Labels:    labels,
	}

	if err != nil {
		entry.Error = err.Error()
	}

	jsonBytes, _ := json.Marshal(entry)
	fmt.Println(string(jsonBytes))
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, labels map[string]string) {
	l.log(DEBUG, msg, nil, labels)
}

// Info logs an info message
func (l *Logger) Info(msg string, labels map[string]string) {
	l.log(INFO, msg, nil, labels)
}

// Warning logs a warning message
func (l *Logger) Warning(msg string, err error, labels map[string]string) {
	l.log(WARNING, msg, err, labels)
}

// Error logs an error message
func (l *Logger) Error(msg string, err error, labels map[string]string) {
	l.log(ERROR, msg, err, labels)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string, err error, labels map[string]string) {
	l.log(FATAL, msg, err, labels)
	os.Exit(1)
}

// Deprecated functions for backward compatibility
// These will be removed in future versions

var defaultLogger = New()

// LogJSON is deprecated, use the Logger methods instead
func LogJSON(level, msg string, err error, labels map[string]string) {
	switch level {
	case "debug":
		defaultLogger.Debug(msg, labels)
	case "info":
		defaultLogger.Info(msg, labels)
	case "warning":
		defaultLogger.Warning(msg, err, labels)
	case "error":
		defaultLogger.Error(msg, err, labels)
	case "fatal":
		defaultLogger.Fatal(msg, err, labels)
	default:
		defaultLogger.Info(msg, labels)
	}
}
