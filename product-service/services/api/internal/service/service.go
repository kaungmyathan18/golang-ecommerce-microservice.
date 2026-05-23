package service

import (
	"context"

	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/internal/inventoryclient"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/internal/observability"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/internal/repository"

	"go.uber.org/zap"
)

type CatalogService struct {
	repo      *repository.Repository
	inventory *inventoryclient.Client
	log       *zap.Logger
	metrics   *observability.Metrics
}

func NewCatalogService(repo *repository.Repository, inventory *inventoryclient.Client, log *zap.Logger, m *observability.Metrics) *CatalogService {
	return &CatalogService{repo: repo, inventory: inventory, log: log, metrics: m}
}

func (s *CatalogService) CreateCategory(ctx context.Context, name string) (*repository.Category, error) {
	return s.repo.CreateCategory(ctx, name)
}

func (s *CatalogService) ListCategories(ctx context.Context) ([]repository.Category, error) {
	return s.repo.ListCategories(ctx)
}

func (s *CatalogService) UpdateCategory(ctx context.Context, id, name string) (*repository.Category, error) {
	return s.repo.UpdateCategory(ctx, id, name)
}

func (s *CatalogService) DeleteCategory(ctx context.Context, id string) error {
	return s.repo.DeleteCategory(ctx, id)
}

func (s *CatalogService) CreateProduct(ctx context.Context, name, description string, price float64, categoryID string, stock int) (*repository.Product, error) {
	p, err := s.repo.CreateProduct(ctx, name, description, price, categoryID, 0)
	if err != nil {
		return nil, err
	}
	if err := s.inventory.SetStock(ctx, p.ID, stock); err != nil {
		return nil, err
	}
	p.Stock = stock
	return p, nil
}

func (s *CatalogService) GetProduct(ctx context.Context, id string) (*repository.Product, error) {
	p, err := s.repo.GetProduct(ctx, id)
	if err != nil {
		return nil, err
	}
	stock, err := s.inventory.GetStock(ctx, id)
	if err != nil {
		return nil, err
	}
	p.Stock = stock
	return p, nil
}

func (s *CatalogService) ListProducts(ctx context.Context, page, limit int) ([]repository.Product, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	products, total, err := s.repo.ListProducts(ctx, (page-1)*limit, limit)
	if err != nil {
		return nil, 0, err
	}
	for i := range products {
		stock, err := s.inventory.GetStock(ctx, products[i].ID)
		if err != nil {
			return nil, 0, err
		}
		products[i].Stock = stock
	}
	return products, total, nil
}

func (s *CatalogService) UpdateProduct(ctx context.Context, id, name, description string, price float64, categoryID string, stock int) (*repository.Product, error) {
	p, err := s.repo.UpdateProduct(ctx, id, name, description, price, categoryID, 0)
	if err != nil {
		return nil, err
	}
	if err := s.inventory.SetStock(ctx, id, stock); err != nil {
		return nil, err
	}
	p.Stock = stock
	return p, nil
}

func (s *CatalogService) DeleteProduct(ctx context.Context, id string) error {
	return s.repo.DeleteProduct(ctx, id)
}

func (s *CatalogService) GetProductCatalog(ctx context.Context, id string) (*repository.Product, error) {
	return s.repo.GetProduct(ctx, id)
}
