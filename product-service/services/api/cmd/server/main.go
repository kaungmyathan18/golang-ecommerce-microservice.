package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/internal/config"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/internal/inventoryclient"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/internal/observability"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/internal/repository"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/services/api/internal/grpcserver"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/services/api/internal/handler"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/services/api/internal/service"
	pb "github.com/kaungmyathan18/golang-ecommerce-microservice/proto/product/pb"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func main() {
	if err := config.InitEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	cfg := config.DefaultConfig("product-service-api")

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

	ctxM, cancelM := context.WithTimeout(context.Background(), 10*time.Second)
	mongoClient, err := mongo.Connect(ctxM, options.Client().ApplyURI(cfg.Mongo.URI))
	cancelM()
	if err != nil {
		logger.Fatal("mongo", zap.Error(err))
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = mongoClient.Disconnect(ctx)
	}()

	if err := repository.EnsureIndexes(context.Background(), mongoClient, cfg.Mongo.Database); err != nil {
		logger.Fatal("mongo indexes", zap.Error(err))
	}

	repo := repository.NewRepository(mongoClient, cfg.Mongo.Database, logger)
	inventoryClient := inventoryclient.New(cfg.Inventory.ServiceURL)
	svc := service.NewCatalogService(repo, inventoryClient, logger, metrics)

	health := handler.NewHealthHandler(logger, mongoClient)
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

	grpcSrv := grpc.NewServer()
	pb.RegisterProductServiceServer(grpcSrv, grpcserver.NewProductServer(svc))
	grpcLis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPC.Port))
	if err != nil {
		logger.Fatal("grpc listen", zap.Error(err))
	}

	errCh := make(chan error, 2)
	go func() {
		logger.Info("http listen", zap.Int("port", cfg.Server.Port))
		errCh <- srv.ListenAndServe()
	}()
	go func() {
		logger.Info("grpc listen", zap.Int("port", cfg.GRPC.Port))
		errCh <- grpcSrv.Serve(grpcLis)
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
		grpcSrv.GracefulStop()
		_ = srv.Shutdown(ctx)
	}
}

func setupRouter(
	cfg *config.Config,
	logger *zap.Logger,
	metrics *observability.Metrics,
	health *handler.HealthHandler,
	api *handler.APIHandler,
) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	if cfg.Server.CORS.Enabled {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   cfg.Server.CORS.AllowedOrigins,
			AllowedMethods:   cfg.Server.CORS.AllowedMethods,
			AllowedHeaders:   cfg.Server.CORS.AllowedHeaders,
			ExposedHeaders:   cfg.Server.CORS.ExposedHeaders,
			AllowCredentials: cfg.Server.CORS.AllowCredentials,
			MaxAge:           cfg.Server.CORS.MaxAge,
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
		ar.Post("/categories", api.CreateCategory)
		ar.Get("/categories", api.ListCategories)
		ar.Put("/categories/{id}", api.UpdateCategory)
		ar.Delete("/categories/{id}", api.DeleteCategory)
		ar.Post("/products", api.CreateProduct)
		ar.Get("/products", api.ListProducts)
		ar.Get("/products/{id}", api.GetProduct)
		ar.Put("/products/{id}", api.UpdateProduct)
		ar.Delete("/products/{id}", api.DeleteProduct)
	})
	return r
}
