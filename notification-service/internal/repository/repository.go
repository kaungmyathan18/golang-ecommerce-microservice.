package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type Notification struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	OrderID   string    `json:"order_id"`
	EventType string    `json:"event_type"`
	Message   string    `json:"message"`
	SentAt    time.Time `json:"sent_at"`
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, userID, orderID, eventType, message string) (*Notification, error) {
	n := Notification{
		ID: uuid.NewString(), UserID: userID, OrderID: orderID,
		EventType: eventType, Message: message, SentAt: time.Now().UTC(),
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO notifications (id, user_id, order_id, event_type, message, sent_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		n.ID, n.UserID, n.OrderID, n.EventType, n.Message, n.SentAt,
	)
	if err != nil {
		return nil, err
	}
	return &n, nil
}
