package validation

import (
	"fmt"
	"strings"
)

func ValidateLogLevel(level string) error {
	if level == "" {
		return fmt.Errorf("log level cannot be empty")
	}

	level = strings.ToLower(level)
	validLevels := map[string]bool{
		"trace": true,
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
		"fatal": true,
	}

	if !validLevels[level] {
		return fmt.Errorf("invalid log level: %s (must be one of trace, debug, info, warn, error, fatal)", level)
	}

	return nil
}
