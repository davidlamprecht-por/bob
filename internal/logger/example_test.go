package logger_test

import (
	"bytes"
	"fmt"
	"strings"

	"bob/internal/logger"
)

// Example demonstrates basic usage of the logging module
func Example() {
	// Initialize with INFO threshold
	logger.Init(logger.INFO)

	// These will be logged
	logger.Info("Application started")
	logger.Warn("This is a warning")
	logger.Error("This is an error")

	// This won't be logged (below threshold)
	logger.Debug("This debug message won't appear")

	// Formatted messages
	logger.Infof("User %s logged in", "john")
	logger.Errorf("Failed to connect: %v", fmt.Errorf("connection refused"))
}

// Example_customLogger demonstrates creating and using a custom logger
func Example_customLogger() {
	var buf bytes.Buffer

	// Create a logger with DEBUG threshold that writes to buffer
	logger := logger.New(logger.DEBUG, &buf)

	// Log some messages
	logger.Debug("Debug message")
	logger.Info("Info message")
	logger.Warn("Warning message")

	// Check the output
	output := buf.String()
	fmt.Println(strings.Contains(output, "[DEBUG]"))
	fmt.Println(strings.Contains(output, "[INFO]"))
	fmt.Println(strings.Contains(output, "[WARN]"))
	// Output:
	// true
	// true
	// true
}

// Example_threshold demonstrates threshold filtering
func Example_threshold() {
	var buf bytes.Buffer

	// Create a logger with WARN threshold
	logger := logger.New(logger.WARN, &buf)

	// These won't be logged (below threshold)
	logger.Debug("Debug message")
	logger.Info("Info message")

	// These will be logged
	logger.Warn("Warning message")
	logger.Error("Error message")

	// Check the output
	output := buf.String()
	fmt.Println(strings.Contains(output, "[DEBUG]"))
	fmt.Println(strings.Contains(output, "[INFO]"))
	fmt.Println(strings.Contains(output, "[WARN]"))
	fmt.Println(strings.Contains(output, "[ERROR]"))
	// Output:
	// false
	// false
	// true
	// true
}
