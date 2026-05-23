package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/internal/config"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/internal/observability"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/internal/rabbitmq"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/internal/repository"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

type cancelEvent struct {
	OrderID   string `json:"order_id"`
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

func main() {
	if err := config.InitEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	cfg := config.DefaultConfig("product-service-worker")
	logger, err := observability.NewLogger(cfg.Log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	ctxM, cancelM := context.WithTimeout(context.Background(), 10*time.Second)
	mongoClient, err := mongo.Connect(ctxM, options.Client().ApplyURI(cfg.Mongo.URI))
	cancelM()
	if err != nil {
		logger.Fatal("mongo", zap.Error(err))
	}
	defer mongoClient.Disconnect(context.Background())

	repo := repository.NewRepository(mongoClient, cfg.Mongo.Database, logger)

	rmq, err := rabbitmq.Connect(cfg.RabbitMQ.URL)
	if err != nil {
		logger.Fatal("rabbitmq", zap.Error(err))
	}
	defer rmq.Close()

	q, err := rmq.DeclareQueue("product.order-cancelled", "order.cancelled")
	if err != nil {
		logger.Fatal("queue declare", zap.Error(err))
	}

	msgs, err := rmq.Channel().Consume(q.Name, "product-worker", false, false, false, false, nil)
	if err != nil {
		logger.Fatal("consume", zap.Error(err))
	}

	logger.Info("product worker started", zap.String("queue", q.Name))

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-sig:
			logger.Info("shutting down")
			return
		case d, ok := <-msgs:
			if !ok {
				return
			}
			handleDelivery(logger, repo, d)
		}
	}
}

func handleDelivery(log *zap.Logger, repo *repository.Repository, d amqp.Delivery) {
	var evt cancelEvent
	if err := json.Unmarshal(d.Body, &evt); err != nil {
		log.Warn("invalid payload", zap.Error(err))
		_ = d.Nack(false, false)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stock, err := repo.IncrementStock(ctx, evt.ProductID, evt.Quantity)
	if err != nil {
		log.Error("restore stock", zap.Error(err), zap.String("order_id", evt.OrderID))
		_ = d.Nack(false, true)
		return
	}
	log.Info("stock restored",
		zap.String("order_id", evt.OrderID),
		zap.String("product_id", evt.ProductID),
		zap.Int("quantity", evt.Quantity),
		zap.Int("stock", stock),
	)
	_ = d.Ack(false)
}
