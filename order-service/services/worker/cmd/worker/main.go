package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kaungmyathan18/golang-ecommerce-microservice/order-service/internal/config"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/order-service/internal/database"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/order-service/internal/observability"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/order-service/internal/rabbitmq"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/order-service/internal/repository"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/order-service/internal/inventoryclient"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/order-service/internal/orderservice"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

type paymentEvent struct {
	PaymentID string  `json:"payment_id"`
	OrderID   string  `json:"order_id"`
	UserID    string  `json:"user_id"`
	Amount    float64 `json:"amount"`
}

func main() {
	if err := config.InitEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	cfg := config.DefaultConfig("order-service-worker")
	logger, err := observability.NewLogger(cfg.Log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	ctxDB, cancelDB := context.WithTimeout(context.Background(), 10*time.Second)
	db, err := database.New(ctxDB, cfg.Database)
	cancelDB()
	if err != nil {
		logger.Fatal("database", zap.Error(err))
	}
	defer db.Close()

	repo := repository.NewRepository(db.SQL)
	inventory, err := inventoryclient.New(cfg.InventoryGRPC.Addr)
	if err != nil {
		logger.Fatal("inventory grpc", zap.Error(err))
	}
	defer inventory.Close()

	// Minimal service for confirm flow (no payment/product clients needed in worker).
	svc := orderservice.New(repo, nil, inventory, nil, logger, nil)

	rmq, err := rabbitmq.Connect(cfg.RabbitMQ.URL)
	if err != nil {
		logger.Fatal("rabbitmq", zap.Error(err))
	}
	defer rmq.Close()

	q, err := rmq.DeclareQueue("order.payment-completed", "payment.completed")
	if err != nil {
		logger.Fatal("queue declare", zap.Error(err))
	}
	msgs, err := rmq.Channel().Consume(q.Name, "order-payment-worker", false, false, false, false, nil)
	if err != nil {
		logger.Fatal("consume", zap.Error(err))
	}

	go func() {
		for d := range msgs {
			handlePaymentCompleted(logger, svc, d)
		}
	}()

	logger.Info("order worker started")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case <-sig:
			return
		case <-ticker.C:
			publishPending(context.Background(), logger, repo, rmq.Channel())
		}
	}
}

func handlePaymentCompleted(log *zap.Logger, svc *orderservice.OrderService, d amqp.Delivery) {
	var evt paymentEvent
	if err := json.Unmarshal(d.Body, &evt); err != nil {
		_ = d.Nack(false, false)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := svc.ConfirmOrderAfterPayment(ctx, evt.OrderID); err != nil {
		log.Error("confirm after payment", zap.Error(err), zap.String("order_id", evt.OrderID))
		_ = d.Nack(false, true)
		return
	}
	log.Info("order confirmed after payment", zap.String("order_id", evt.OrderID))
	_ = d.Ack(false)
}

func publishPending(ctx context.Context, log *zap.Logger, repo *repository.Repository, ch *amqp.Channel) {
	events, err := repo.FetchUnpublishedOutbox(ctx, 50)
	if err != nil {
		log.Error("fetch outbox", zap.Error(err))
		return
	}
	for _, e := range events {
		if err := rabbitmq.Publish(ch, e.EventType, e.Payload); err != nil {
			log.Error("publish event", zap.Error(err), zap.String("event_id", e.ID))
			continue
		}
		if err := repo.MarkOutboxPublished(ctx, e.ID); err != nil {
			log.Error("mark published", zap.Error(err), zap.String("event_id", e.ID))
		} else {
			log.Info("published event", zap.String("event_type", e.EventType), zap.String("event_id", e.ID))
		}
	}
}
