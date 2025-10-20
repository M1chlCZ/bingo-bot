package errors

import (
	"github.com/M1chlCZ/bingo-bot/logger"
)

func LogError(err error, message string) {
	if err == nil {
		return
	}

	sanitizedMsg := sanitizeMessage(message)

	logger.Errorf("%s: %v", sanitizedMsg, sanitizeMessage(err.Error()))
}

func LogErrorWithFields(err error, message string, fields map[string]interface{}) {
	if err == nil {
		return
	}

	sanitizedMsg := sanitizeMessage(message)

	sanitizedFields := make(map[string]interface{})
	for k, v := range fields {

		if isSensitiveField(k) {
			sanitizedFields[k] = "REDACTED"
			continue
		}

		if strVal, ok := v.(string); ok {
			sanitizedFields[k] = sanitizeMessage(strVal)
		} else {
			sanitizedFields[k] = v
		}
	}

	sanitizedFields["error"] = sanitizeMessage(err.Error())

	logger.ErrorWithFields(sanitizedMsg, sanitizedFields)
}

func LogFatal(err error, message string) {
	if err == nil {
		return
	}

	sanitizedMsg := sanitizeMessage(message)

	logger.Fatalf("%s: %v", sanitizedMsg, sanitizeMessage(err.Error()))
}

func LogFatalWithFields(err error, message string, fields map[string]interface{}) {
	if err == nil {
		return
	}

	sanitizedMsg := sanitizeMessage(message)

	sanitizedFields := make(map[string]interface{})
	for k, v := range fields {

		if isSensitiveField(k) {
			sanitizedFields[k] = "REDACTED"
			continue
		}

		if strVal, ok := v.(string); ok {
			sanitizedFields[k] = sanitizeMessage(strVal)
		} else {
			sanitizedFields[k] = v
		}
	}

	sanitizedFields["error"] = sanitizeMessage(err.Error())

	logger.FatalWithFields(sanitizedMsg, sanitizedFields)
}

func isSensitiveField(fieldName string) bool {
	for _, pattern := range SensitiveDataPatterns {
		if pattern == fieldName {
			return true
		}
	}
	return false
}
