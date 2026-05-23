package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/kaungmyathan18/golang-ecommerce-microservice/payment-service/internal/config"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/payment-service/internal/rabbitmq"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/payment-service/internal/repository"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

type PaymentEvent struct {
	PaymentID string  `json:"payment_id"`
	OrderID   string  `json:"order_id"`
	UserID    string  `json:"user_id"`
	Amount    float64 `json:"amount"`
}

type PaymentService struct {
	repo   *repository.Repository
	rmqCh  *amqp.Channel
	cfg    *config.Config
	log    *zap.Logger
}

func NewPaymentService(repo *repository.Repository, rmqCh *amqp.Channel, cfg *config.Config, log *zap.Logger) *PaymentService {
	return &PaymentService{repo: repo, rmqCh: rmqCh, cfg: cfg, log: log}
}

func (s *PaymentService) InitiatePayment(ctx context.Context, orderID, userID string, amount float64) (*repository.Payment, error) {
	p, err := s.repo.Create(ctx, orderID, userID, amount)
	if err != nil {
		return nil, err
	}
	go s.processStubPayment(p)
	return p, nil
}

func (s *PaymentService) GetPayment(ctx context.Context, id string) (*repository.Payment, error) {
	return s.repo.Get(ctx, id)
}

func (s *PaymentService) processStubPayment(p *repository.Payment) {
	time.Sleep(s.cfg.Payment.StubDelay)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.repo.MarkCompleted(ctx, p.ID); err != nil {
		s.log.Error("mark payment completed", zap.Error(err))
		return
	}
	evt := PaymentEvent{PaymentID: p.ID, OrderID: p.OrderID, UserID: p.UserID, Amount: p.Amount}
	body, _ := json.Marshal(evt)
	if err := rabbitmq.Publish(s.rmqCh, "payment.completed", body); err != nil {
		s.log.Error("publish payment.completed", zap.Error(err))
		return
	}
	s.log.Info("payment completed", zap.String("payment_id", p.ID), zap.String("order_id", p.OrderID))
}
