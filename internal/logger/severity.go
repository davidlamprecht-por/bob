package logger

import "strings"

// Severity represents the severity level of a log message
type Severity int

const (
	// DEBUG is the lowest severity level for detailed debugging information
	DEBUG Severity = iota
	// INFO is for general informational messages
	INFO
	// WARN is for warning messages that indicate potential issues
	WARN
	// ERROR is for error messages that indicate failures
	ERROR
	// FATAL is the highest severity level for critical errors
	FATAL
)

// String returns the string representation of the severity level
func (s Severity) String() string {
	switch s {
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

// ParseSeverity converts a string to a Severity level
func ParseSeverity(s string) Severity {
	switch strings.ToUpper(s) {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARN", "WARNING":
		return WARN
	case "ERROR":
		return ERROR
	case "FATAL":
		return FATAL
	default:
		return INFO // default to INFO if unknown
	}
}
