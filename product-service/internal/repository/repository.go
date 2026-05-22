package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/internal/database"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrDuplicateTitle = errors.New("duplicate title")
)

type Product struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Price       float64   `json:"price"`
	Name        string    `json:"name"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ProductRepository struct {
	pool *pgxpool.Pool
	log  *zap.Logger
}

func NewProductRepository(db *database.DB, log *zap.Logger) *ProductRepository {
	return &ProductRepository{pool: db.Pool, log: log}
}

func (r *ProductRepository) Create(ctx context.Context, title, description, name string) (*Product, error) {
	u := Product{
		ID:          uuid.NewString(),
		Title:       title,
		Description: description,
		Name:        name,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO products (id, title, description, name, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6)`,
		u.ID, u.Title, u.Description, u.Name, u.CreatedAt, u.UpdatedAt,
	)
	if err != nil {
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Code == "23505" {
			return nil, ErrDuplicateTitle
		}
		return nil, err
	}
	return &u, nil
}

func (r *ProductRepository) Get(ctx context.Context, id string) (*Product, error) {
	var u Product
	err := r.pool.QueryRow(ctx,
		`SELECT id, title, description, name, created_at, updated_at FROM products WHERE id = $1`, id,
	).Scan(&u.ID, &u.Title, &u.Description, &u.Name, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *ProductRepository) ListPaged(ctx context.Context, offset, limit int) ([]Product, int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM products`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, title, description, name, created_at, updated_at FROM products ORDER BY created_at ASC LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Product
	for rows.Next() {
		var u Product
		if err := rows.Scan(&u.ID, &u.Title, &u.Description, &u.Name, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, u)
	}
	return out, total, rows.Err()
}
