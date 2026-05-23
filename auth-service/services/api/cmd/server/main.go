package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kaungmyathan18/golang-ecommerce-microservice/auth-service/internal/config"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/auth-service/internal/database"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/auth-service/internal/observability"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/auth-service/internal/repository"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/auth-service/internal/token"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/auth-service/services/api/internal/handler"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/auth-service/services/api/internal/service"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

func main() {
	if err := config.InitEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	cfg := config.DefaultConfig("auth-service-api")
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
	tokens := token.NewIssuer(cfg.JWT.Secret, cfg.JWT.TTL)
	svc := service.NewAuthService(repo, tokens)
	health := handler.NewHealthHandler(logger, db)
	api := handler.NewAPIHandler(svc, tokens, logger)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Route("/health", func(sr chi.Router) {
		sr.Get("/live", health.Live)
		sr.Get("/ready", health.Ready)
	})
	r.Route("/api/v1/auth", func(ar chi.Router) {
		ar.Post("/register", api.Register)
		ar.Post("/login", api.Login)
		ar.Get("/me", api.Me)
	})

	srv := &http.Server{Addr: fmt.Sprintf(":%d", cfg.Server.Port), Handler: r}
	go func() { logger.Info("listen", zap.Int("port", cfg.Server.Port)); _ = srv.ListenAndServe() }()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
