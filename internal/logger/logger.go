// Package logger provides a lightweight, severity-based logging system with configurable thresholds.
//
// The package supports five severity levels (DEBUG, INFO, WARN, ERROR, FATAL) and allows
// filtering messages by severity threshold. Messages below the configured threshold are
// silently discarded.
//
// Basic usage:
//
//	logger.Init(logger.INFO)
//	logger.Info("Application started")
//	logger.Errorf("Failed to connect: %v", err)
//	logger.Fatal("Critical error") // Logs and exits
//	logger.Fatal(err)               // Also accepts errors
//
// The package is thread-safe and currently logs to console (stdout), with file output
// planned for future releases.
package logger

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Logger represents a logger instance
type Logger struct {
	threshold Severity
	output    io.Writer
	mu        sync.Mutex
}

// defaultLogger is the package-level logger instance
var (
	defaultLogger *Logger
	once          sync.Once
)

// Init initializes the default logger with a severity threshold
// Only messages with severity >= threshold will be logged
func Init(threshold Severity) {
	once.Do(func() {
		defaultLogger = &Logger{
			threshold: threshold,
			output:    os.Stdout,
		}
	})
}

// InitWithString initializes the default logger with a string severity level
func InitWithString(level string) {
	Init(ParseSeverity(level))
}

// SetThreshold updates the severity threshold for the default logger
func SetThreshold(threshold Severity) {
	getDefaultLogger().SetThreshold(threshold)
}

// SetOutput sets the output writer for the default logger
func SetOutput(w io.Writer) {
	getDefaultLogger().SetOutput(w)
}

// getDefaultLogger returns the default logger, initializing it if needed
func getDefaultLogger() *Logger {
	once.Do(func() {
		defaultLogger = &Logger{
			threshold: INFO, // default threshold
			output:    os.Stdout,
		}
	})
	return defaultLogger
}

// New creates a new Logger instance with the specified threshold
func New(threshold Severity, output io.Writer) *Logger {
	return &Logger{
		threshold: threshold,
		output:    output,
	}
}

// SetThreshold updates the severity threshold
func (l *Logger) SetThreshold(threshold Severity) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.threshold = threshold
}

// SetOutput sets the output writer
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.output = w
}

// Log logs a message with the specified severity
func (l *Logger) Log(severity Severity, message string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if severity < l.threshold {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(l.output, "[%s] [%s] %s\n", timestamp, severity.String(), message)
}

// Logf logs a formatted message with the specified severity
func (l *Logger) Logf(severity Severity, format string, args ...any) {
	l.Log(severity, fmt.Sprintf(format, args...))
}

// Fatal logs a message with FATAL severity and exits the program
// Accepts either a string or error type
// The severity can be overridden if needed
func (l *Logger) Fatal(message any, severity ...Severity) {
	sev := FATAL
	if len(severity) > 0 {
		sev = severity[0]
	}

	var msg string
	switch v := message.(type) {
	case error:
		msg = v.Error()
	case string:
		msg = v
	default:
		msg = fmt.Sprintf("%v", v)
	}

	l.Log(sev, msg)
	os.Exit(1)
}

// Fatalf logs a formatted message with FATAL severity and exits the program
func (l *Logger) Fatalf(format string, args ...any) {
	l.Fatal(fmt.Sprintf(format, args...))
}

// Convenience methods for Logger instance

// Debug logs a debug message
func (l *Logger) Debug(message string) {
	l.Log(DEBUG, message)
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(format string, args ...any) {
	l.Logf(DEBUG, format, args...)
}

// Info logs an info message
func (l *Logger) Info(message string) {
	l.Log(INFO, message)
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, args ...any) {
	l.Logf(INFO, format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(message string) {
	l.Log(WARN, message)
}

// Warnf logs a formatted warning message
func (l *Logger) Warnf(format string, args ...any) {
	l.Logf(WARN, format, args...)
}

// Error logs an error message
func (l *Logger) Error(message string) {
	l.Log(ERROR, message)
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, args ...any) {
	l.Logf(ERROR, format, args...)
}

// Package-level convenience functions that use the default logger

// Log logs a message using the default logger
func Log(severity Severity, message string) {
	getDefaultLogger().Log(severity, message)
}

// Logf logs a formatted message using the default logger
func Logf(severity Severity, format string, args ...any) {
	getDefaultLogger().Logf(severity, format, args...)
}

// Fatal logs a fatal message using the default logger and exits
// Accepts either a string or error type
func Fatal(message any, severity ...Severity) {
	getDefaultLogger().Fatal(message, severity...)
}

// Fatalf logs a formatted fatal message using the default logger and exits
func Fatalf(format string, args ...any) {
	getDefaultLogger().Fatalf(format, args...)
}

// Convenience methods for specific severity levels

// Debug logs a debug message
func Debug(message string) {
	Log(DEBUG, message)
}

// Debugf logs a formatted debug message
func Debugf(format string, args ...any) {
	Logf(DEBUG, format, args...)
}

// Info logs an info message
func Info(message string) {
	Log(INFO, message)
}

// Infof logs a formatted info message
func Infof(format string, args ...any) {
	Logf(INFO, format, args...)
}

// Warn logs a warning message
func Warn(message string) {
	Log(WARN, message)
}

// Warnf logs a formatted warning message
func Warnf(format string, args ...any) {
	Logf(WARN, format, args...)
}

// Error logs an error message
func Error(message string) {
	Log(ERROR, message)
}

// Errorf logs a formatted error message
func Errorf(format string, args ...any) {
	Logf(ERROR, format, args...)
}
