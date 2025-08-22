package retry

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/lancelop89/youtube-trend-tracker/internal/errors"
	"github.com/lancelop89/youtube-trend-tracker/internal/logger"
)

var log = logger.New()

// Config holds retry configuration
type Config struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
}

// DefaultConfig returns a default retry configuration
func DefaultConfig() Config {
	return Config{
		MaxAttempts:  5,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
	}
}

// Operation is a function that can be retried
type Operation func() error

// OperationWithContext is a function that can be retried with context
type OperationWithContext func(ctx context.Context) error

// Do executes an operation with retry logic
func Do(operation Operation, config Config) error {
	return DoWithContext(context.Background(), func(ctx context.Context) error {
		return operation()
	}, config)
}

// DoWithContext executes an operation with retry logic and context
func DoWithContext(ctx context.Context, operation OperationWithContext, config Config) error {
	var lastErr error
	delay := config.InitialDelay

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Execute operation
		err := operation(ctx)
		if err == nil {
			if attempt > 1 {
				log.Info(fmt.Sprintf("Operation succeeded after %d attempts", attempt), nil)
			}
			return nil
		}

		lastErr = err

		// Check if error is retriable
		if appErr, ok := err.(*errors.AppError); ok {
			if !appErr.IsRetriable() {
				log.Error(fmt.Sprintf("Non-retriable error occurred: %v", err), err, nil)
				return err
			}
		}

		// Don't retry on last attempt
		if attempt == config.MaxAttempts {
			break
		}

		// Log retry attempt
		log.Warning(fmt.Sprintf("Attempt %d/%d failed, retrying in %v", attempt, config.MaxAttempts, delay), err, map[string]string{
			"attempt": fmt.Sprintf("%d", attempt),
			"delay":   delay.String(),
		})

		// Wait before retry
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}

		// Calculate next delay with exponential backoff
		delay = time.Duration(float64(delay) * config.Multiplier)
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w", config.MaxAttempts, lastErr)
}

// WithExponentialBackoff is a helper function for common exponential backoff retry
func WithExponentialBackoff(operation Operation) error {
	return Do(operation, DefaultConfig())
}

// WithCustomBackoff allows custom backoff configuration
func WithCustomBackoff(operation Operation, maxAttempts int, initialDelay time.Duration) error {
	config := Config{
		MaxAttempts:  maxAttempts,
		InitialDelay: initialDelay,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
	}
	return Do(operation, config)
}

// IsMaxRetriesExceeded checks if an error is due to max retries being exceeded
func IsMaxRetriesExceeded(err error) bool {
	if err == nil {
		return false
	}
	// Simple check - could be enhanced
	return fmt.Sprintf("%v", err)[:9] == "operation"
}

// CalculateBackoff calculates the backoff duration for a given attempt
func CalculateBackoff(attempt int, config Config) time.Duration {
	if attempt <= 0 {
		return config.InitialDelay
	}

	delay := config.InitialDelay * time.Duration(math.Pow(config.Multiplier, float64(attempt-1)))
	if delay > config.MaxDelay {
		return config.MaxDelay
	}
	return delay
}
