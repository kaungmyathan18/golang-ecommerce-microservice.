package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kaungmyathan18/golang-ecommerce-microservice/notification-service/internal/config"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/notification-service/internal/database"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/notification-service/internal/observability"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/notification-service/internal/rabbitmq"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/notification-service/internal/repository"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

type orderEvent struct {
	OrderID   string  `json:"order_id"`
	UserID    string  `json:"user_id"`
	ProductID string  `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Total     float64 `json:"total_price"`
}

func main() {
	if err := config.InitEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	cfg := config.DefaultConfig("notification-service")
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

	if cfg.Database.AutoMigrate {
		if err := database.RunMigrations(context.Background(), db, cfg.Database.MigrationsPath); err != nil {
			logger.Fatal("migrations", zap.Error(err))
		}
	}

	repo := repository.NewRepository(db.SQL)
	rmq, err := rabbitmq.Connect(cfg.RabbitMQ.URL)
	if err != nil {
		logger.Fatal("rabbitmq", zap.Error(err))
	}
	defer rmq.Close()

	for _, binding := range []struct{ queue, key, tag string }{
		{"notification.order-confirmed", "order.confirmed", "notification-confirmed"},
		{"notification.order-cancelled", "order.cancelled", "notification-cancelled"},
	} {
		if _, err := rmq.DeclareQueue(binding.queue, binding.key); err != nil {
			logger.Fatal("queue declare", zap.Error(err), zap.String("queue", binding.queue))
		}
		ch, err := rmq.Channel().Consume(binding.queue, binding.tag, false, false, false, false, nil)
		if err != nil {
			logger.Fatal("consume", zap.Error(err))
		}
		go func(msgs <-chan amqp.Delivery) {
			for d := range msgs {
				handleDelivery(logger, repo, d)
			}
		}(ch)
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Get("/health/live", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Get("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(r.Context()); err != nil {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{Addr: fmt.Sprintf(":%d", cfg.Server.Port), Handler: r}
	go func() {
		logger.Info("health server", zap.Int("port", cfg.Server.Port))
		_ = srv.ListenAndServe()
	}()

	logger.Info("notification consumer started")
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	logger.Info("shutting down")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
}

func handleDelivery(log *zap.Logger, repo *repository.Repository, d amqp.Delivery) {
	var evt orderEvent
	if err := json.Unmarshal(d.Body, &evt); err != nil {
		log.Warn("invalid payload", zap.Error(err))
		_ = d.Nack(false, false)
		return
	}
	msg := fmt.Sprintf("[EMAIL STUB] event=%s order=%s user=%s", d.RoutingKey, evt.OrderID, evt.UserID)
	log.Info(msg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := repo.Create(ctx, evt.UserID, evt.OrderID, d.RoutingKey, msg); err != nil {
		log.Error("save notification", zap.Error(err))
		_ = d.Nack(false, true)
		return
	}
	_ = d.Ack(false)
}
