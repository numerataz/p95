package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps the pgx connection pool
type DB struct {
	Pool *pgxpool.Pool
}

// Config holds database configuration
type Config struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

// DefaultConfig returns default database configuration
func DefaultConfig(url string) Config {
	return Config{
		URL:             url,
		MaxConns:        25,
		MinConns:        5,
		MaxConnLifetime: time.Hour,
		MaxConnIdleTime: 30 * time.Minute,
	}
}

// New creates a new database connection pool
func New(ctx context.Context, cfg Config) (*DB, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	// Configure pool settings
	poolConfig.MaxConns = cfg.MaxConns
	poolConfig.MinConns = cfg.MinConns
	poolConfig.MaxConnLifetime = cfg.MaxConnLifetime
	poolConfig.MaxConnIdleTime = cfg.MaxConnIdleTime

	// Create the pool
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{Pool: pool}, nil
}

// Close closes the database connection pool
func (db *DB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
	}
}

// Health checks the database connection health
func (db *DB) Health(ctx context.Context) error {
	return db.Pool.Ping(ctx)
}

// BeginTx starts a new transaction
func (db *DB) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return db.Pool.Begin(ctx)
}

// Repositories holds all repository instances
type Repositories struct {
	Users         *UserRepository
	Teams         *TeamRepository
	Apps          *AppRepository
	Runs          *RunRepository
	Metrics       *MetricsRepository
	APIKeys       *APIKeyRepository
	Sessions      *SessionRepository
	Continuations *ContinuationRepository
}

// NewRepositories creates all repository instances
func NewRepositories(db *DB) *Repositories {
	return &Repositories{
		Users:         NewUserRepository(db),
		Teams:         NewTeamRepository(db),
		Apps:          NewAppRepository(db),
		Runs:          NewRunRepository(db),
		Metrics:       NewMetricsRepository(db),
		APIKeys:       NewAPIKeyRepository(db),
		Sessions:      NewSessionRepository(db),
		Continuations: NewContinuationRepository(db),
	}
}
