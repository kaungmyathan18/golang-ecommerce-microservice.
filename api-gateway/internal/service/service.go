package service

import (
	"context"
	"errors"

	"github.com/kaungmyathan18/golang-ecommerce-microservice/api-gateway/internal/repository"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/api-gateway/internal/observability"

	"go.uber.org/zap"
)

type UserService struct {
	repo    *repository.UserRepository
	log     *zap.Logger
	metrics *observability.Metrics
}

func NewUserService(
	repo *repository.UserRepository,
	log *zap.Logger,
	m *observability.Metrics,
) *UserService {
	return &UserService{
		repo:    repo,
		log:     log,
		metrics: m,
	}
}

func (s *UserService) CreateUser(ctx context.Context, email, name string) (*repository.User, error) {
	u, err := s.repo.Create(ctx, email, name)
	if err != nil {
		if errors.Is(err, repository.ErrDuplicateEmail) {
			return nil, repository.ErrDuplicateEmail
		}
		return nil, err
	}
	return u, nil
}

func (s *UserService) GetUser(ctx context.Context, id string) (*repository.User, error) {
	return s.repo.Get(ctx, id)
}

func (s *UserService) ListUsersPaged(ctx context.Context, page, limit int) ([]repository.User, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	offset := (page - 1) * limit
	return s.repo.ListPaged(ctx, offset, limit)
}

// HandleQueuePayload processes a dequeued message (stub).
func (s *UserService) HandleQueuePayload(ctx context.Context, payload string) error {
	s.log.Info("queue payload", zap.String("payload", payload))
	return nil
}

func (s *UserService) PingDeps(ctx context.Context) error {
	_ = ctx
	return nil
}
