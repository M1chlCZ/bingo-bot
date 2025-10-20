package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/M1chlCZ/bingo-bot/logger"
)

type EventType string

const (
	EventTypeAccess EventType = "ACCESS"

	EventTypeModify EventType = "MODIFY"

	EventTypeCreate EventType = "CREATE"

	EventTypeDelete EventType = "DELETE"

	EventTypeAuth EventType = "AUTH"

	EventTypeTrade EventType = "TRADE"
)

type AuditEntry struct {
	Timestamp   string                 `json:"timestamp"`
	EventType   EventType              `json:"event_type"`
	Component   string                 `json:"component"`
	Operation   string                 `json:"operation"`
	User        string                 `json:"user,omitempty"`
	Resource    string                 `json:"resource"`
	Status      string                 `json:"status"`
	Description string                 `json:"description,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

type Config struct {
	Enabled      bool
	LogToConsole bool
	LogToFile    bool
	FilePath     string
}

var (
	config = Config{
		Enabled:      false,
		LogToConsole: false,
		LogToFile:    false,
		FilePath:     "audit.log",
	}
	auditFile *os.File
)

func Initialize(cfg Config) error {
	config = cfg

	if config.LogToFile {
		var err error
		auditFile, err = os.OpenFile(config.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open audit log file: %w", err)
		}
	}

	return nil
}

func Close() {
	if auditFile != nil {
		auditFile.Close()
	}
}

func Log(eventType EventType, component, operation, resource, status string, description string, details map[string]interface{}) {
	if !config.Enabled {
		return
	}

	entry := AuditEntry{
		Timestamp:   time.Now().Format(time.RFC3339),
		EventType:   eventType,
		Component:   component,
		Operation:   operation,
		Resource:    resource,
		Status:      status,
		Description: description,
		Details:     sanitizeDetails(details),
	}

	jsonData, err := json.Marshal(entry)
	if err != nil {
		logger.Errorf("Failed to marshal audit entry: %v", err)
		return
	}

	if config.LogToConsole {
		logger.InfoWithFields("AUDIT", map[string]interface{}{
			"event_type": string(eventType),
			"component":  component,
			"operation":  operation,
			"resource":   resource,
			"status":     status,
			"details":    sanitizeDetails(details),
		})
	}

	if config.LogToFile && auditFile != nil {
		if _, err := auditFile.WriteString(string(jsonData) + "\n"); err != nil {
			logger.Errorf("Failed to write to audit log file: %v", err)
		}
	}
}

func LogAccess(component, operation, resource string, success bool, description string, details map[string]interface{}) {
	status := "SUCCESS"
	if !success {
		status = "FAILURE"
	}
	Log(EventTypeAccess, component, operation, resource, status, description, details)
}

func LogModify(component, operation, resource string, success bool, description string, details map[string]interface{}) {
	status := "SUCCESS"
	if !success {
		status = "FAILURE"
	}
	Log(EventTypeModify, component, operation, resource, status, description, details)
}

func LogCreate(component, operation, resource string, success bool, description string, details map[string]interface{}) {
	status := "SUCCESS"
	if !success {
		status = "FAILURE"
	}
	Log(EventTypeCreate, component, operation, resource, status, description, details)
}

func LogDelete(component, operation, resource string, success bool, description string, details map[string]interface{}) {
	status := "SUCCESS"
	if !success {
		status = "FAILURE"
	}
	Log(EventTypeDelete, component, operation, resource, status, description, details)
}

func LogAuth(component, operation, resource string, success bool, description string, details map[string]interface{}) {
	status := "SUCCESS"
	if !success {
		status = "FAILURE"
	}
	Log(EventTypeAuth, component, operation, resource, status, description, details)
}

func LogTrade(component, operation, resource string, success bool, description string, details map[string]interface{}) {
	status := "SUCCESS"
	if !success {
		status = "FAILURE"
	}
	Log(EventTypeTrade, component, operation, resource, status, description, details)
}

func sanitizeDetails(details map[string]interface{}) map[string]interface{} {
	if details == nil {
		return nil
	}

	sanitized := make(map[string]interface{})
	for k, v := range details {

		if isSensitiveField(k) {
			sanitized[k] = "REDACTED"
		} else {
			sanitized[k] = v
		}
	}

	return sanitized
}

func isSensitiveField(fieldName string) bool {
	sensitiveFields := []string{
		"key", "secret", "password", "token", "auth", "credential", "api",
	}

	for _, field := range sensitiveFields {
		if field == fieldName {
			return true
		}
	}

	return false
}
