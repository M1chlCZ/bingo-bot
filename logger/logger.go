package logger

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

const (
	Reset   Color = "\033[0m"
	Black   Color = "\033[30m"
	Red     Color = "\033[31m"
	Green   Color = "\033[32m"
	Yellow  Color = "\033[33m"
	Blue    Color = "\033[34m"
	Magenta Color = "\033[35m"
	Cyan    Color = "\033[36m"
	White   Color = "\033[37m"

	BrightBlack   Color = "\033[90m"
	BrightRed     Color = "\033[91m"
	BrightGreen   Color = "\033[92m"
	BrightYellow  Color = "\033[93m"
	BrightBlue    Color = "\033[94m"
	BrightMagenta Color = "\033[95m"
	BrightCyan    Color = "\033[96m"
	BrightWhite   Color = "\033[97m"
)

type LogLevel int
type ColorsEnabled bool

const (
	TRACE LogLevel = iota
	DEBUG
	INFO
	WARN
	ERROR
	FATAL
)

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

var currentLogLevel LogLevel
var enableColors ColorsEnabled

var logger = log.New(os.Stdout, "", log.Ldate|log.Ltime)

func InitLogger(logLevel *string, colorEnabled *bool) {
	switch *logLevel {
	case "trace":
		SetLogLevel(TRACE)
	case "debug":
		SetLogLevel(DEBUG)
	case "info":
		SetLogLevel(INFO)
	case "warn":
		SetLogLevel(WARN)
	case "error":
		SetLogLevel(ERROR)
	case "fatal":
		SetLogLevel(FATAL)
	default:
		SetLogLevel(INFO)
	}

	if colorEnabled == nil {
		SetColorEnabled(true)
	} else {
		SetColorEnabled(ColorsEnabled(*colorEnabled))
	}

	Info("Application started")
	Debug("This is a debug message")
}

// LogWithFields logs a message with structured fields
func LogWithFields(level LogLevel, msg string, fields map[string]interface{}) {
	if level < currentLogLevel {
		return
	}

	entry := LogEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		Level:     getLevelString(level),
		Message:   msg,
		Fields:    fields,
	}

	if jsonData, err := json.Marshal(entry); err == nil {
		fmt.Println(string(jsonData))
	} else {
		Error("Failed to marshal log entry:", err)
	}
}

