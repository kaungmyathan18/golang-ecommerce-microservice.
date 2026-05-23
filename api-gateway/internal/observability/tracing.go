package observability

import (
	"context"
	"net/http"

	"github.com/kaungmyathan18/golang-ecommerce-microservice/api-gateway/internal/config"
)

// TracerShutdown stops exporters on exit.
type TracerShutdown interface {
	Shutdown(context.Context) error
}

type noopTP struct{}

func (noopTP) Shutdown(context.Context) error { return nil }

func NewTracerProvider(cfg config.OtelConfig) (TracerShutdown, error) {
	return noopTP{}, nil
}

func TracingMiddleware(next http.Handler) http.Handler {
	return next
}
