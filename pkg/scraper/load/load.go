// Package load incrementally persists scraper output in PostgreSQL.
//
// Loaders add missing source detail records without rewriting them. Complete MP
// and officer report loads do refresh their report-subject summary metadata so a
// re-scrape can correct source totals and category URLs.
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
