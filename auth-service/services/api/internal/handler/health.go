package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/auth-service/internal/database"

	"go.uber.org/zap"
)

type HealthHandler struct {
	log *zap.Logger
	db *database.DB
}

func NewHealthHandler(
	log *zap.Logger,
	db *database.DB,
) *HealthHandler {
	return &HealthHandler{
		log: log,
		db: db,
	}
}

func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	checkCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	checks := map[string]string{"self": "healthy"}
	if h.db != nil {
		if err := h.db.Ping(checkCtx); err != nil {
			checks["database"] = "unhealthy: " + err.Error()
		} else {
			checks["database"] = "healthy"
		}
	}
	_ = checkCtx

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
}
