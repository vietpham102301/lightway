package sql

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	cfg "github.com/vietpham102301/hermes/config"
)

func NewPostgresDB(connString string, postgresConf cfg.PostgresSQLConfig) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("unable to parse config: %w", err)
	}

	config.MaxConns = postgresConf.MaxConn
	config.MaxConnLifetime = time.Duration(postgresConf.ConnMaxLifetime) * time.Minute
	config.MaxConnIdleTime = time.Duration(postgresConf.ConnMaxLifetime/2) * time.Minute
	config.HealthCheckPeriod = 1 * time.Minute

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
