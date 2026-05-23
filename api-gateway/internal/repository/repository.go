package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"sort"
	"sync"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrDuplicateEmail = errors.New("duplicate email")
)

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type UserRepository struct {
	mu      sync.RWMutex
	users   map[string]User
	byEmail map[string]string
	log     *zap.Logger
}

func NewUserRepository(log *zap.Logger) *UserRepository {
	return &UserRepository{
		users:   make(map[string]User),
		byEmail: make(map[string]string),
		log:     log,
	}
}

func (r *UserRepository) Create(ctx context.Context, email, name string) (*User, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byEmail[email]; ok {
		return nil, ErrDuplicateEmail
	}
	u := User{
		ID:        uuid.NewString(),
		Email:     email,
		Name:      name,
		CreatedAt: time.Now().UTC(),
	}
	r.users[u.ID] = u
	r.byEmail[email] = u.ID
	return &u, nil
}

func (r *UserRepository) Get(ctx context.Context, id string) (*User, error) {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	return &u, nil
}

func (r *UserRepository) ListPaged(ctx context.Context, offset, limit int) ([]User, int64, error) {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()
	total := int64(len(r.users))
	list := make([]User, 0, len(r.users))
	for _, u := range r.users {
		list = append(list, u)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.Before(list[j].CreatedAt)
	})
	if offset >= len(list) {
		return []User{}, total, nil
	}
	end := offset + limit
	if end > len(list) {
		end = len(list)
	}
	return list[offset:end], total, nil
}
