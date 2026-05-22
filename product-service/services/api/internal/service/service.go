package service

import (
	"context"
	"errors"

	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/internal/observability"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/internal/queue"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/internal/repository"

	"go.uber.org/zap"
)

type ProductService struct {
	repo    *repository.ProductRepository
	queue   *queue.Client
	log     *zap.Logger
	metrics *observability.Metrics
}

func NewProductService(
	repo *repository.ProductRepository,
	q *queue.Client,
	log *zap.Logger,
	m *observability.Metrics,
) *ProductService {
	return &ProductService{
		repo:    repo,
		queue:   q,
		log:     log,
		metrics: m,
	}
}

func (s *ProductService) CreateProduct(ctx context.Context, title, description, name string) (*repository.Product, error) {
	u, err := s.repo.Create(ctx, title, description, name)
	if err != nil {
		if errors.Is(err, repository.ErrDuplicateTitle) {
			return nil, repository.ErrDuplicateTitle
		}
		return nil, err
	}
	return u, nil
}

func (s *ProductService) GetProduct(ctx context.Context, id string) (*repository.Product, error) {
	return s.repo.Get(ctx, id)
}

func (s *ProductService) ListProductsPaged(ctx context.Context, page, limit int) ([]repository.Product, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	offset := (page - 1) * limit
	return s.repo.ListPaged(ctx, offset, limit)
}

func (s *ProductService) EnqueueDemo(ctx context.Context, payload string) error {
	return s.queue.Enqueue(ctx, "tasks", payload)
}
