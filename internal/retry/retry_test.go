package retry

import (
	"context"
	"testing"
	"time"

	"github.com/lancelop89/youtube-trend-tracker/internal/errors"
)

func TestRetrySuccess(t *testing.T) {
	attempts := 0
	operation := func() error {
		attempts++
		if attempts < 3 {
			return errors.Temporary("temporary error", nil)
		}
		return nil
	}

	config := Config{
		MaxAttempts:  5,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}

	err := Do(operation, config)
	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestRetryMaxAttempts(t *testing.T) {
	attempts := 0
	operation := func() error {
		attempts++
		return errors.Temporary("always fails", nil)
	}

	config := Config{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}

	err := Do(operation, config)
	if err == nil {
		t.Error("Expected error after max attempts")
	}
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestRetryNonRetriableError(t *testing.T) {
	attempts := 0
	operation := func() error {
		attempts++
		return errors.Validation("validation error", nil)
	}

	config := Config{
		MaxAttempts:  5,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}

	err := Do(operation, config)
	if err == nil {
		t.Error("Expected error for non-retriable error")
	}
	if attempts != 1 {
		t.Errorf("Expected 1 attempt for non-retriable error, got %d", attempts)
	}
}

func TestRetryWithContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	attempts := 0
	operation := func(ctx context.Context) error {
		attempts++
		return errors.Temporary("temporary error", nil)
	}

	config := Config{
		MaxAttempts:  10,
		InitialDelay: 30 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	}

	err := DoWithContext(ctx, operation, config)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected context deadline exceeded, got %v", err)
	}
	// Should have attempted at least once, but not all 10 times due to timeout
	if attempts == 0 || attempts >= 10 {
		t.Errorf("Unexpected number of attempts: %d", attempts)
	}
}

func TestCalculateBackoff(t *testing.T) {
	config := Config{
		InitialDelay: 1 * time.Second,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 1 * time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{5, 10 * time.Second}, // Capped at MaxDelay
		{6, 10 * time.Second}, // Still capped
	}

	for _, tt := range tests {
		got := CalculateBackoff(tt.attempt, config)
		if got != tt.expected {
			t.Errorf("CalculateBackoff(%d) = %v, want %v", tt.attempt, got, tt.expected)
		}
	}
}

func TestWithExponentialBackoff(t *testing.T) {
	attempts := 0
	operation := func() error {
		attempts++
		if attempts < 2 {
			return errors.Temporary("temporary error", nil)
		}
		return nil
	}

	err := WithExponentialBackoff(operation)
	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}
	if attempts < 2 {
		t.Errorf("Expected at least 2 attempts, got %d", attempts)
	}
}
