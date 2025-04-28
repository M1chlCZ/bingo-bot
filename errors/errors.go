package errors

import (
	"errors"
	"fmt"
	"strings"
)

// Standard error types
var (
	ErrInternal          = errors.New("internal error")
	ErrInvalidInput      = errors.New("invalid input")
	ErrNotFound          = errors.New("not found")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrForbidden         = errors.New("forbidden")
	ErrTimeout           = errors.New("timeout")
	ErrTemporaryFailure  = errors.New("temporary failure")
	ErrPermanentFailure  = errors.New("permanent failure")
	ErrConfigurationError = errors.New("configuration error")
	ErrNetworkError      = errors.New("network error")
)

// SensitiveDataPatterns contains patterns that should be redacted from error messages
var SensitiveDataPatterns = []string{
	"key",
	"secret",
	"password",
	"token",
	"auth",
	"credential",
	"api",
}

// Wrap wraps an error with a message, sanitizing any sensitive information
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	
	// Sanitize the message
	sanitizedMsg := sanitizeMessage(message)
	
	// Wrap the error
	return fmt.Errorf("%s: %w", sanitizedMsg, err)
}

// WrapWithType wraps an error with a standard error type and message
func WrapWithType(err error, errType error, message string) error {
	if err == nil {
		return nil
	}
	
	// Sanitize the message
	sanitizedMsg := sanitizeMessage(message)
	
	// Wrap with type and message
	return fmt.Errorf("%s - %s: %w", errType.Error(), sanitizedMsg, err)
}

// New creates a new error with a sanitized message
func New(message string) error {
	return errors.New(sanitizeMessage(message))
}

// NewWithType creates a new error with a standard error type and sanitized message
func NewWithType(errType error, message string) error {
	return fmt.Errorf("%s - %s", errType.Error(), sanitizeMessage(message))
}

// sanitizeMessage removes or redacts sensitive information from error messages
func sanitizeMessage(message string) string {
	// Check for sensitive patterns
	for _, pattern := range SensitiveDataPatterns {
		// Look for pattern followed by a value
		patternWithValue := fmt.Sprintf("%s[^:=]*[:=]\\s*[^\\s,;]+", pattern)
		// Replace with redacted version
		message = replacePattern(message, patternWithValue, pattern+"=REDACTED")
	}
	
	return message
}

// replacePattern is a helper function to replace patterns in strings
func replacePattern(input, pattern, replacement string) string {
	// This is a simplified implementation
	// In a real implementation, you would use regex to match and replace patterns
	
	// For now, we'll do a simple case-insensitive check
	lowerInput := strings.ToLower(input)
	for _, p := range SensitiveDataPatterns {
		if strings.Contains(lowerInput, p) {
			// Find the start of the pattern
			index := strings.Index(lowerInput, p)
			if index >= 0 {
				// Find the end of the value (space, comma, semicolon, or end of string)
				endIndex := len(input)
				for i := index + len(p); i < len(input); i++ {
					if input[i] == ' ' || input[i] == ',' || input[i] == ';' {
						endIndex = i
						break
					}
				}
				
				// Replace the sensitive part
				if endIndex > index+len(p) {
					return input[:index+len(p)] + "=REDACTED" + input[endIndex:]
				}
			}
		}
	}
	
	return input
}

// Is reports whether any error in err's chain matches target
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As finds the first error in err's chain that matches target, and if so, sets
// target to that error value and returns true. Otherwise, it returns false.
func As(err error, target interface{}) bool {
	return errors.As(err, target)
}

// Unwrap returns the result of calling the Unwrap method on err, if err's
// type contains an Unwrap method returning error.
// Otherwise, Unwrap returns nil.
func Unwrap(err error) error {
	return errors.Unwrap(err)
}