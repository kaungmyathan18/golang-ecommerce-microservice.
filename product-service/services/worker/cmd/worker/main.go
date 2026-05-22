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

	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/internal/config"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/internal/database"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/internal/queue"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/internal/observability"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/services/worker/internal/processor"

	"go.uber.org/zap"
)

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

	logger.Info("starting worker", zap.String("app", cfg.App.Name))
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

	ctx := context.Background()
	db, err := database.New(ctx, cfg.Database)
	if err != nil {
		logger.Fatal("database", zap.Error(err))
	}
	defer func() { _ = db.Close() }()

	queueClient, err := queue.New(cfg.Queue)
	if err != nil {
		logger.Fatal("queue", zap.Error(err))
	}
	defer func() { _ = queueClient.Close() }()

	proc := processor.New(cfg.Worker, queueClient, logger)

	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()
	proc.Start(workerCtx)

	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/health/live", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	healthMux.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		checks := map[string]string{"self": "healthy"}
		checkCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := queueClient.Ping(checkCtx); err != nil {
			checks["queue"] = "unhealthy: " + err.Error()
		} else {
			checks["queue"] = "healthy"
		}
		status := http.StatusOK
		for _, v := range checks {
			if len(v) >= 9 && v[:9] == "unhealthy" {
				status = http.StatusServiceUnavailable
				break
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(checks)
	})
	healthSrv := &http.Server{
		Addr:              ":8081",
		Handler:           healthMux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		logger.Info("health endpoint", zap.String("addr", healthSrv.Addr))
		if err := healthSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("health server", zap.Error(err))
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	logger.Info("shutting down")
	workerCancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = healthSrv.Shutdown(shutdownCtx)
}
