package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound          = errors.New("not found")
	ErrInvalidTransition = errors.New("invalid status transition")
)

type OrderStatus string

const (
	StatusPending    OrderStatus = "pending"
	StatusConfirmed  OrderStatus = "confirmed"
	StatusShipped    OrderStatus = "shipped"
	StatusDelivered  OrderStatus = "delivered"
	StatusCancelled  OrderStatus = "cancelled"
)

type Order struct {
	ID               string      `json:"id"`
	UserID           string      `json:"user_id"`
	ProductID        string      `json:"product_id"`
	Quantity         int         `json:"quantity"`
	TotalPrice       float64     `json:"total_price"`
	Status           OrderStatus `json:"status"`
	StockDecremented bool        `json:"stock_decremented"`
	CreatedAt        time.Time   `json:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at"`
}

type OutboxEvent struct {
	ID        string
	EventType string
	Payload   json.RawMessage
	Published bool
	CreatedAt time.Time
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateOrderWithOutbox(ctx context.Context, order Order, eventType string, payload any) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO orders (id, user_id, product_id, quantity, total_price, status, stock_decremented)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		order.ID, order.UserID, order.ProductID, order.Quantity, order.TotalPrice, order.Status, order.StockDecremented,
	)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO outbox (id, event_type, payload, published)
		VALUES (?, ?, ?, false)`,
		uuid.NewString(), eventType, payloadBytes,
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) GetOrder(ctx context.Context, id string) (*Order, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, product_id, quantity, total_price, status, stock_decremented, created_at, updated_at
		FROM orders WHERE id = ?`, id)
	return scanOrder(row)
}

func (r *Repository) ListOrders(ctx context.Context, offset, limit int) ([]Order, int64, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM orders`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, product_id, quantity, total_price, status, stock_decremented, created_at, updated_at
		FROM orders ORDER BY created_at ASC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Order
	for rows.Next() {
		o, err := scanOrderRows(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *o)
	}
	return out, total, rows.Err()
}

func (r *Repository) ListOrdersByUser(ctx context.Context, userID string) ([]Order, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, product_id, quantity, total_price, status, stock_decremented, created_at, updated_at
		FROM orders WHERE user_id = ? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Order
	for rows.Next() {
		o, err := scanOrderRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *o)
	}
	return out, rows.Err()
}

func (r *Repository) ConfirmOrderWithOutbox(ctx context.Context, orderID string, payload any) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `
		UPDATE orders SET status = ?, stock_decremented = true, updated_at = NOW()
		WHERE id = ? AND status = ?`, StatusConfirmed, orderID, StatusPending)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrInvalidTransition
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO outbox (id, event_type, payload, published)
		VALUES (?, ?, ?, false)`,
		uuid.NewString(), "order.confirmed", payloadBytes,
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) CancelOrderWithOutbox(ctx context.Context, orderID string, payload any) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var status OrderStatus
	var stockDec bool
	err = tx.QueryRowContext(ctx, `SELECT status, stock_decremented FROM orders WHERE id = ? FOR UPDATE`, orderID).Scan(&status, &stockDec)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if status != StatusPending && status != StatusConfirmed {
		return ErrInvalidTransition
	}
	_, err = tx.ExecContext(ctx, `UPDATE orders SET status = ?, updated_at = NOW() WHERE id = ?`, StatusCancelled, orderID)
	if err != nil {
		return err
	}
	if stockDec {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO outbox (id, event_type, payload, published)
			VALUES (?, ?, ?, false)`,
			uuid.NewString(), "order.cancelled", payloadBytes,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *Repository) FetchUnpublishedOutbox(ctx context.Context, limit int) ([]OutboxEvent, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, event_type, payload, published, created_at
		FROM outbox WHERE published = false ORDER BY created_at ASC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OutboxEvent
	for rows.Next() {
		var e OutboxEvent
		if err := rows.Scan(&e.ID, &e.EventType, &e.Payload, &e.Published, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Repository) MarkOutboxPublished(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE outbox SET published = true WHERE id = ?`, id)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanOrder(row rowScanner) (*Order, error) {
	var o Order
	var status string
	err := row.Scan(&o.ID, &o.UserID, &o.ProductID, &o.Quantity, &o.TotalPrice, &status, &o.StockDecremented, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	o.Status = OrderStatus(status)
	return &o, nil
}

func scanOrderRows(rows *sql.Rows) (*Order, error) {
	var o Order
	var status string
	err := rows.Scan(&o.ID, &o.UserID, &o.ProductID, &o.Quantity, &o.TotalPrice, &status, &o.StockDecremented, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return nil, err
	}
	o.Status = OrderStatus(status)
	return &o, nil
}
