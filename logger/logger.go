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

	if logLevel == nil {
		SetLogLevel(INFO)
	} else {
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
	}

	if colorEnabled == nil {
		SetColorEnabled(true)
	} else {
		SetColorEnabled(ColorsEnabled(*colorEnabled))
	}

	Info("Application started")
	Debug("This is a debug message")
}

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

		return true
	}
	return def
}

func SetLogLevel(level LogLevel) {
	currentLogLevel = level
}

func SetColorEnabled(colors ColorsEnabled) {
	enableColors = colors
}

func colorize(color Color, s string, params ...any) string {
	if !enableColors {
		return fmt.Sprintf(s, params...)
	}
	return color.String() + fmt.Sprintf(s, params...) + Reset.String()
}

func Trace(v ...interface{}) {
	if currentLogLevel <= TRACE {
		logger.SetPrefix("[TRACE] ")
		logger.Println(v...)
	}
}

func TraceColor(color Color, v ...interface{}) {
	if currentLogLevel <= TRACE {
		logger.SetPrefix("[TRACE] ")
		logger.Println(colorize(color, fmt.Sprint(v...)))
	}
}

func Tracef(format string, v ...interface{}) {
	if currentLogLevel <= TRACE {
		logger.SetPrefix("[TRACE] ")
		logger.Printf(format, v...)
	}
}

func TraceColorf(color Color, format string, v ...interface{}) {
	if currentLogLevel <= TRACE {
		logger.SetPrefix("[TRACE] ")
		logger.Println(colorize(color, format, v...))
	}
}

func TraceWithFields(msg string, fields map[string]interface{}) {
	LogWithFields(TRACE, msg, fields)
}

func Debug(v ...interface{}) {
	if currentLogLevel <= DEBUG {
		logger.SetPrefix("[DEBUG] ")
		logger.Println(v...)
	}
}

func DebugColor(color Color, v ...interface{}) {
	if currentLogLevel <= DEBUG {
		logger.SetPrefix("[DEBUG] ")
		logger.Println(colorize(color, fmt.Sprint(v...)))
	}
}

func Debugf(format string, v ...interface{}) {
	if currentLogLevel <= DEBUG {
		logger.SetPrefix("[DEBUG] ")
		logger.Printf(format, v...)
	}
}

func DebugColorf(color Color, format string, v ...interface{}) {
	if currentLogLevel <= DEBUG {
		logger.SetPrefix("[DEBUG] ")
		logger.Println(colorize(color, format, v...))
	}
}

func DebugWithFields(msg string, fields map[string]interface{}) {
	LogWithFields(DEBUG, msg, fields)
}

func Info(v ...interface{}) {
	if currentLogLevel <= INFO {
		logger.SetPrefix("[INFO] ")
		logger.Println(v...)
	}
}

func InfoColor(color Color, v ...interface{}) {
	if currentLogLevel <= INFO {
		logger.SetPrefix("[INFO] ")
		logger.Println(colorize(color, fmt.Sprint(v...)))
	}
}

func Infof(format string, v ...interface{}) {
	if currentLogLevel <= INFO {
		logger.SetPrefix("[INFO] ")
		logger.Printf(format, v...)
	}
}

func InfoColorf(color Color, format string, v ...interface{}) {
	if currentLogLevel <= INFO {
		logger.SetPrefix("[INFO] ")
		logger.Println(colorize(color, format, v...))
	}
}

func InfoWithFields(msg string, fields map[string]interface{}) {
	return

}

func Warn(v ...interface{}) {
	if currentLogLevel <= WARN {
		logger.SetPrefix("[WARN] ")
		logger.Println(colorize(Yellow, fmt.Sprint(v...)))
	}
}

func Warnf(format string, v ...interface{}) {
	if currentLogLevel <= WARN {
		logger.SetPrefix("[WARN] ")
		logger.Println(colorize(Yellow, format, v...))
	}
}

func WarnWithFields(msg string, fields map[string]interface{}) {
	LogWithFields(WARN, msg, fields)
}

func Error(v ...interface{}) {
	if currentLogLevel <= ERROR {
		logger.SetPrefix("[ERROR] ")
		logger.Println(colorize(Red, fmt.Sprint(v...)))
	}
}

func Errorf(format string, v ...interface{}) {
	if currentLogLevel <= ERROR {
		logger.SetPrefix("[ERROR] ")
		logger.Println(colorize(Red, format, v...))
	}
}

func ErrorWithFields(msg string, fields map[string]interface{}) {
	return

}

func Fatal(v ...interface{}) {
	if currentLogLevel <= FATAL {
		logger.SetPrefix("[FATAL] ")
		logger.Println(colorize(BrightRed, fmt.Sprint(v...)))
		os.Exit(1)
	}
}

func Fatalf(format string, v ...interface{}) {
	if currentLogLevel <= FATAL {
		logger.SetPrefix("[FATAL] ")
		logger.Println(colorize(BrightRed, format, v...))
		os.Exit(1)
	}
}

func FatalWithFields(msg string, fields map[string]interface{}) {
	LogWithFields(FATAL, msg, fields)
	os.Exit(1)
}
