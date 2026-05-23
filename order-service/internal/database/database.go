package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kaungmyathan18/golang-ecommerce-microservice/order-service/internal/config"
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
)

// DB wraps database/sql for MySQL and SQLite.
type DB struct {
	SQL *sql.DB
}

func New(_ context.Context, cfg config.DatabaseConfig) (*DB, error) {
	var dsn string
	switch cfg.Driver {
	case "mysql":
		dsn = cfg.MySQLDSN()
	case "sqlite":
		dsn = cfg.DSN
	default:
		return nil, fmt.Errorf("unsupported driver %q", cfg.Driver)
	}
	db, err := sql.Open(cfg.Driver, dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return &DB{SQL: db}, nil
}

func (d *DB) Close() error {
	return d.SQL.Close()
}

func (d *DB) Ping(ctx context.Context) error {
	return d.SQL.PingContext(ctx)
}

func RunMigrations(ctx context.Context, d *DB, dir string) error {
	return runSQLFiles(ctx, func(ctx context.Context, q string) error {
		_, err := d.SQL.ExecContext(ctx, q)
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
