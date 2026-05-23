package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kaungmyathan18/golang-ecommerce-microservice/api-gateway/internal/cache"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/api-gateway/internal/config"
	gwmiddleware "github.com/kaungmyathan18/golang-ecommerce-microservice/api-gateway/internal/middleware"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/api-gateway/internal/observability"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/api-gateway/internal/proxy"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/api-gateway/internal/ratelimit"

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

	cfg := config.DefaultConfig("api-gateway")
	logger, err := observability.NewLogger(cfg.Log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("starting", zap.String("app", cfg.App.Name))

	cacheClient, err := cache.New(cfg.Cache)
	if err != nil {
		logger.Fatal("cache", zap.Error(err))
	}
	defer cacheClient.Close()

	gwProxy, err := proxy.New(cfg.Gateway.ProductServiceURL, cfg.Gateway.OrderServiceURL, cfg.Gateway.InventoryServiceURL, cfg.Gateway.PaymentServiceURL, cfg.Gateway.AuthServiceURL)
	if err != nil {
		logger.Fatal("proxy", zap.Error(err))
	}

	limiter := ratelimit.New(cacheClient.Redis(), cfg.Gateway.RateLimitRPM)
	router := setupRouter(cfg, logger, gwProxy, limiter)

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

func setupRouter(cfg *config.Config, logger *zap.Logger, gw *proxy.Gateway, limiter *ratelimit.Limiter) *chi.Mux {
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
	r.Use(gwmiddleware.AccessLog(logger))
	r.Use(limiter.Middleware)
	r.Use(gwmiddleware.JWTAuth(cfg.Gateway.JWTSecret))

	r.Get("/health/live", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Get("/health/ready", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	r.Handle("/api/v1/*", gw)
	return r
}
