package orderservice

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/order-service/internal/inventoryclient"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/order-service/internal/observability"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/order-service/internal/paymentclient"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/order-service/internal/productclient"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/order-service/internal/repository"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type OrderEventPayload struct {
	OrderID   string  `json:"order_id"`
	UserID    string  `json:"user_id"`
	ProductID string  `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Total     float64 `json:"total_price"`
}

type OrderService struct {
	repo      *repository.Repository
	product   *productclient.Client
	inventory *inventoryclient.Client
	payment   *paymentclient.Client
	log       *zap.Logger
	metrics   *observability.Metrics
}

func New(
	repo *repository.Repository,
	product *productclient.Client,
	inventory *inventoryclient.Client,
	payment *paymentclient.Client,
	log *zap.Logger,
	m *observability.Metrics,
) *OrderService {
	return &OrderService{repo: repo, product: product, inventory: inventory, payment: payment, log: log, metrics: m}
}

func (s *OrderService) CreateOrder(ctx context.Context, userID, productID string, quantity int) (*repository.Order, error) {
	if quantity < 1 {
		return nil, errors.New("quantity must be positive")
	}
	product, err := s.product.GetProduct(ctx, productID)
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	stock, err := s.inventory.CheckStock(ctx, productID, int32(quantity))
	if err != nil {
		return nil, err
	}
	if !stock.GetAvailable() {
		return nil, errors.New("insufficient stock")
	}
	total := product.GetPrice() * float64(quantity)
	order := repository.Order{
		ID: uuid.NewString(), UserID: userID, ProductID: productID,
		Quantity: quantity, TotalPrice: total, Status: repository.StatusPending,
	}
	payload := OrderEventPayload{
		OrderID: order.ID, UserID: userID, ProductID: productID,
		Quantity: quantity, Total: total,
	}
	if err := s.repo.CreateOrderWithOutbox(ctx, order, "order.created", payload); err != nil {
		return nil, err
	}
	if _, err := s.payment.InitiatePayment(ctx, order.ID, userID, total); err != nil {
		s.log.Error("initiate payment", zap.Error(err), zap.String("order_id", order.ID))
		return nil, err
	}
	return s.repo.GetOrder(ctx, order.ID)
}

func (s *OrderService) ConfirmOrderAfterPayment(ctx context.Context, orderID string) error {
	order, err := s.repo.GetOrder(ctx, orderID)
	if err != nil {
		return err
	}
	if order.Status != repository.StatusPending {
		return nil
	}
	if _, err := s.inventory.DecrementStock(ctx, order.ProductID, int32(order.Quantity)); err != nil {
		return err
	}
	payload := OrderEventPayload{
		OrderID: order.ID, UserID: order.UserID, ProductID: order.ProductID,
		Quantity: order.Quantity, Total: order.TotalPrice,
	}
	return s.repo.ConfirmOrderWithOutbox(ctx, orderID, payload)
}

func (s *OrderService) GetOrder(ctx context.Context, id string) (*repository.Order, error) {
	return s.repo.GetOrder(ctx, id)
}

func (s *OrderService) ListOrders(ctx context.Context, page, limit int) ([]repository.Order, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	return s.repo.ListOrders(ctx, (page-1)*limit, limit)
}

func (s *OrderService) ListOrdersByUser(ctx context.Context, userID string) ([]repository.Order, error) {
	return s.repo.ListOrdersByUser(ctx, userID)
}

func (s *OrderService) CancelOrder(ctx context.Context, id string) (*repository.Order, error) {
	order, err := s.repo.GetOrder(ctx, id)
	if err != nil {
		return nil, err
	}
	payload := OrderEventPayload{
		OrderID: order.ID, UserID: order.UserID, ProductID: order.ProductID,
		Quantity: order.Quantity, Total: order.TotalPrice,
	}
	if err := s.repo.CancelOrderWithOutbox(ctx, id, payload); err != nil {
		return nil, err
	}
	return s.repo.GetOrder(ctx, id)
}
