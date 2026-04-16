package supermoto

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

/*
Supermoto Database

Provides a simple wrapper around pgx connection pools for PostgreSQL.

Features:
- Connection pool creation with immediate validation
- Descriptive errors on connection failure

Requires pgx/v5:
https://github.com/jackc/pgx
*/

// Connect creates a new pgx connection pool and verifies the connection with a ping.
// Returns an error if the pool cannot be created or the database is unreachable.
// Pass nil for logger to use the default standard library logger.
func Connect(ctx context.Context, databaseURL string, logger *log.Logger) (*pgxpool.Pool, error) {
	if logger == nil {
		logger = log.Default()
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	// Verify the connection is actually reachable before returning
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	logger.Println("Successfully connected to database")
	return pool, nil
}
