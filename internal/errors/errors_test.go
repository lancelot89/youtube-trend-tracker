package errors

import (
	"fmt"
	"testing"
)

func TestAppError(t *testing.T) {
	tests := []struct {
		name      string
		err       *AppError
		wantMsg   string
		wantType  ErrorType
		retriable bool
	}{
		{
			name:      "Config error",
			err:       Config("invalid configuration", nil),
			wantMsg:   "[CONFIG] invalid configuration",
			wantType:  ErrTypeConfig,
			retriable: false,
		},
		{
			name:      "API error with underlying error",
			err:       API("API call failed", fmt.Errorf("connection timeout")),
			wantMsg:   "[API] API call failed: connection timeout",
			wantType:  ErrTypeAPI,
			retriable: false,
		},
		{
			name:      "Temporary error",
			err:       Temporary("rate limit exceeded", nil),
			wantMsg:   "[TEMPORARY] rate limit exceeded",
			wantType:  ErrTypeTemporary,
			retriable: true,
		},
		{
			name:      "Storage error",
			err:       Storage("database connection failed", nil),
			wantMsg:   "[STORAGE] database connection failed",
			wantType:  ErrTypeStorage,
			retriable: false,
		},
		{
			name:      "Validation error",
			err:       Validation("invalid input", nil),
			wantMsg:   "[VALIDATION] invalid input",
			wantType:  ErrTypeValidation,
			retriable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("Error() = %v, want %v", got, tt.wantMsg)
			}
			if tt.err.Type != tt.wantType {
				t.Errorf("Type = %v, want %v", tt.err.Type, tt.wantType)
			}
			if tt.err.IsRetriable() != tt.retriable {
				t.Errorf("IsRetriable() = %v, want %v", tt.err.IsRetriable(), tt.retriable)
			}
		})
	}
}

func TestAppErrorWithContext(t *testing.T) {
	context := map[string]interface{}{
		"channel_id": "ABC123",
		"attempt":    3,
	}

	err := NewWithContext(ErrTypeAPI, "API failed", nil, context)

	if err.Context["channel_id"] != "ABC123" {
		t.Errorf("Context channel_id = %v, want ABC123", err.Context["channel_id"])
	}
	if err.Context["attempt"] != 3 {
		t.Errorf("Context attempt = %v, want 3", err.Context["attempt"])
	}
}

func TestIsAppError(t *testing.T) {
	appErr := Config("test", nil)
	regularErr := fmt.Errorf("regular error")

	if !IsAppError(appErr) {
		t.Error("IsAppError() should return true for AppError")
	}
	if IsAppError(regularErr) {
		t.Error("IsAppError() should return false for regular error")
	}
}

func TestGetType(t *testing.T) {
	appErr := API("test", nil)
	regularErr := fmt.Errorf("regular error")

	if errType, ok := GetType(appErr); !ok || errType != ErrTypeAPI {
		t.Errorf("GetType() = %v, %v, want %v, true", errType, ok, ErrTypeAPI)
	}
	if errType, ok := GetType(regularErr); ok {
		t.Errorf("GetType() = %v, %v, want '', false", errType, ok)
	}
}

func TestUnwrap(t *testing.T) {
	underlyingErr := fmt.Errorf("underlying error")
	appErr := API("wrapper", underlyingErr)

	if appErr.Unwrap() != underlyingErr {
		t.Errorf("Unwrap() = %v, want %v", appErr.Unwrap(), underlyingErr)
	}
}
