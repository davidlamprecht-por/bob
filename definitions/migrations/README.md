# Database Migrations

This directory contains all database migrations for the BOB project.

## Migration Files

Migrations are SQL files that follow the naming convention: `mXXXX_description.sql`

Example:
- `m0001_schema.sql` - Initial database schema
- `m0002_add_user_settings.sql` - Add user settings table

**Important**: Migration files are executed in alphabetical order, so use sequential numbering.

## Running Migrations

### Using the migrate command

```bash
# Run all pending migrations
go run cmd/migrate/main.go

# Or build and run
go build -o bin/migrate cmd/migrate/main.go
./bin/migrate

# Check migration status
go run cmd/migrate/main.go status
```

### Commands

- `migrate` or `migrate run` - Execute all pending migrations
- `migrate status` - Show which migrations have been executed and which are pending
- `migrate help` - Show help information

## How It Works

1. The migration runner scans the `definitions/migrations/` directory for `m*.sql` files
2. It loads the list of executed migrations from the `migrations` table in the database
3. It validates that executed migrations haven't been modified (checksum validation)
4. It runs any pending migrations in alphabetical order
5. Each migration is tracked in the `migrations` table with:
   - Migration name
   - Execution timestamp
   - SHA256 checksum
   - Execution time in milliseconds

## Creating New Migrations

1. Create a new file with the next sequential number: `mXXXX_description.sql`
2. Write your SQL statements in the file
3. Ensure your migration is idempotent (use `IF NOT EXISTS`, `IF EXISTS`, etc.)
4. Run `go run cmd/migrate/main.go` to apply the migration

### Migration Best Practices

1. **Use Sequential Numbering**: Always use the next number in sequence
2. **Be Idempotent**: Migrations should be safe to run multiple times
3. **No Manual Edits**: Never modify a migration file after it's been executed
4. **Test Locally First**: Always test migrations on a local database first
5. **Include Comments**: Document what the migration does and why

### Example Migration Structure

```sql
-- ============================================
-- Migration: m0002_add_user_settings
-- Description: Add user settings table
-- ============================================

CREATE TABLE IF NOT EXISTS user_settings (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT NOT NULL,
    setting_key VARCHAR(100) NOT NULL,
    setting_value TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (user_id) REFERENCES user_external_ids(id) ON DELETE CASCADE,
    UNIQUE INDEX idx_user_setting (user_id, setting_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Record this migration
INSERT INTO migrations (migration_name, checksum)
VALUES ('m0002_add_user_settings', SHA2('m0002_add_user_settings', 256))
ON DUPLICATE KEY UPDATE applied_at = applied_at;
```

## Configuration

The migration tool uses the same database configuration as the main application:

- `DB_HOST` - Database host (default: localhost)
- `DB_PORT` - Database port (default: 3306)
- `DB_USER` - Database username
- `DB_PASSWORD` - Database password
- `DB_NAME` - Database name

These can be set in your `.env` file or as environment variables.

## Troubleshooting

### "Migration has been modified (checksum mismatch)"

This means a migration file that has already been executed has been changed. This is not allowed because it could lead to inconsistencies between different environments.

**Solution**: Create a new migration file to make the additional changes.

### "Migrations table doesn't exist"

This is normal on first run. The first migration (`m0001_schema.sql`) creates the migrations table.

### "No migration files found"

Ensure you're running the command from the project root directory, or that the `definitions/migrations/` directory exists and contains migration files.

## Database Schema Evolution

When you need to make schema changes:

1. Create a new migration file with the next sequential number
2. Include both the schema changes and any necessary data migrations
3. Test thoroughly in development
4. Run migrations in staging before production
5. Keep migrations small and focused on one logical change

## Safety Features

- **Checksum Validation**: Ensures executed migrations haven't been modified
- **Alphabetical Ordering**: Guarantees consistent execution order
- **Transaction Support**: Each migration runs in the context of the database's transaction support
- **Execution Tracking**: Records when each migration was executed and how long it took
- **Idempotent Checks**: Migrations use `IF NOT EXISTS` to be safely re-runnable
