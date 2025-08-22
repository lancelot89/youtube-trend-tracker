package logger

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestLogLevels(t *testing.T) {
	tests := []struct {
		name      string
		envLevel  string
		logLevel  LogLevel
		shouldLog bool
	}{
		{"Info logs at info level", "info", INFO, true},
		{"Debug doesn't log at info level", "info", DEBUG, false},
		{"Error logs at info level", "info", ERROR, true},
		{"Warning logs at warning level", "warning", WARNING, true},
		{"Info doesn't log at error level", "error", INFO, false},
		{"Debug logs at debug level", "debug", DEBUG, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			os.Setenv("LOG_LEVEL", tt.envLevel)
			defer os.Unsetenv("LOG_LEVEL")

			// Create logger
			l := New()

			// Capture output
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Log message
			l.log(tt.logLevel, "test message", nil, nil)

			// Restore stdout
			w.Close()
			os.Stdout = old

			// Read captured output
			var buf bytes.Buffer
			buf.ReadFrom(r)
			output := buf.String()

			if tt.shouldLog && output == "" {
				t.Errorf("Expected log output but got none")
			}
			if !tt.shouldLog && output != "" {
				t.Errorf("Expected no log output but got: %s", output)
			}

			// If we got output, verify it's valid JSON
			if output != "" {
				output = strings.TrimSpace(output)
				var entry Entry
				if err := json.Unmarshal([]byte(output), &entry); err != nil {
					t.Errorf("Invalid JSON output: %v", err)
				}
				if entry.Message != "test message" {
					t.Errorf("Expected message 'test message', got '%s'", entry.Message)
				}
				if entry.Level != string(tt.logLevel) {
					t.Errorf("Expected level '%s', got '%s'", tt.logLevel, entry.Level)
				}
			}
		})
	}
}

func TestLoggerMethods(t *testing.T) {
	// Set to debug level to capture all logs
	os.Setenv("LOG_LEVEL", "debug")
	defer os.Unsetenv("LOG_LEVEL")

	l := New()

	tests := []struct {
		name   string
		method func()
		level  string
	}{
		{
			name:   "Debug method",
			method: func() { l.Debug("debug message", nil) },
			level:  "debug",
		},
		{
			name:   "Info method",
			method: func() { l.Info("info message", nil) },
			level:  "info",
		},
		{
			name:   "Warning method",
			method: func() { l.Warning("warning message", nil, nil) },
			level:  "warning",
		},
		{
			name:   "Error method",
			method: func() { l.Error("error message", nil, nil) },
			level:  "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture output
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Call method
			tt.method()

			// Restore stdout
			w.Close()
			os.Stdout = old

			// Read captured output
			var buf bytes.Buffer
			buf.ReadFrom(r)
			output := strings.TrimSpace(buf.String())

			// Verify output
			var entry Entry
			if err := json.Unmarshal([]byte(output), &entry); err != nil {
				t.Errorf("Invalid JSON output: %v", err)
			}
			if entry.Level != tt.level {
				t.Errorf("Expected level '%s', got '%s'", tt.level, entry.Level)
			}
		})
	}
}

func TestLoggerWithLabels(t *testing.T) {
	os.Setenv("LOG_LEVEL", "info")
	defer os.Unsetenv("LOG_LEVEL")

	l := New()

	// Capture output
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Log with labels
	labels := map[string]string{
		"channel_id":  "test123",
		"video_count": "10",
	}
	l.Info("test with labels", labels)

	// Restore stdout
	w.Close()
	os.Stdout = old

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := strings.TrimSpace(buf.String())

	// Verify output
	var entry Entry
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Errorf("Invalid JSON output: %v", err)
	}
	if entry.Labels["channel_id"] != "test123" {
		t.Errorf("Expected label channel_id='test123', got '%s'", entry.Labels["channel_id"])
	}
	if entry.Labels["video_count"] != "10" {
		t.Errorf("Expected label video_count='10', got '%s'", entry.Labels["video_count"])
	}
}
