package logger

import (
	"fmt"
	"log"
	"os"
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
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

var currentLogLevel LogLevel
var enableColors ColorsEnabled

var logger = log.New(os.Stdout, "", log.Ldate|log.Ltime)

func InitLogger(logLevel *string, colorEnabled *bool) {
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

	if colorEnabled == nil {
		SetColorEnabled(true)
	} else {
		SetColorEnabled(ColorsEnabled(*colorEnabled))
	}

	Info("Application started")
	Debug("This is a debug message")
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

func Fatalf(s string, err error) {
	logger.Fatalf(s, err)
}
