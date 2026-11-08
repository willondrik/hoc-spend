// Package load incrementally persists scraper output in PostgreSQL.
//
// Loaders are additive: they identify source records that are already present
// and insert only records that are new. They do not update existing rows.
package load

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Loader incrementally writes scraper results through a PostgreSQL pool.
type Loader struct {
	pool *pgxpool.Pool
}

// New creates a Loader backed by pool.
func New(pool *pgxpool.Pool) (*Loader, error) {
	if pool == nil {
		return nil, fmt.Errorf("load: pool is required")
	}

	return &Loader{pool: pool}, nil
}

// Ping verifies that the loader can reach its configured database.
func (l *Loader) Ping(ctx context.Context) error {
	if err := l.pool.Ping(ctx); err != nil {
		return fmt.Errorf("load: ping database: %w", err)
	}
	return nil
}
