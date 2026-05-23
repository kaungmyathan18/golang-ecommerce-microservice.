package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
	"go.mongodb.org/mongo-driver/mongo"

	"go.uber.org/zap"
)

type HealthHandler struct {
	log *zap.Logger
	mongo *mongo.Client
}

func NewHealthHandler(
	log *zap.Logger,
	mongo *mongo.Client,
) *HealthHandler {
	return &HealthHandler{
		log: log,
		mongo: mongo,
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
	if h.mongo != nil {
		if err := h.mongo.Ping(checkCtx, nil); err != nil {
			checks["mongo"] = "unhealthy: " + err.Error()
		} else {
			checks["mongo"] = "healthy"
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
