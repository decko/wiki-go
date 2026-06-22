package logger

import (
	"log"
	"os"
	"strings"
)

type level int

const (
	levelDebug level = iota
	levelInfo
	levelWarn
	levelError
)

var currentLevel = levelInfo

// Init sets the minimum log level. Accepted values: "debug", "info", "warn", "error".
// Unrecognised values fall back to "info".
func Init(lvl string) {
	switch strings.ToLower(lvl) {
	case "debug":
		currentLevel = levelDebug
	case "warn":
		currentLevel = levelWarn
	case "error":
		currentLevel = levelError
	default:
		currentLevel = levelInfo
	}
}

// Info logs an informational message (startup progress, normal operations).
func Info(format string, v ...any) {
	if currentLevel <= levelInfo {
		log.Printf("[INFO] "+format, v...)
	}
}

// Debug logs a verbose diagnostic message (request tracing, path details).
func Debug(format string, v ...any) {
	if currentLevel <= levelDebug {
		log.Printf("[DEBUG] "+format, v...)
	}
}

// Warn logs a warning that the application can recover from.
func Warn(format string, v ...any) {
	if currentLevel <= levelWarn {
		log.Printf("[WARN] "+format, v...)
	}
}

// Error logs an error condition.
func Error(format string, v ...any) {
	if currentLevel <= levelError {
		log.Printf("[ERROR] "+format, v...)
	}
}

// Fatal logs an error then exits with status 1.
func Fatal(format string, v ...any) {
	log.Printf("[ERROR] "+format, v...)
	os.Exit(1)
}
