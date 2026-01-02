package main

import (
	"bob/internal/config"
	"bob/internal/database"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	// Load configuration
	config.Init()

	// Parse command line arguments
	command := "run" // default command
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	// Connect to database
	fmt.Println("Connecting to database...")
	connStr := config.Current.DBConnectionString()
	if err := database.Connect(connStr); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	fmt.Println("✓ Connected to database successfully\n")

	// Get migrations directory path
	migrationsDir := getMigrationsDir()

	// Create migration runner
	runner := database.NewMigrationRunner(database.DB, migrationsDir)

	// Execute command
	var err error
	switch command {
	case "run", "up":
		err = runner.Run()
	case "status":
		err = runner.Status()
	case "help", "-h", "--help":
		printHelp()
		return
	default:
		fmt.Fprintf(os.Stderr, "Error: Unknown command '%s'\n\n", command)
		printHelp()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
		os.Exit(1)
	}
}

// getMigrationsDir returns the path to the migrations directory
func getMigrationsDir() string {
	// Try to find migrations directory relative to current working directory
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not get working directory: %v\n", err)
		return "definitions/migrations"
	}

	// Check common locations
	possiblePaths := []string{
		filepath.Join(cwd, "definitions", "migrations"),
		filepath.Join(cwd, "..", "..", "definitions", "migrations"),
		"definitions/migrations",
	}

	for _, path := range possiblePaths {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			absPath, _ := filepath.Abs(path)
			fmt.Printf("Using migrations directory: %s\n\n", absPath)
			return path
		}
	}

	// Default fallback
	fmt.Println("Using default migrations directory: definitions/migrations\n")
	return "definitions/migrations"
}

func printHelp() {
	fmt.Println("BOB Database Migration Tool")
	fmt.Println("===========================")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  migrate [command]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  run, up    Run all pending migrations (default)")
	fmt.Println("  status     Show migration status")
	fmt.Println("  help       Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  migrate              # Run all pending migrations")
	fmt.Println("  migrate run          # Run all pending migrations")
	fmt.Println("  migrate status       # Show which migrations have been applied")
	fmt.Println()
	fmt.Println("Configuration:")
	fmt.Println("  Database connection settings are read from environment variables or .env file:")
	fmt.Println("    DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME")
	fmt.Println()
}
