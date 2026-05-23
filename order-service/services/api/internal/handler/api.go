package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/kaungmyathan18/golang-ecommerce-microservice/order-service/internal/apiresponse"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/order-service/internal/observability"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/order-service/internal/repository"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/order-service/internal/validation"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/order-service/internal/orderservice"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type APIHandler struct {
	svc     *orderservice.OrderService
	log     *zap.Logger
	metrics *observability.Metrics
}

func NewAPIHandler(svc *orderservice.OrderService, log *zap.Logger, m *observability.Metrics) *APIHandler {
	return &APIHandler{svc: svc, log: log, metrics: m}
}

type createOrderReq struct {
	UserID    string `json:"user_id" validate:"required,uuid"`
	ProductID string `json:"product_id" validate:"required,uuid"`
	Quantity  int    `json:"quantity" validate:"required,gte=1"`
}

func (h *APIHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req createOrderReq
	if err := validation.DecodeJSON(r, &req); err != nil {
		validation.WriteError(w, r, err)
		return
	}
	o, err := h.svc.CreateOrder(r.Context(), req.UserID, req.ProductID, req.Quantity)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			apiresponse.WriteProblem(w, r, http.StatusNotFound, apiresponse.ProblemTypeURI(r, "not-found"), "Not Found", "Product not found.", nil)
			return
		}
		if err.Error() == "insufficient stock" {
			apiresponse.WriteProblem(w, r, http.StatusConflict, apiresponse.ProblemTypeURI(r, "insufficient-stock"), "Conflict", "Insufficient stock.", nil)
			return
		}
		h.log.Warn("create order", zap.Error(err))
		apiresponse.WriteInternalError(w, r, err)
		return
	}
	apiresponse.WriteJSON(w, r, http.StatusCreated, o, nil, nil)
}

func (h *APIHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := validation.Var(id, "required,uuid"); err != nil {
		validation.WriteError(w, r, err)
		return
	}
	o, err := h.svc.GetOrder(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			apiresponse.WriteProblem(w, r, http.StatusNotFound, apiresponse.ProblemTypeURI(r, "not-found"), "Not Found", "Order not found.", nil)
			return
		}
		apiresponse.WriteInternalError(w, r, err)
		return
	}
	apiresponse.WriteJSON(w, r, http.StatusOK, o, nil, nil)
}

func (h *APIHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	page, limit := parsePageLimit(r)
	orders, total, err := h.svc.ListOrders(r.Context(), page, limit)
	if err != nil {
		apiresponse.WriteInternalError(w, r, err)
		return
	}
	offset := (page - 1) * limit
	hasMore := int64(offset+len(orders)) < total
	pagination := &apiresponse.PaginationMeta{Page: page, Limit: limit, Total: total, HasMore: hasMore}
	links := apiresponse.PageLinks(r, page, limit, hasMore)
	apiresponse.WriteJSON(w, r, http.StatusOK, orders, pagination, links)
}

func (h *APIHandler) ListOrdersByUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")
	if err := validation.Var(userID, "required,uuid"); err != nil {
		validation.WriteError(w, r, err)
		return
	}
	orders, err := h.svc.ListOrdersByUser(r.Context(), userID)
	if err != nil {
		apiresponse.WriteInternalError(w, r, err)
		return
	}
	apiresponse.WriteJSON(w, r, http.StatusOK, orders, nil, nil)
}

func (h *APIHandler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := validation.Var(id, "required,uuid"); err != nil {
		validation.WriteError(w, r, err)
		return
	}
	o, err := h.svc.CancelOrder(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			apiresponse.WriteProblem(w, r, http.StatusNotFound, apiresponse.ProblemTypeURI(r, "not-found"), "Not Found", "Order not found.", nil)
			return
		}
		if errors.Is(err, repository.ErrInvalidTransition) {
			apiresponse.WriteProblem(w, r, http.StatusConflict, apiresponse.ProblemTypeURI(r, "invalid-transition"), "Conflict", "Order cannot be cancelled.", nil)
			return
		}
		apiresponse.WriteInternalError(w, r, err)
		return
	}
	apiresponse.WriteJSON(w, r, http.StatusOK, o, nil, nil)
}

func parsePageLimit(r *http.Request) (page, limit int) {
	page, limit = 1, 20
	if v := r.URL.Query().Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			page = n
		}
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	return page, limit
}
