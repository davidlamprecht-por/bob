# Logger Module

A lightweight logger module for Bob that provides severity-based logging with configurable thresholds.

## Features

- **Five severity levels**: DEBUG, INFO, WARN, ERROR, FATAL
- **Threshold filtering**: Only log messages at or above a configured severity level
- **Flexible API**: Package-level functions for simple usage, or create custom logger instances
- **Thread-safe**: Safe for concurrent use
- **Fatal logging**: Automatically exits with severity FATAL (configurable)
- **Formatted logging**: Support for `Printf`-style formatted messages
- **Console output**: Currently logs to stdout (file logging planned for future)

## Severity Levels

The module supports five severity levels, from lowest to highest:

1. **DEBUG** - Detailed debugging information
2. **INFO** - General informational messages
3. **WARN** - Warning messages for potential issues
4. **ERROR** - Error messages for failures
5. **FATAL** - Critical errors (causes program exit)

## Usage

### Basic Setup

Initialize the logger with a severity threshold at application startup:

```go
import "bob/internal/logger"

func main() {
    // Initialize with INFO threshold - only INFO, WARN, ERROR, FATAL will be logged
    logger.Init(logger.INFO)

    // Or initialize with a string
    logger.InitWithString("INFO")

    // Your application code...
}
```

### Logging Messages

#### Using convenience functions (recommended for most cases)

```go
// Simple messages
logger.Debug("Detailed debug information")
logger.Info("Application started successfully")
logger.Warn("Configuration value not set, using default")
logger.Error("Failed to connect to database")

// Formatted messages
logger.Debugf("Processing item %d of %d", i, total)
logger.Infof("User %s logged in", username)
logger.Warnf("Connection retry attempt %d", retryCount)
logger.Errorf("Database error: %v", err)
```

#### Using explicit severity

```go
logger.Log(logger.INFO, "This is an info message")
logger.Logf(logger.ERROR, "Error occurred: %v", err)
```

#### Fatal logging

```go
// Fatal with default FATAL severity (exits program)
logger.Fatal("Critical error: cannot continue")

// Fatal with formatted message
logger.Fatalf("Critical error: %v", err)

// Fatal with custom severity (still exits, but logs at different level)
logger.Fatal("Custom fatal message", logger.ERROR)
```

**Important distinction:**
- `logger.Log(logger.FATAL, "message")` - Logs with [FATAL] label but **continues execution**
- `logger.Fatal("message")` - Logs with [FATAL] label and **exits the program** with `os.Exit(1)`

Use `Log(FATAL, ...)` when you want to log something as critical but handle the exit yourself. Use `Fatal(...)` when you want automatic program termination.

### Threshold Configuration

The threshold determines which messages are logged. Messages below the threshold are silently discarded.

```go
// Set threshold to WARN - only WARN, ERROR, FATAL are logged
logger.Init(logger.WARN)

// Change threshold at runtime
logger.SetThreshold(logger.DEBUG) // Now all messages are logged
```

### Custom Logger Instances

For more control, create custom logger instances:

```go
import (
    "os"
    "bob/internal/logger"
)

// Create a logger with DEBUG threshold
debugLogger := logger.New(logger.DEBUG, os.Stdout)

// Use the custom logger
debugLogger.Log(logger.INFO, "Using custom logger")
debugLogger.Logf(logger.DEBUG, "Debug value: %d", value)

// Change threshold on custom logger
debugLogger.SetThreshold(logger.ERROR)
```

### Integration with Config

The logging module can be integrated with the application config:

```go
import (
    "bob/internal/config"
    "bob/internal/logger"
)

func main() {
    config.Init()

    // Initialize logger from config
    logger.InitWithString(config.Current.LogLevel)

    logger.Info("Application started with configured log level")
}
```

## Examples

### Example 1: Basic Application Logging

```go
package main

import "bob/internal/logger"

func main() {
    logger.Init(logger.INFO)

    logger.Info("Application starting...")

    if err := doSomething(); err != nil {
        logger.Errorf("Operation failed: %v", err)
    }

    logger.Info("Application completed successfully")
}
```

### Example 2: Debug Mode

```go
package main

import (
    "flag"
    "bob/internal/logger"
)

func main() {
    debug := flag.Bool("debug", false, "Enable debug logging")
    flag.Parse()

    if *debug {
        logger.Init(logger.DEBUG)
        logger.Debug("Debug mode enabled")
    } else {
        logger.Init(logger.INFO)
    }

    logger.Info("Application started")
    logger.Debug("This will only appear in debug mode")
}
```

### Example 3: Production vs Development

```go
package main

import (
    "os"
    "bob/internal/logger"
)

func main() {
    env := os.Getenv("ENVIRONMENT")

    if env == "production" {
        logger.Init(logger.WARN) // Only warnings and errors in production
    } else {
        logger.Init(logger.DEBUG) // Everything in development
    }

    logger.Debug("Starting application in development mode")
    logger.Info("Loading configuration")
    logger.Warn("Using default timeout value")
}
```

## Future Enhancements

- File output support (in addition to console)
- Log rotation
- Multiple output destinations
- Structured logging (JSON format)
- Custom formatters

## Thread Safety

All logging operations are thread-safe and can be used safely from multiple goroutines.
