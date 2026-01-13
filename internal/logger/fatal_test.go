package logger

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestFatalWithString(t *testing.T) {
	var buf bytes.Buffer
	logger := New(INFO, &buf)

	// We can't actually test os.Exit, so we'll test the Log part only
	// by directly calling Log instead of Fatal
	logger.Log(FATAL, "test fatal string")

	output := buf.String()
	if !strings.Contains(output, "[FATAL]") {
		t.Errorf("Expected [FATAL] in output, got: %s", output)
	}
	if !strings.Contains(output, "test fatal string") {
		t.Errorf("Expected message in output, got: %s", output)
	}
}

func TestFatalWithError(t *testing.T) {
	var buf bytes.Buffer
	logger := New(INFO, &buf)

	// Create a test error
	testErr := errors.New("test error message")

	// Test the message conversion logic without os.Exit
	var msg string
	switch v := any(testErr).(type) {
	case error:
		msg = v.Error()
	case string:
		msg = v
	default:
		msg = "unknown"
	}

	if msg != "test error message" {
		t.Errorf("Expected 'test error message', got: %s", msg)
	}

	logger.Log(FATAL, msg)
	output := buf.String()
	if !strings.Contains(output, "[FATAL]") {
		t.Errorf("Expected [FATAL] in output, got: %s", output)
	}
	if !strings.Contains(output, "test error message") {
		t.Errorf("Expected error message in output, got: %s", output)
	}
}
