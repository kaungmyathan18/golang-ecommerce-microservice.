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

	"github.com/kaungmyathan18/golang-ecommerce-microservice/inventory-service/internal/config"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/inventory-service/internal/observability"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/inventory-service/internal/repository"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/inventory-service/services/api/internal/grpcserver"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/inventory-service/services/api/internal/handler"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/inventory-service/services/api/internal/service"
	pb "github.com/kaungmyathan18/golang-ecommerce-microservice/proto/inventory/pb"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
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
	cfg := config.DefaultConfig("inventory-service-api")
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
	svc := service.NewInventoryService(repo)
	health := handler.NewHealthHandler(logger, mongoClient)
	api := handler.NewAPIHandler(svc, logger)

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Route("/health", func(sr chi.Router) {
		sr.Get("/live", health.Live)
		sr.Get("/ready", health.Ready)
	})
	router.Route("/api/v1/inventory", func(ar chi.Router) {
		ar.Get("/{product_id}", api.GetStock)
		ar.Put("/{product_id}", api.SetStock)
	})

	srv := &http.Server{Addr: fmt.Sprintf(":%d", cfg.Server.Port), Handler: router}
	grpcSrv := grpc.NewServer()
	pb.RegisterInventoryServiceServer(grpcSrv, grpcserver.NewInventoryServer(svc))
	grpcLis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPC.Port))
	if err != nil {
		logger.Fatal("grpc listen", zap.Error(err))
	}

	errCh := make(chan error, 2)
	go func() { errCh <- srv.ListenAndServe() }()
	go func() { errCh <- grpcSrv.Serve(grpcLis) }()

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
