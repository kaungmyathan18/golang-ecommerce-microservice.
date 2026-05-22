package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
}

func New(ctx context.Context, cfg config.DatabaseConfig) (*DB, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.PostgresDSN())
	if err != nil {
		return nil, err
	}
	poolCfg.MaxConns = int32(cfg.MaxOpenConns)
	poolCfg.MaxConnLifetime = cfg.ConnMaxLifetime
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &DB{Pool: pool}, nil
}

func (d *DB) Close() error {
	d.Pool.Close()
	return nil
}

func (d *DB) Ping(ctx context.Context) error {
	return d.Pool.Ping(ctx)
}

func RunMigrations(ctx context.Context, d *DB, dir string) error {
	return runSQLFiles(ctx, func(ctx context.Context, q string) error {
		_, err := d.Pool.Exec(ctx, q)
		return err
	}, dir)
}

func runSQLFiles(ctx context.Context, exec func(context.Context, string) error, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		files = append(files, e.Name())
	}
	sort.Strings(files)
	for _, name := range files {
		path := filepath.Join(dir, name)
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		if err := exec(ctx, string(body)); err != nil {
			cancel()
			return fmt.Errorf("%s: %w", name, err)
		}
		cancel()
	}
	return nil
}
