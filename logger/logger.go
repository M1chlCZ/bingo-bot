package logger

import (
	"log"
	"os"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

var currentLogLevel LogLevel

var logger = log.New(os.Stdout, "", log.Ldate|log.Ltime)

func InitLogger(logLevel *string) {
	switch *logLevel {
	case "debug":
		SetLogLevel(DEBUG)
	case "info":
		SetLogLevel(INFO)
	case "warn":
		SetLogLevel(WARN)
	case "error":
		SetLogLevel(ERROR)
	default:
		SetLogLevel(INFO)
	}

	Info("Application started")
	Debug("This is a debug message")
}

// SetLogLevel sets the global log level
func SetLogLevel(level LogLevel) {
	currentLogLevel = level
}

// Debug logs debug-level messages
func Debug(v ...interface{}) {
	if currentLogLevel <= DEBUG {
		logger.SetPrefix("[DEBUG] ")
		logger.Println(v...)
	}
}

// Debugf logs debug-level formatted messages
func Debugf(format string, v ...interface{}) {
	if currentLogLevel <= DEBUG {
		logger.SetPrefix("[DEBUG] ")
		logger.Printf(format, v...)
	}
}

// Info logs info-level messages
func Info(v ...interface{}) {
	if currentLogLevel <= INFO {
		logger.SetPrefix("[INFO] ")
		logger.Println(v...)
	}
}

// Infof logs info-level formatted messages
func Infof(format string, v ...interface{}) {
	if currentLogLevel <= INFO {
		logger.SetPrefix("[INFO] ")
		logger.Printf(format, v...)
	}
}

// Warn logs warning-level messages
func Warn(v ...interface{}) {
	if currentLogLevel <= WARN {
		logger.SetPrefix("[WARN] ")
		logger.Println(v...)
	}
}

// Warnf logs warning-level formatted messages
func Warnf(format string, v ...interface{}) {
	if currentLogLevel <= WARN {
		logger.SetPrefix("[WARN] ")
		logger.Printf(format, v...)
	}
}

// Error logs error-level messages
func Error(v ...interface{}) {
	if currentLogLevel <= ERROR {
		logger.SetPrefix("[ERROR] ")
		logger.Println(v...)
	}
}

// Errorf logs error-level formatted messages
func Errorf(format string, v ...interface{}) {
	if currentLogLevel <= ERROR {
		logger.SetPrefix("[ERROR] ")
		logger.Printf(format, v...)
	}
}
