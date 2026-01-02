.PHONY: help migrate migrate-status migrate-build test build run clean

# Default target
help:
	@echo "BOB - Available Commands"
	@echo "========================"
	@echo ""
	@echo "Database Migrations:"
	@echo "  make migrate         - Run all pending database migrations"
	@echo "  make migrate-status  - Show migration status"
	@echo "  make migrate-build   - Build the migrate binary"
	@echo ""
	@echo "Development:"
	@echo "  make build          - Build the main application"
	@echo "  make run            - Run the application"
	@echo "  make test           - Run tests"
	@echo "  make clean          - Clean build artifacts"
	@echo ""

# Database migration commands
migrate:
	@echo "Running database migrations..."
	@go run cmd/migrate/main.go run

migrate-status:
	@go run cmd/migrate/main.go status

migrate-build:
	@echo "Building migrate binary..."
	@mkdir -p bin
	@go build -o bin/migrate cmd/migrate/main.go
	@echo "✓ Built: bin/migrate"

# Application build commands
build:
	@echo "Building BOB application..."
	@mkdir -p bin
	@go build -o bin/bob cmd/bob/main.go
	@echo "✓ Built: bin/bob"

# Run the application
run:
	@go run cmd/bob/main.go

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@echo "✓ Cleaned"

# Initialize database (run first migration)
db-init: migrate

# Reset database (DEVELOPMENT ONLY - drops all tables)
db-reset:
	@echo "⚠️  WARNING: This will drop ALL tables!"
	@read -p "Are you sure? Type 'yes' to continue: " confirm; \
	if [ "$$confirm" = "yes" ]; then \
		echo "Resetting database..."; \
		mysql -h $(shell grep DB_HOST .env | cut -d '=' -f2) \
		      -P $(shell grep DB_PORT .env | cut -d '=' -f2) \
		      -u $(shell grep DB_USER .env | cut -d '=' -f2) \
		      -p$(shell grep DB_PASSWORD .env | cut -d '=' -f2) \
		      $(shell grep DB_NAME .env | cut -d '=' -f2) < scripts/reset_database.sql; \
		echo "✓ Database reset complete"; \
		echo "Now run: make migrate"; \
	else \
		echo "Cancelled."; \
	fi

# Show Go environment
env:
	@go env
