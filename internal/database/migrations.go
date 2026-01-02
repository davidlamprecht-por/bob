package database

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Migration represents a database migration
type Migration struct {
	Name         string
	FilePath     string
	Checksum     string
	SQL          string
	ExecutedAt   time.Time
	IsExecuted   bool
	ExecutionTime int
}

// MigrationRunner handles database migrations
type MigrationRunner struct {
	db              *sql.DB
	migrationsDir   string
	migrations      []Migration
	executedMigrations map[string]Migration
}

// NewMigrationRunner creates a new migration runner
func NewMigrationRunner(db *sql.DB, migrationsDir string) *MigrationRunner {
	return &MigrationRunner{
		db:              db,
		migrationsDir:   migrationsDir,
		migrations:      []Migration{},
		executedMigrations: make(map[string]Migration),
	}
}

// LoadMigrations loads all migration files from the migrations directory
func (mr *MigrationRunner) LoadMigrations() error {
	files, err := filepath.Glob(filepath.Join(mr.migrationsDir, "m*.sql"))
	if err != nil {
		return fmt.Errorf("failed to list migration files: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no migration files found in %s", mr.migrationsDir)
	}

	// Sort files alphabetically to ensure correct order
	sort.Strings(files)

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", file, err)
		}

		migrationName := filepath.Base(file)
		migrationName = strings.TrimSuffix(migrationName, ".sql")

		checksum := fmt.Sprintf("%x", sha256.Sum256(content))

		mr.migrations = append(mr.migrations, Migration{
			Name:       migrationName,
			FilePath:   file,
			SQL:        string(content),
			Checksum:   checksum,
			IsExecuted: false,
		})
	}

	return nil
}

// LoadExecutedMigrations loads the list of already executed migrations from the database
func (mr *MigrationRunner) LoadExecutedMigrations() error {
	// Check if migrations table exists, if not, we'll create it in the first migration
	var tableExists int
	err := mr.db.QueryRow(`
		SELECT COUNT(*)
		FROM information_schema.tables
		WHERE table_schema = DATABASE()
		AND table_name = 'migrations'
	`).Scan(&tableExists)

	if err != nil {
		return fmt.Errorf("failed to check migrations table: %w", err)
	}

	if tableExists == 0 {
		// Migrations table doesn't exist yet, this is fine for first run
		return nil
	}

	rows, err := mr.db.Query(`
		SELECT migration_name, applied_at, checksum, execution_time_ms
		FROM migrations
		ORDER BY applied_at ASC
	`)
	if err != nil {
		return fmt.Errorf("failed to query executed migrations: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var migration Migration
		var executionTimeMs sql.NullInt64
		var checksum sql.NullString

		err := rows.Scan(
			&migration.Name,
			&migration.ExecutedAt,
			&checksum,
			&executionTimeMs,
		)
		if err != nil {
			return fmt.Errorf("failed to scan migration row: %w", err)
		}

		if checksum.Valid {
			migration.Checksum = checksum.String
		}
		if executionTimeMs.Valid {
			migration.ExecutionTime = int(executionTimeMs.Int64)
		}
		migration.IsExecuted = true

		mr.executedMigrations[migration.Name] = migration
	}

	return rows.Err()
}

// GetPendingMigrations returns migrations that haven't been executed yet
func (mr *MigrationRunner) GetPendingMigrations() []Migration {
	var pending []Migration

	for _, migration := range mr.migrations {
		if _, executed := mr.executedMigrations[migration.Name]; !executed {
			pending = append(pending, migration)
		}
	}

	return pending
}

// ValidateMigrations checks if executed migrations match their current files
func (mr *MigrationRunner) ValidateMigrations() error {
	for name, executedMigration := range mr.executedMigrations {
		// Find the migration in our loaded migrations
		var foundMigration *Migration
		for i := range mr.migrations {
			if mr.migrations[i].Name == name {
				foundMigration = &mr.migrations[i]
				break
			}
		}

		if foundMigration == nil {
			return fmt.Errorf("executed migration %s not found in migration files", name)
		}

		// Check if checksum matches (if we have a stored checksum)
		if executedMigration.Checksum != "" && foundMigration.Checksum != executedMigration.Checksum {
			return fmt.Errorf("migration %s has been modified (checksum mismatch)", name)
		}
	}

	return nil
}

// RunMigration executes a single migration
func (mr *MigrationRunner) RunMigration(migration Migration) error {
	startTime := time.Now()

	// Execute the migration SQL
	_, err := mr.db.Exec(migration.SQL)
	if err != nil {
		return fmt.Errorf("failed to execute migration %s: %w", migration.Name, err)
	}

	executionTime := time.Since(startTime).Milliseconds()

	// The migration itself should insert its own record, but we'll verify it exists
	// by checking if the migrations table now exists and has this entry
	var count int
	err = mr.db.QueryRow("SELECT COUNT(*) FROM migrations WHERE migration_name = ?", migration.Name).Scan(&count)
	if err != nil {
		// If the query fails, it might be because this IS the initial migration creating the table
		// In that case, we should verify the table exists now
		var tableExists int
		checkErr := mr.db.QueryRow(`
			SELECT COUNT(*)
			FROM information_schema.tables
			WHERE table_schema = DATABASE()
			AND table_name = 'migrations'
		`).Scan(&tableExists)

		if checkErr != nil || tableExists == 0 {
			return fmt.Errorf("migrations table was not created by migration %s", migration.Name)
		}

		// Table exists but record isn't there, this might be the initial migration
		// The initial migration should have inserted itself, but let's check again
		err = mr.db.QueryRow("SELECT COUNT(*) FROM migrations WHERE migration_name = ?", migration.Name).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to verify migration record for %s: %w", migration.Name, err)
		}
	}

	if count == 0 {
		// Migration didn't insert itself, we need to insert it manually
		_, err = mr.db.Exec(`
			INSERT INTO migrations (migration_name, checksum, execution_time_ms)
			VALUES (?, ?, ?)
		`, migration.Name, migration.Checksum, executionTime)
		if err != nil {
			return fmt.Errorf("failed to record migration %s: %w", migration.Name, err)
		}
	}

	fmt.Printf("✓ Executed migration %s (took %dms)\n", migration.Name, executionTime)
	return nil
}

