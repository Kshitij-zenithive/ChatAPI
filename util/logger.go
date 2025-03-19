package util

import (
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strings"
	"time"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	// DEBUG level
	DEBUG LogLevel = iota
	// INFO level
	INFO
	// WARN level
	WARN
	// ERROR level
	ERROR
	// FATAL level
	FATAL
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
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

// Logger is a simple logging utility
type Logger struct {
	minLevel LogLevel
	out      io.Writer
}

// NewLogger creates a new logger
func NewLogger() *Logger {
	// Determine minimum log level from environment
	levelStr := strings.ToUpper(os.Getenv("LOG_LEVEL"))
	var minLevel LogLevel
	
	switch levelStr {
	case "DEBUG":
		minLevel = DEBUG
	case "INFO":
		minLevel = INFO
	case "WARN":
		minLevel = WARN
	case "ERROR":
		minLevel = ERROR
	case "FATAL":
		minLevel = FATAL
	default:
		minLevel = INFO // Default to INFO
	}
	
	return &Logger{
		minLevel: minLevel,
		out:      os.Stdout,
	}
}

// SetOutput sets the output destination for the logger
func (l *Logger) SetOutput(w io.Writer) {
	l.out = w
}

// SetLevel sets the minimum log level
func (l *Logger) SetLevel(level LogLevel) {
	l.minLevel = level
}

// log formats and logs a message at the specified level
func (l *Logger) log(level LogLevel, msg string, keyvals ...interface{}) {
	if level < l.minLevel {
		return
	}
	
	// Get caller info
	_, file, line, ok := runtime.Caller(2)
	caller := "unknown"
	if ok {
		// Extract just the file name, not the full path
		parts := strings.Split(file, "/")
		caller = fmt.Sprintf("%s:%d", parts[len(parts)-1], line)
	}
	
	// Format time
	now := time.Now().Format("2006-01-02 15:04:05.000")
	
	// Format key-value pairs
	kvPairs := ""
	for i := 0; i < len(keyvals); i += 2 {
		var v interface{} = "MISSING"
		if i+1 < len(keyvals) {
			v = keyvals[i+1]
		}
		kvPairs += fmt.Sprintf(" %v=%v", keyvals[i], v)
	}
	
	// Format log message
	logLine := fmt.Sprintf("[%s] %s %-5s %s%s\n", now, caller, level, msg, kvPairs)
	
	fmt.Fprint(l.out, logLine)
	
	// Exit program for fatal errors
	if level == FATAL {
		os.Exit(1)
	}
}

// Debug logs a message at DEBUG level
func (l *Logger) Debug(msg string, keyvals ...interface{}) {
	l.log(DEBUG, msg, keyvals...)
}

// Info logs a message at INFO level
func (l *Logger) Info(msg string, keyvals ...interface{}) {
	l.log(INFO, msg, keyvals...)
}

// Warn logs a message at WARN level
func (l *Logger) Warn(msg string, keyvals ...interface{}) {
	l.log(WARN, msg, keyvals...)
}

// Error logs a message at ERROR level
func (l *Logger) Error(msg string, keyvals ...interface{}) {
	l.log(ERROR, msg, keyvals...)
}

// Fatal logs a message at FATAL level and exits
func (l *Logger) Fatal(msg string, keyvals ...interface{}) {
	l.log(FATAL, msg, keyvals...)
}
