package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// DeploymentMode represents the deployment mode
type DeploymentMode string

const (
	DeploymentModeSelfHosted DeploymentMode = "self-hosted"
	DeploymentModeCloud      DeploymentMode = "cloud"
)

// Config holds all configuration for the application
type Config struct {
	// Server
	Port           string
	Host           string
	DeploymentMode DeploymentMode
	Debug          bool

	// Database
	DatabaseURL string
	RedisURL    string

	// Authentication
	JWTSecret            string
	JWTAccessExpiration  time.Duration
	JWTRefreshExpiration time.Duration
	SessionDuration      time.Duration

	// Self-hosted specific
	AdminEmail    string
	AdminPassword string

	// Cloud specific
	EnableBilling bool
	StripeKey     string

	// Features
	EnableMultiTenancy bool
	MaxRunsPerApp      int // 0 = unlimited
	MaxMetricsPerRun   int // 0 = unlimited
	RetentionDays      int // 0 = unlimited

	// Metrics
	MetricsBatchSize    int
	MetricsFlushInterval time.Duration

	// WebSocket
	WSReadBufferSize  int
	WSWriteBufferSize int
	WSPingInterval    time.Duration
	WSPongWait        time.Duration
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	mode := DeploymentMode(getEnv("DEPLOYMENT_MODE", "self-hosted"))

	cfg := &Config{
		// Server
		Port:           getEnv("PORT", "8080"),
		Host:           getEnv("HOST", "0.0.0.0"),
		DeploymentMode: mode,
		Debug:          getEnvBool("DEBUG", false),

		// Database
		DatabaseURL: getEnv("DATABASE_URL", "postgres://sixtyseven:sixtyseven@localhost:5432/sixtyseven?sslmode=disable"),
		RedisURL:    getEnv("REDIS_URL", ""),

		// Authentication
		JWTSecret:            getEnv("JWT_SECRET", ""),
		JWTAccessExpiration:  getEnvDuration("JWT_ACCESS_EXPIRATION", 15*time.Minute),
		JWTRefreshExpiration: getEnvDuration("JWT_REFRESH_EXPIRATION", 7*24*time.Hour),
		SessionDuration:      getEnvDuration("SESSION_DURATION", 24*time.Hour),

		// Metrics
		MetricsBatchSize:     getEnvInt("METRICS_BATCH_SIZE", 5000),
		MetricsFlushInterval: getEnvDuration("METRICS_FLUSH_INTERVAL", 1*time.Second),

		// WebSocket
		WSReadBufferSize:  getEnvInt("WS_READ_BUFFER_SIZE", 1024),
		WSWriteBufferSize: getEnvInt("WS_WRITE_BUFFER_SIZE", 1024),
		WSPingInterval:    getEnvDuration("WS_PING_INTERVAL", 30*time.Second),
		WSPongWait:        getEnvDuration("WS_PONG_WAIT", 60*time.Second),
	}

	// Mode-specific configuration
	if mode == DeploymentModeSelfHosted {
		cfg.AdminEmail = getEnv("ADMIN_EMAIL", "admin@localhost")
		cfg.AdminPassword = getEnv("ADMIN_PASSWORD", "")
		cfg.EnableMultiTenancy = false
		cfg.MaxRunsPerApp = 0      // Unlimited
		cfg.MaxMetricsPerRun = 0   // Unlimited
		cfg.RetentionDays = 0      // Unlimited
	} else {
		cfg.EnableMultiTenancy = true
		cfg.EnableBilling = getEnvBool("ENABLE_BILLING", true)
		cfg.StripeKey = getEnv("STRIPE_KEY", "")
		cfg.MaxRunsPerApp = getEnvInt("MAX_RUNS_PER_APP", 100)
		cfg.MaxMetricsPerRun = getEnvInt("MAX_METRICS_PER_RUN", 1000000)
		cfg.RetentionDays = getEnvInt("RETENTION_DAYS", 90)
	}

	// Validate required configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.JWTSecret == "" {
		// Generate a warning but don't fail in dev mode
		if c.Debug {
			c.JWTSecret = "dev-secret-change-in-production"
		} else {
			return fmt.Errorf("JWT_SECRET is required")
		}
	}

	if c.DeploymentMode == DeploymentModeSelfHosted && c.AdminPassword == "" && !c.Debug {
		return fmt.Errorf("ADMIN_PASSWORD is required for self-hosted mode")
	}

	return nil
}

// IsSelfHosted returns true if running in self-hosted mode
func (c *Config) IsSelfHosted() bool {
	return c.DeploymentMode == DeploymentModeSelfHosted
}

// IsCloud returns true if running in cloud mode
func (c *Config) IsCloud() bool {
	return c.DeploymentMode == DeploymentModeCloud
}

// Address returns the full server address
func (c *Config) Address() string {
	return fmt.Sprintf("%s:%s", c.Host, c.Port)
}

// Helper functions for environment variables

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		b, err := strconv.ParseBool(value)
		if err == nil {
			return b
		}
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		i, err := strconv.Atoi(value)
		if err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		d, err := time.ParseDuration(value)
		if err == nil {
			return d
		}
	}
	return defaultValue
}
