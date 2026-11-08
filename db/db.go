package db

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Db struct {
	Pool *pgxpool.Pool
}

const defaultConnectTimeout = 15 * time.Second

// New opens and verifies a PostgreSQL connection using the current process
// environment. DATABASE_URL is preferred because it preserves DigitalOcean's
// TLS settings and safely encodes credentials.
func New() (*Db, error) {
	return NewContext(context.Background())
}

// NewContext opens and verifies a PostgreSQL connection using ctx.
func NewContext(ctx context.Context) (*Db, error) {
	conf, err := getCfg()
	if err != nil {
		return nil, err
	}

	pool, err := createPool(ctx, conf.URL)
	if err != nil {
		return nil, fmt.Errorf("database: %w", err)
	}

	return &Db{
		Pool: pool,
	}, nil
}

func createPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse connection URL: %w", err)
	}
	if cfg.ConnConfig.ConnectTimeout == 0 {
		cfg.ConnConfig.ConnectTimeout = defaultConnectTimeout
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}

func getCfg() (*Conf, error) {
	url := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if url == "" {
		url = strings.TrimSpace(os.Getenv("DB_URL"))
	}
	if url == "" {
		return nil, fmt.Errorf("DATABASE_URL or DB_URL is required")
	}

	return &Conf{URL: url}, nil
}

func (db *Db) Close() {
	if db.Pool != nil {
		db.Pool.Close()
	}
}
