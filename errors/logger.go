package errors

import (
	"github.com/M1chlCZ/bingo-bot/logger"
)

// LogError logs an error with a sanitized message
func LogError(err error, message string) {
	if err == nil {
		return
	}
	
	// Sanitize the message
	sanitizedMsg := sanitizeMessage(message)
	
	// Log the error
	logger.Errorf("%s: %v", sanitizedMsg, sanitizeMessage(err.Error()))
}

// LogErrorWithFields logs an error with structured fields, sanitizing sensitive information
func LogErrorWithFields(err error, message string, fields map[string]interface{}) {
	if err == nil {
		return
	}
	
	// Sanitize the message
	sanitizedMsg := sanitizeMessage(message)
	
	// Sanitize fields
	sanitizedFields := make(map[string]interface{})
	for k, v := range fields {
		// Skip sensitive fields
		if isSensitiveField(k) {
			sanitizedFields[k] = "REDACTED"
			continue
		}
		
		// Handle string values that might contain sensitive information
		if strVal, ok := v.(string); ok {
			sanitizedFields[k] = sanitizeMessage(strVal)
		} else {
			sanitizedFields[k] = v
		}
	}
	
	// Add error to fields
	sanitizedFields["error"] = sanitizeMessage(err.Error())
	
	// Log with structured fields
	logger.ErrorWithFields(sanitizedMsg, sanitizedFields)
}

// LogFatal logs a fatal error with a sanitized message and exits the program
func LogFatal(err error, message string) {
	if err == nil {
		return
	}
	
	// Sanitize the message
	sanitizedMsg := sanitizeMessage(message)
	
	// Log the fatal error
	logger.Fatalf("%s: %v", sanitizedMsg, sanitizeMessage(err.Error()))
}

// LogFatalWithFields logs a fatal error with structured fields, sanitizing sensitive information, and exits the program
func LogFatalWithFields(err error, message string, fields map[string]interface{}) {
	if err == nil {
		return
	}
	
	// Sanitize the message
	sanitizedMsg := sanitizeMessage(message)
	
	// Sanitize fields
	sanitizedFields := make(map[string]interface{})
	for k, v := range fields {
		// Skip sensitive fields
		if isSensitiveField(k) {
			sanitizedFields[k] = "REDACTED"
			continue
		}
		
		// Handle string values that might contain sensitive information
		if strVal, ok := v.(string); ok {
			sanitizedFields[k] = sanitizeMessage(strVal)
		} else {
			sanitizedFields[k] = v
		}
	}
	
	// Add error to fields
	sanitizedFields["error"] = sanitizeMessage(err.Error())
	
	// Log with structured fields
	logger.FatalWithFields(sanitizedMsg, sanitizedFields)
}

// isSensitiveField checks if a field name is sensitive
func isSensitiveField(fieldName string) bool {
	for _, pattern := range SensitiveDataPatterns {
		if pattern == fieldName {
			return true
		}
	}
	return false
}