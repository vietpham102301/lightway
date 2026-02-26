package sql

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresConfig holds the configuration for a PostgreSQL connection pool.
// Zero values for optional fields will use sensible defaults.
type PostgresConfig struct {
	MaxConn           int32 // default: 10
	ConnMaxLifetime   int   // in minutes, default: 60
	ConnMaxIdleTime   int   // in minutes, default: ConnMaxLifetime/2
	HealthCheckPeriod int   // in minutes, default: 1
}

func (c *PostgresConfig) applyDefaults() {
	if c.MaxConn <= 0 {
		c.MaxConn = 10
	}
	if c.ConnMaxLifetime <= 0 {
		c.ConnMaxLifetime = 60
	}
	if c.ConnMaxIdleTime <= 0 {
		c.ConnMaxIdleTime = c.ConnMaxLifetime / 2
	}
	if c.HealthCheckPeriod <= 0 {
		c.HealthCheckPeriod = 1
	}
}

func NewPostgresDB(connString string, cfg PostgresConfig) (*pgxpool.Pool, error) {
	cfg.applyDefaults()

	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("unable to parse config: %w", err)
	}

	config.MaxConns = cfg.MaxConn
	config.MaxConnLifetime = time.Duration(cfg.ConnMaxLifetime) * time.Minute
	config.MaxConnIdleTime = time.Duration(cfg.ConnMaxIdleTime) * time.Minute
	config.HealthCheckPeriod = time.Duration(cfg.HealthCheckPeriod) * time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	return pool, nil
}
