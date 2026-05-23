package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kaungmyathan18/golang-ecommerce-microservice/order-service/internal/config"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/order-service/internal/database"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/order-service/internal/observability"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/order-service/internal/repository"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/order-service/internal/inventoryclient"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/order-service/internal/orderservice"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/order-service/internal/paymentclient"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/order-service/internal/productclient"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/order-service/services/api/internal/handler"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.uber.org/zap"
)

func main() {
	if err := config.InitEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	cfg := config.DefaultConfig("order-service-api")
	logger, err := observability.NewLogger(cfg.Log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("starting", zap.String("app", cfg.App.Name))
	stopProfiling, err := observability.MaybeStartPyroscope(cfg.App.Name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pyroscope: %v\n", err)
		os.Exit(1)
	}
	defer stopProfiling()

	shutdownTracer, err := observability.NewTracerProvider(cfg.Otel)
	if err != nil {
		logger.Fatal("tracer", zap.Error(err))
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTracer.Shutdown(ctx)
	}()

	metrics, err := observability.NewMetrics()
	if err != nil {
		logger.Fatal("metrics", zap.Error(err))
	}

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

	productClient, err := productclient.New(cfg.ProductGRPC.Addr)
	if err != nil {
		logger.Fatal("product grpc", zap.Error(err))
	}
	defer productClient.Close()

	inventoryClient, err := inventoryclient.New(cfg.InventoryGRPC.Addr)
	if err != nil {
		logger.Fatal("inventory grpc", zap.Error(err))
	}
	defer inventoryClient.Close()

	paymentClient := paymentclient.New(cfg.Payment.ServiceURL)

	repo := repository.NewRepository(db.SQL)
	svc := orderservice.New(repo, productClient, inventoryClient, paymentClient, logger, metrics)
	health := handler.NewHealthHandler(logger, db)
	api := handler.NewAPIHandler(svc, logger, metrics)

	router := setupRouter(cfg, logger, metrics, health, api)
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:           router,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("listen", zap.Int("port", cfg.Server.Port))
		errCh <- srv.ListenAndServe()
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			logger.Fatal("server", zap.Error(err))
		}
	case <-sig:
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}
}

func setupRouter(cfg *config.Config, logger *zap.Logger, metrics *observability.Metrics, health *handler.HealthHandler, api *handler.APIHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	if cfg.Server.CORS.Enabled {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins: cfg.Server.CORS.AllowedOrigins, AllowedMethods: cfg.Server.CORS.AllowedMethods,
			AllowedHeaders: cfg.Server.CORS.AllowedHeaders, ExposedHeaders: cfg.Server.CORS.ExposedHeaders,
			AllowCredentials: cfg.Server.CORS.AllowCredentials, MaxAge: cfg.Server.CORS.MaxAge,
		}))
	}
	r.Use(observability.LoggingMiddleware(logger))
	r.Use(metrics.Middleware())
	r.Use(observability.TracingMiddleware)

	r.Route("/health", func(sr chi.Router) {
		sr.Get("/live", health.Live)
		sr.Get("/ready", health.Ready)
	})
	r.Handle("/metrics", metrics.Handler())

	r.Route("/api/v1", func(ar chi.Router) {
		ar.Use(middleware.Timeout(60 * time.Second))
		ar.Post("/orders", api.CreateOrder)
		ar.Get("/orders", api.ListOrders)
		ar.Get("/orders/{id}", api.GetOrder)
		ar.Put("/orders/{id}/cancel", api.CancelOrder)
		ar.Get("/orders/user/{user_id}", api.ListOrdersByUser)
	})
	return r
}
