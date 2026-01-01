package config

type Config struct {
	// Cache settings
	MaxCacheSize    int
	GraceBufferSize int

	// DB settings (for later)
	DBConnectionString string

	// Add more config as needed
}

// Current holds the active configuration
var Current Config

// Load loads configuration from environment or defaults
func Load() Config {
	// TODO: Load from .env file, env vars, etc.
	return Config{
		MaxCacheSize:    10000,
		GraceBufferSize: 11000,
	}
}

// Init initializes the configuration
func Init() {
	Current = Load()
}
