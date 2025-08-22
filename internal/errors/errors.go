package errors

import (
	"fmt"
	"time"
)

// ErrorType represents the category of error
type ErrorType string

const (
	// Configuration errors
	ErrTypeConfig ErrorType = "CONFIG"
	// API related errors
	ErrTypeAPI ErrorType = "API"
	// Database/Storage errors
	ErrTypeStorage ErrorType = "STORAGE"
	// Validation errors
	ErrTypeValidation ErrorType = "VALIDATION"
	// Temporary/Retriable errors
	ErrTypeTemporary ErrorType = "TEMPORARY"
)

// AppError represents a structured application error
type AppError struct {
	Type      ErrorType
	Message   string
	Err       error
	Timestamp time.Time
	Context   map[string]interface{}
	Retriable bool
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

// Unwrap allows errors.Is and errors.As to work
func (e *AppError) Unwrap() error {
	return e.Err
}

// IsRetriable returns whether the error is retriable
func (e *AppError) IsRetriable() bool {
	return e.Retriable
}

// New creates a new AppError
func New(errType ErrorType, message string, err error) *AppError {
	return &AppError{
		Type:      errType,
		Message:   message,
		Err:       err,
		Timestamp: time.Now(),
		Context:   make(map[string]interface{}),
		Retriable: errType == ErrTypeTemporary,
	}
}

// NewWithContext creates a new AppError with context
func NewWithContext(errType ErrorType, message string, err error, context map[string]interface{}) *AppError {
	return &AppError{
		Type:      errType,
		Message:   message,
		Err:       err,
		Timestamp: time.Now(),
		Context:   context,
		Retriable: errType == ErrTypeTemporary,
	}
}

// Config creates a configuration error
func Config(message string, err error) *AppError {
	return New(ErrTypeConfig, message, err)
}

// API creates an API error
func API(message string, err error) *AppError {
	return New(ErrTypeAPI, message, err)
}

// Storage creates a storage error
func Storage(message string, err error) *AppError {
	return New(ErrTypeStorage, message, err)
}

// Validation creates a validation error
func Validation(message string, err error) *AppError {
	return New(ErrTypeValidation, message, err)
}

// Temporary creates a temporary/retriable error
func Temporary(message string, err error) *AppError {
	e := New(ErrTypeTemporary, message, err)
	e.Retriable = true
	return e
}

// IsAppError checks if an error is an AppError
func IsAppError(err error) bool {
	_, ok := err.(*AppError)
	return ok
}

// GetType returns the error type if it's an AppError
func GetType(err error) (ErrorType, bool) {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Type, true
	}
	return "", false
}