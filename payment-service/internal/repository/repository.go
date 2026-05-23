package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("not found")

type PaymentStatus string

const (
	StatusPending   PaymentStatus = "pending"
	StatusCompleted PaymentStatus = "completed"
	StatusFailed    PaymentStatus = "failed"
)

type Payment struct {
	ID        string        `json:"id"`
	OrderID   string        `json:"order_id"`
	UserID    string        `json:"user_id"`
	Amount    float64       `json:"amount"`
	Status    PaymentStatus `json:"status"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, orderID, userID string, amount float64) (*Payment, error) {
	p := Payment{
		ID: uuid.NewString(), OrderID: orderID, UserID: userID,
		Amount: amount, Status: StatusPending, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO payments (id, order_id, user_id, amount, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.OrderID, p.UserID, p.Amount, p.Status, p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *Repository) Get(ctx context.Context, id string) (*Payment, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, order_id, user_id, amount, status, created_at, updated_at
		FROM payments WHERE id = ?`, id)
	var p Payment
	var status string
	if err := row.Scan(&p.ID, &p.OrderID, &p.UserID, &p.Amount, &status, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	p.Status = PaymentStatus(status)
	return &p, nil
}

func (r *Repository) MarkCompleted(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE payments SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND status = ?`,
		StatusCompleted, id, StatusPending,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
