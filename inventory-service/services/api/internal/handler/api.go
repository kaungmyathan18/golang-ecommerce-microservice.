package handler

import (
	"errors"
	"net/http"

	"github.com/kaungmyathan18/golang-ecommerce-microservice/inventory-service/internal/apiresponse"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/inventory-service/internal/repository"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/inventory-service/internal/validation"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/inventory-service/services/api/internal/service"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type APIHandler struct {
	svc *service.InventoryService
	log *zap.Logger
}

func NewAPIHandler(svc *service.InventoryService, log *zap.Logger) *APIHandler {
	return &APIHandler{svc: svc, log: log}
}

type setStockReq struct {
	Quantity int `json:"quantity" validate:"gte=0"`
}

func (h *APIHandler) GetStock(w http.ResponseWriter, r *http.Request) {
	productID := chi.URLParam(r, "product_id")
	if err := validation.Var(productID, "required,uuid"); err != nil {
		validation.WriteError(w, r, err)
		return
	}
	qty, err := h.svc.GetStock(r.Context(), productID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			apiresponse.WriteProblem(w, r, http.StatusNotFound, apiresponse.ProblemTypeURI(r, "not-found"), "Not Found", "Inventory not found.", nil)
			return
		}
		apiresponse.WriteInternalError(w, r, err)
		return
	}
	apiresponse.WriteJSON(w, r, http.StatusOK, map[string]any{"product_id": productID, "quantity": qty}, nil, nil)
}

func (h *APIHandler) SetStock(w http.ResponseWriter, r *http.Request) {
	productID := chi.URLParam(r, "product_id")
	if err := validation.Var(productID, "required,uuid"); err != nil {
		validation.WriteError(w, r, err)
		return
	}
	var req setStockReq
	if err := validation.DecodeJSON(r, &req); err != nil {
		validation.WriteError(w, r, err)
		return
	}
	qty, err := h.svc.SetStock(r.Context(), productID, req.Quantity)
	if err != nil {
		apiresponse.WriteInternalError(w, r, err)
		return
	}
	apiresponse.WriteJSON(w, r, http.StatusOK, map[string]any{"product_id": productID, "quantity": qty}, nil, nil)
}