// RunPendingMigrations executes all pending migrations in order
func (mr *MigrationRunner) RunPendingMigrations() error {
	pending := mr.GetPendingMigrations()

	if len(pending) == 0 {
		fmt.Println("No pending migrations to run")
		return nil
	}

	fmt.Printf("Found %d pending migration(s)\n", len(pending))

	for _, migration := range pending {
		fmt.Printf("Running migration: %s\n", migration.Name)
		if err := mr.RunMigration(migration); err != nil {
			return err
		}
	}

	fmt.Println("\n✓ All migrations completed successfully")
	return nil
}

// Run is the main entry point for running migrations
func (mr *MigrationRunner) Run() error {
	fmt.Println("Loading migration files...")
	if err := mr.LoadMigrations(); err != nil {
		return err
	}
	fmt.Printf("Found %d migration file(s)\n", len(mr.migrations))

	fmt.Println("\nLoading executed migrations from database...")
	if err := mr.LoadExecutedMigrations(); err != nil {
		return err
	}
	fmt.Printf("Found %d executed migration(s)\n", len(mr.executedMigrations))

	fmt.Println("\nValidating migrations...")
	if err := mr.ValidateMigrations(); err != nil {
		return err
	}
	fmt.Println("✓ Migration validation passed")

	fmt.Println("\nRunning pending migrations...")
	return mr.RunPendingMigrations()
}

// Status displays the current migration status
func (mr *MigrationRunner) Status() error {
	if err := mr.LoadMigrations(); err != nil {
		return err
	}

	if err := mr.LoadExecutedMigrations(); err != nil {
		return err
	}

	fmt.Println("Migration Status:")
	fmt.Println("================")

	if len(mr.migrations) == 0 {
		fmt.Println("No migrations found")
		return nil
	}

	for _, migration := range mr.migrations {
		if executed, ok := mr.executedMigrations[migration.Name]; ok {
			fmt.Printf("✓ %s (executed: %s)\n", migration.Name, executed.ExecutedAt.Format("2006-01-02 15:04:05"))
		} else {
			fmt.Printf("○ %s (pending)\n", migration.Name)
		}
	}

	pending := mr.GetPendingMigrations()
	fmt.Printf("\nTotal: %d | Executed: %d | Pending: %d\n",
		len(mr.migrations),
		len(mr.executedMigrations),
		len(pending))

	return nil
}
