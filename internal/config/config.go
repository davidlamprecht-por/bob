package config

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all application configuration
type Config struct {
	// Database Configuration
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string

	// Slack Configuration
	SlackBotToken     string
	SlackAppToken     string
	SlackSigningSecret string

	// OpenAI Configuration
	OpenAIAPIKey string

	// Azure DevOps Configuration
	ADOOrgURL string
	ADOProject string
	ADOPAT    string

	// Logging Configuration
	LogLevel string
	LogFile  string

	// Session Cache Configuration
	MaxCacheSize    int
	GraceBufferSize int
	CacheTTLSeconds int
}

// Current holds the active configuration
var Current Config

// Init initializes the configuration by loading from .env file
func Init() {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables only")
	}

	var err error
	Current, err = Load()
	if err != nil {
		log.Fatalf("Configuration Error: %v\n\nHint: Copy .env.dist to .env and fill in your values.", err)
	}
}

// Load loads configuration from environment variables
func Load() (Config, error) {
	cfg := Config{}

	// Helper to get required env var
	getRequired := func(key string) (string, error) {
		value := os.Getenv(key)
		if value == "" {
			return "", fmt.Errorf("missing required environment variable: %s\nPlease set %s in your .env file or environment", key, key)
		}
		return value, nil
	}

	// Helper to get optional env var with default
	getOptional := func(key, defaultValue string) string {
		value := os.Getenv(key)
		if value == "" {
			return defaultValue
		}
		return value
	}

	// Helper to get optional int with default
	getOptionalInt := func(key string, defaultValue int) (int, error) {
		value := os.Getenv(key)
		if value == "" {
			return defaultValue, nil
		}
		intValue, err := strconv.Atoi(value)
		if err != nil {
			return 0, fmt.Errorf("invalid integer value for %s: %s", key, value)
		}
		return intValue, nil
	}

	var err error

	// Database Configuration
	if cfg.DBHost, err = getRequired("DB_HOST"); err != nil {
		return cfg, err
	}
	if cfg.DBPort, err = getOptionalInt("DB_PORT", 3306); err != nil {
		return cfg, err
	}
	if cfg.DBUser, err = getRequired("DB_USER"); err != nil {
		return cfg, err
	}
	if cfg.DBPassword, err = getRequired("DB_PASSWORD"); err != nil {
		return cfg, err
	}
	if cfg.DBName, err = getRequired("DB_NAME"); err != nil {
		return cfg, err
	}

	// Slack Configuration
	if cfg.SlackBotToken, err = getRequired("SLACK_BOT_TOKEN"); err != nil {
		return cfg, err
	}
	if cfg.SlackAppToken, err = getRequired("SLACK_APP_TOKEN"); err != nil {
		return cfg, err
	}
	if cfg.SlackSigningSecret, err = getRequired("SLACK_SIGNING_SECRET"); err != nil {
		return cfg, err
	}

	// OpenAI Configuration
	if cfg.OpenAIAPIKey, err = getRequired("OPENAI_API_KEY"); err != nil {
		return cfg, err
	}

	// Azure DevOps Configuration
	if cfg.ADOOrgURL, err = getRequired("ADO_ORG_URL"); err != nil {
		return cfg, err
	}
	if cfg.ADOProject, err = getRequired("ADO_PROJECT"); err != nil {
		return cfg, err
	}
	if cfg.ADOPAT, err = getRequired("ADO_PAT"); err != nil {
		return cfg, err
	}

	// Logging Configuration (optional with defaults)
	cfg.LogLevel = getOptional("LOG_LEVEL", "INFO")
	cfg.LogFile = getOptional("LOG_FILE", "logs/bob.log")

	// Session Cache Configuration (optional with defaults)
	if cfg.MaxCacheSize, err = getOptionalInt("SESSION_CACHE_MAX_SIZE", 10000); err != nil {
		return cfg, err
	}
	// Grace buffer is 10% above max cache size by default
	if cfg.GraceBufferSize, err = getOptionalInt("SESSION_CACHE_GRACE_BUFFER", cfg.MaxCacheSize+1000); err != nil {
		return cfg, err
	}
	if cfg.CacheTTLSeconds, err = getOptionalInt("SESSION_CACHE_TTL_SECONDS", 28800); err != nil {
		return cfg, err
	}

	return cfg, nil
}

// DBConnectionString returns the formatted database connection string
func (c *Config) DBConnectionString() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local&multiStatements=true",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName)
}
