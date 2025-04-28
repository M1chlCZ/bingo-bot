package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/M1chlCZ/bingo-bot/logger"
)

// EventType represents the type of security event being audited
type EventType string

const (
	// EventTypeAccess represents access to sensitive information
	EventTypeAccess EventType = "ACCESS"
	
	// EventTypeModify represents modification of sensitive information
	EventTypeModify EventType = "MODIFY"
	
	// EventTypeCreate represents creation of sensitive information
	EventTypeCreate EventType = "CREATE"
	
	// EventTypeDelete represents deletion of sensitive information
	EventTypeDelete EventType = "DELETE"
	
	// EventTypeAuth represents authentication events
	EventTypeAuth EventType = "AUTH"
	
	// EventTypeTrade represents trading operations
	EventTypeTrade EventType = "TRADE"
)

// AuditEntry represents a structured audit log entry
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

// Config holds configuration for the audit logger
type Config struct {
	Enabled      bool
	LogToConsole bool
	LogToFile    bool
	FilePath     string
}

var (
	config = Config{
		Enabled:      true,
		LogToConsole: true,
		LogToFile:    false,
		FilePath:     "audit.log",
	}
	auditFile *os.File
)

// Initialize sets up the audit logger with the provided configuration
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

// Close closes any open resources used by the audit logger
func Close() {
	if auditFile != nil {
		auditFile.Close()
	}
}

// Log logs a security-sensitive operation
func Log(eventType EventType, component, operation, resource, status string, description string, details map[string]interface{}) {
	if !config.Enabled {
		return
	}
	
	// Create audit entry
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
	
	// Convert to JSON
	jsonData, err := json.Marshal(entry)
	if err != nil {
		logger.Errorf("Failed to marshal audit entry: %v", err)
		return
	}
	
	// Log to console
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
	
	// Log to file
	if config.LogToFile && auditFile != nil {
		if _, err := auditFile.WriteString(string(jsonData) + "\n"); err != nil {
			logger.Errorf("Failed to write to audit log file: %v", err)
		}
	}
}

// LogAccess logs access to sensitive information
func LogAccess(component, operation, resource string, success bool, description string, details map[string]interface{}) {
	status := "SUCCESS"
	if !success {
		status = "FAILURE"
	}
	Log(EventTypeAccess, component, operation, resource, status, description, details)
}

// LogModify logs modification of sensitive information
func LogModify(component, operation, resource string, success bool, description string, details map[string]interface{}) {
	status := "SUCCESS"
	if !success {
		status = "FAILURE"
	}
	Log(EventTypeModify, component, operation, resource, status, description, details)
}

// LogCreate logs creation of sensitive information
func LogCreate(component, operation, resource string, success bool, description string, details map[string]interface{}) {
	status := "SUCCESS"
	if !success {
		status = "FAILURE"
	}
	Log(EventTypeCreate, component, operation, resource, status, description, details)
}

// LogDelete logs deletion of sensitive information
func LogDelete(component, operation, resource string, success bool, description string, details map[string]interface{}) {
	status := "SUCCESS"
	if !success {
		status = "FAILURE"
	}
	Log(EventTypeDelete, component, operation, resource, status, description, details)
}

// LogAuth logs authentication events
func LogAuth(component, operation, resource string, success bool, description string, details map[string]interface{}) {
	status := "SUCCESS"
	if !success {
		status = "FAILURE"
	}
	Log(EventTypeAuth, component, operation, resource, status, description, details)
}

// LogTrade logs trading operations
func LogTrade(component, operation, resource string, success bool, description string, details map[string]interface{}) {
	status := "SUCCESS"
	if !success {
		status = "FAILURE"
	}
	Log(EventTypeTrade, component, operation, resource, status, description, details)
}

// sanitizeDetails removes sensitive information from details
func sanitizeDetails(details map[string]interface{}) map[string]interface{} {
	if details == nil {
		return nil
	}
	
	sanitized := make(map[string]interface{})
	for k, v := range details {
		// Redact sensitive fields
		if isSensitiveField(k) {
			sanitized[k] = "REDACTED"
		} else {
			sanitized[k] = v
		}
	}
	
	return sanitized
}

// isSensitiveField checks if a field name is sensitive
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