// getLevelString returns the string representation of a log level
func getLevelString(level LogLevel) string {
	switch level {
	case TRACE:
		return "TRACE"
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

func EnvOrDefault(key, def string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return def
}

func EnvOrDefaultBool(key string, def bool) bool {
	if val, ok := os.LookupEnv(key); ok {
		if val == "false" || val == "0" {
			return false
		}
		// otherwise treat anything else as true
		return true
	}
	return def
}

// SetLogLevel sets the global log level
func SetLogLevel(level LogLevel) {
	currentLogLevel = level
}

// SetColorEnabled sets the global color enabled flag
func SetColorEnabled(colors ColorsEnabled) {
	enableColors = colors
}

func colorize(color Color, s string, params ...any) string {
	if !enableColors {
		return fmt.Sprintf(s, params...)
	}
	return color.String() + fmt.Sprintf(s, params...) + Reset.String()
}

// Trace logs trace-level messages
func Trace(v ...interface{}) {
	if currentLogLevel <= TRACE {
		logger.SetPrefix("[TRACE] ")
		logger.Println(v...)
	}
}

// TraceColor logs trace-level messages
func TraceColor(color Color, v ...interface{}) {
	if currentLogLevel <= TRACE {
		logger.SetPrefix("[TRACE] ")
		logger.Println(colorize(color, fmt.Sprint(v...)))
	}
}

// Tracef logs trace-level formatted messages
func Tracef(format string, v ...interface{}) {
	if currentLogLevel <= TRACE {
		logger.SetPrefix("[TRACE] ")
		logger.Printf(format, v...)
	}
}

// TraceColorf logs trace-level formatted messages in color
func TraceColorf(color Color, format string, v ...interface{}) {
	if currentLogLevel <= TRACE {
		logger.SetPrefix("[TRACE] ")
		logger.Println(colorize(color, format, v...))
	}
}

// TraceWithFields logs trace-level messages with structured fields
func TraceWithFields(msg string, fields map[string]interface{}) {
	LogWithFields(TRACE, msg, fields)
}

// Debug logs debug-level messages
func Debug(v ...interface{}) {
	if currentLogLevel <= DEBUG {
		logger.SetPrefix("[DEBUG] ")
		logger.Println(v...)
	}
}

// DebugColor logs debug-level messages
func DebugColor(color Color, v ...interface{}) {
	if currentLogLevel <= DEBUG {
		logger.SetPrefix("[DEBUG] ")
		logger.Println(colorize(color, fmt.Sprint(v...)))
	}
}

// Debugf logs debug-level formatted messages
func Debugf(format string, v ...interface{}) {
	if currentLogLevel <= DEBUG {
		logger.SetPrefix("[DEBUG] ")
		logger.Printf(format, v...)
	}
}

// DebugColorf logs debug-level formatted messages in color
func DebugColorf(color Color, format string, v ...interface{}) {
	if currentLogLevel <= DEBUG {
		logger.SetPrefix("[DEBUG] ")
		logger.Println(colorize(color, format, v...))
	}
}

// DebugWithFields logs debug-level messages with structured fields
func DebugWithFields(msg string, fields map[string]interface{}) {
	LogWithFields(DEBUG, msg, fields)
}

// Info logs info-level messages
func Info(v ...interface{}) {
	if currentLogLevel <= INFO {
		logger.SetPrefix("[INFO] ")
		logger.Println(v...)
	}
}

// InfoColor logs info-level messages in Color
func InfoColor(color Color, v ...interface{}) {
	if currentLogLevel <= INFO {
		logger.SetPrefix("[INFO] ")
		logger.Println(colorize(color, fmt.Sprint(v...)))
	}
}

// Infof logs info-level formatted messages
func Infof(format string, v ...interface{}) {
	if currentLogLevel <= INFO {
		logger.SetPrefix("[INFO] ")
		logger.Printf(format, v...)
	}
}

// InfoColorf logs info-level formatted messages in Color
func InfoColorf(color Color, format string, v ...interface{}) {
	if currentLogLevel <= INFO {
		logger.SetPrefix("[INFO] ")
		logger.Println(colorize(color, format, v...))
	}
}

// InfoWithFields logs info-level messages with structured fields
func InfoWithFields(msg string, fields map[string]interface{}) {
	LogWithFields(INFO, msg, fields)
}

// Warn logs warning-level messages
func Warn(v ...interface{}) {
	if currentLogLevel <= WARN {
		logger.SetPrefix("[WARN] ")
		logger.Println(colorize(Yellow, fmt.Sprint(v...)))
	}
}

// Warnf logs warning-level formatted messages
func Warnf(format string, v ...interface{}) {
	if currentLogLevel <= WARN {
		logger.SetPrefix("[WARN] ")
		logger.Println(colorize(Yellow, format, v...))
	}
}

// WarnWithFields logs warning-level messages with structured fields
func WarnWithFields(msg string, fields map[string]interface{}) {
	LogWithFields(WARN, msg, fields)
}

// Error logs error-level messages
func Error(v ...interface{}) {
	if currentLogLevel <= ERROR {
		logger.SetPrefix("[ERROR] ")
		logger.Println(colorize(Red, fmt.Sprint(v...)))
	}
}

// Errorf logs error-level formatted messages
func Errorf(format string, v ...interface{}) {
	if currentLogLevel <= ERROR {
		logger.SetPrefix("[ERROR] ")
		logger.Println(colorize(Red, format, v...))
	}
}

// ErrorWithFields logs error-level messages with structured fields
func ErrorWithFields(msg string, fields map[string]interface{}) {
	LogWithFields(ERROR, msg, fields)
}

// Fatal logs fatal-level messages and exits the program
func Fatal(v ...interface{}) {
	if currentLogLevel <= FATAL {
		logger.SetPrefix("[FATAL] ")
		logger.Println(colorize(BrightRed, fmt.Sprint(v...)))
		os.Exit(1)
	}
}

// Fatalf logs fatal-level formatted messages and exits the program
func Fatalf(format string, v ...interface{}) {
	if currentLogLevel <= FATAL {
		logger.SetPrefix("[FATAL] ")
		logger.Println(colorize(BrightRed, format, v...))
		os.Exit(1)
	}
}

// FatalWithFields logs fatal-level messages with structured fields and exits the program
func FatalWithFields(msg string, fields map[string]interface{}) {
	LogWithFields(FATAL, msg, fields)
	os.Exit(1)
}
