package errors

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrInternal           = errors.New("internal error")
	ErrInvalidInput       = errors.New("invalid input")
	ErrNotFound           = errors.New("not found")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
	ErrTimeout            = errors.New("timeout")
	ErrTemporaryFailure   = errors.New("temporary failure")
	ErrPermanentFailure   = errors.New("permanent failure")
	ErrConfigurationError = errors.New("configuration error")
	ErrNetworkError       = errors.New("network error")
)

var SensitiveDataPatterns = []string{
	"key",
	"secret",
	"password",
	"token",
	"auth",
	"credential",
	"api",
}

func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}

	sanitizedMsg := sanitizeMessage(message)

	return fmt.Errorf("%s: %w", sanitizedMsg, err)
}

func WrapWithType(err error, errType error, message string) error {
	if err == nil {
		return nil
	}

	sanitizedMsg := sanitizeMessage(message)

	return fmt.Errorf("%s - %s: %w", errType.Error(), sanitizedMsg, err)
}

func New(message string) error {
	return errors.New(sanitizeMessage(message))
}

func NewWithType(errType error, message string) error {
	return fmt.Errorf("%s - %s", errType.Error(), sanitizeMessage(message))
}

func sanitizeMessage(message string) string {

	for _, pattern := range SensitiveDataPatterns {

		patternWithValue := fmt.Sprintf("%s[^:=]*[:=]\\s*[^\\s,;]+", pattern)

		message = replacePattern(message, patternWithValue, pattern+"=REDACTED")
	}

	return message
}

func replacePattern(input, pattern, replacement string) string {

	lowerInput := strings.ToLower(input)
	for _, p := range SensitiveDataPatterns {
		if strings.Contains(lowerInput, p) {

			index := strings.Index(lowerInput, p)
			if index >= 0 {

				endIndex := len(input)
				for i := index + len(p); i < len(input); i++ {
					if input[i] == ' ' || input[i] == ',' || input[i] == ';' {
						endIndex = i
						break
					}
				}

				if endIndex > index+len(p) {
					return input[:index+len(p)] + "=REDACTED" + input[endIndex:]
				}
			}
		}
	}

	return input
}

func Is(err, target error) bool {
	return errors.Is(err, target)
}

func As(err error, target interface{}) bool {
	return errors.As(err, target)
}

func Unwrap(err error) error {
	return errors.Unwrap(err)
}
