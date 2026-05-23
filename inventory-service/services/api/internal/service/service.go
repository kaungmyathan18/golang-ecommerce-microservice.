package service

import (
	"context"

	"github.com/kaungmyathan18/golang-ecommerce-microservice/inventory-service/internal/repository"
)

type InventoryService struct {
	repo *repository.Repository
}

func NewInventoryService(repo *repository.Repository) *InventoryService {
	return &InventoryService{repo: repo}
}

func (s *InventoryService) GetStock(ctx context.Context, productID string) (int, error) {
	return s.repo.GetStock(ctx, productID)
}

func (s *InventoryService) CheckStock(ctx context.Context, productID string, qty int) (bool, int, error) {
	stock, err := s.repo.GetStock(ctx, productID)
	if err != nil {
		return false, 0, err
	}
	return stock >= qty, stock, nil
}

func (s *InventoryService) SetStock(ctx context.Context, productID string, qty int) (int, error) {
	return s.repo.SetStock(ctx, productID, qty)
}

func (s *InventoryService) DecrementStock(ctx context.Context, productID string, qty int) (int, error) {
	return s.repo.DecrementStock(ctx, productID, qty)
}

func (s *InventoryService) IncrementStock(ctx context.Context, productID string, qty int) (int, error) {
	return s.repo.IncrementStock(ctx, productID, qty)
}
