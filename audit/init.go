package audit

import (
	"os"

	"github.com/M1chlCZ/bingo-bot/logger"
)

func DefaultConfig() Config {
	return Config{
		Enabled:      false,
		LogToConsole: false,
		LogToFile:    false,
		FilePath:     "audit.log",
	}
}

func InitAuditLogger() error {
	config := DefaultConfig()

	if enabled := os.Getenv("AUDIT_ENABLED"); enabled != "" {
		config.Enabled = enabled == "true" || enabled == "1"
	}

	if logToConsole := os.Getenv("AUDIT_LOG_TO_CONSOLE"); logToConsole != "" {
		config.LogToConsole = logToConsole == "true" || logToConsole == "1"
	}

	if logToFile := os.Getenv("AUDIT_LOG_TO_FILE"); logToFile != "" {
		config.LogToFile = logToFile == "true" || logToFile == "1"
	}

	if filePath := os.Getenv("AUDIT_FILE_PATH"); filePath != "" {
		config.FilePath = filePath
	}

	err := Initialize(config)
	if err != nil {
		return err
	}

	logger.Infof("Audit logging initialized (Enabled: %v, LogToConsole: %v, LogToFile: %v, FilePath: %s)",
		config.Enabled, config.LogToConsole, config.LogToFile, config.FilePath)

	return nil
}
