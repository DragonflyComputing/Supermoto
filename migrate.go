package supermoto

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

/*
Supermoto Migration

A simple, forward-only database migration system for PostgreSQL using PGX connection pools.

Features:
- Sequential migration execution based on numeric version prefixes
- Migration tracking in schema_migrations table
- Concurrent migration protection via advisory locks
- Filename format: NNN_description.sql (e.g., 002_create_users.sql)

Requires PGX connection pools:
https://github.com/jackc/pgx/wiki/Getting-started-with-pgx#using-a-connection-pool
*/

const migrationLockID = 307839

// Migrate applies all pending database migrations from the specified directory.
// Migrations must follow the naming convention: NNN_description.sql where NNN is a numeric version.
// Returns an error if any migration fails or if the migration directory cannot be accessed.
// Pass nil for logger to use the default standard library logger.
func Migrate(ctx context.Context, migrationsDir string, pool *pgxpool.Pool, logger *log.Logger) error {
	if logger == nil {
		logger = log.Default()
	}

	// Validate inputs
	if pool == nil {
		return fmt.Errorf("database pool cannot be nil")
	}
	if migrationsDir == "" {
		return fmt.Errorf("migrations directory cannot be empty")
	}

	// Clean the path to prevent directory traversal
	migrationsDir = filepath.Clean(migrationsDir)

	// Check if context is already cancelled before starting
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Acquire advisory lock to prevent concurrent migrations
	_, err := pool.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLockID)
	if err != nil {
		return fmt.Errorf("failed to acquire migration lock: %w", err)
	}

	var lockReleased bool
	defer func() {
		if !lockReleased {
			// Use background context for lock release to ensure it completes
			// even if the original context was cancelled
			if _, err := pool.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", migrationLockID); err != nil {
				logger.Printf("CRITICAL: Failed to release migration lock %d: %v - manual intervention may be required", migrationLockID, err)
			} else {
				lockReleased = true
			}
		}
	}()

	// Read migration files
	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory '%s': %w", migrationsDir, err)
	}

	// Ensure tracking table exists
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER NOT NULL PRIMARY KEY,
			filename VARCHAR(255) NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`)
	if err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	// Get applied migrations
	applied := make(map[string]bool)
	rows, err := pool.Query(ctx, "SELECT filename FROM schema_migrations")
	if err != nil {
		return fmt.Errorf("failed to query applied migrations: %w", err)
	}

	for rows.Next() {
		var filename string
		if err := rows.Scan(&filename); err != nil {
			rows.Close()
			return fmt.Errorf("failed to scan migration filename: %w", err)
		}
		applied[filename] = true
	}

	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("database error while reading applied migrations: %w", err)
	}
	rows.Close()

	// Parse and sort migration files
	var migrations []migration
	for _, file := range files {
		// Skip subdirectories
		if file.IsDir() {
			continue
		}

		filename := file.Name()

		// Filename validation
		if !strings.HasSuffix(filename, ".sql") {
			return fmt.Errorf("invalid migration file '%s': must end with .sql", filename)
		}

		// Extract version from filename: "001_description.sql"
		parts := strings.Split(filename, "_")
		if len(parts) < 2 {
			return fmt.Errorf("invalid migration filename '%s': must follow format 'NNN_description.sql' (e.g., '001_create_users.sql')", filename)
		}

		version := parts[0]
		if version == "" {
			return fmt.Errorf("invalid migration filename '%s': version number cannot be empty, must follow format 'NNN_description.sql'", filename)
		}

		// Validate and convert version to an integer
		versionInt, err := strconv.Atoi(version)
		if err != nil {
			return fmt.Errorf("invalid migration filename '%s': version '%s' must be numeric, must follow format 'NNN_description.sql'", filename, version)
		}

		// Validate version is not negative
		if versionInt < 0 {
			return fmt.Errorf("invalid migration filename '%s': version number cannot be negative", filename)
		}

		// Validate version fits in int32
		if versionInt > 2147483647 {
			return fmt.Errorf("invalid migration filename '%s': version number %d exceeds maximum value", filename, versionInt)
		}

		migrations = append(migrations, migration{
			version:  int32(versionInt),
			filename: filename,
		})
	}

	// Check for duplicate version numbers
	versionMap := make(map[int32]string)
	for _, m := range migrations {
		if existingFile, exists := versionMap[m.version]; exists {
			return fmt.Errorf("duplicate migration version %d found in files: %s and %s",
				m.version, existingFile, m.filename)
		}
		versionMap[m.version] = m.filename
	}

	// Sort migrations by numeric version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})

	if len(migrations) == 0 {
		logger.Println("No migration files found - nothing to apply")
		return nil
	}

	// Apply pending migrations
	for _, m := range migrations {
		// Check for context cancellation before each migration
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if applied[m.filename] {
			logger.Printf("Skipping already applied migration: %s", m.filename)
			continue
		}

		logger.Printf("Applying migration: %s", m.filename)

		filePath := filepath.Join(migrationsDir, m.filename)
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", m.filename, err)
		}

		if err := applyMigration(ctx, pool, m, string(content), logger); err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", m.filename, err)
		}

		logger.Printf("Applied migration: %s", m.filename)
	}

	logger.Println("All migrations applied successfully")
	return nil
}

// migration represents a single database migration with its version and filename.
type migration struct {
	version  int32
	filename string
}

// applyMigration executes a single migration within a transaction.
// If the migration SQL fails, the transaction is rolled back and the migration is not recorded.
// Only successful migrations are recorded in the schema_migrations table.
func applyMigration(ctx context.Context, pool *pgxpool.Pool, m migration, sql string, logger *log.Logger) error {
	// Check for context cancellation before starting migration
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	sql = strings.TrimSpace(sql)
	if sql == "" {
		return fmt.Errorf("migration SQL cannot be empty")
	}

	// Start transaction
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin migration transaction: %w", err)
	}

	var committed bool
	defer func() {
		if !committed {
			if err := tx.Rollback(ctx); err != nil {
				logger.Printf("Failed to rollback transaction: %v", err)
			}
		}
	}()

	// Execute migration SQL
	if _, err := tx.Exec(ctx, sql); err != nil {
		return fmt.Errorf("failed to execute migration SQL: %w", err)
	}

	// Record migration with both version and filename
	if _, err := tx.Exec(ctx,
		"INSERT INTO schema_migrations (version, filename) VALUES ($1, $2)",
		m.version, m.filename); err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit migration: %w", err)
	}
	committed = true

	return nil
}
