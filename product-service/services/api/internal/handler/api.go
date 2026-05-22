package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/internal/apiresponse"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/internal/observability"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/internal/repository"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/internal/validation"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/product-service/services/api/internal/service"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type APIHandler struct {
	svc     *service.ProductService
	log     *zap.Logger
	metrics *observability.Metrics
}

func NewAPIHandler(svc *service.ProductService, log *zap.Logger, m *observability.Metrics) *APIHandler {
	return &APIHandler{svc: svc, log: log, metrics: m}
}

type createProductReq struct {
	Title       string `json:"title" validate:"required,min=1,max=200"`
	Description string `json:"description" validate:"required,min=1,max=1000"`
	Name        string `json:"name" validate:"required,min=1,max=200"`
}

func (h *APIHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var req createProductReq
	if err := validation.DecodeJSON(r, &req); err != nil {
		validation.WriteError(w, r, err)
		return
	}
	u, err := h.svc.CreateProduct(r.Context(), req.Title, req.Description, req.Name)
	if err != nil {
		if errors.Is(err, repository.ErrDuplicateTitle) {
			apiresponse.WriteProblem(w, r, http.StatusConflict,
				apiresponse.ProblemTypeURI(r, "duplicate-title"),
				"Conflict",
				"A product with this title already exists.",
				nil,
			)
			return
		}
		h.log.Warn("create product", zap.Error(err))
		apiresponse.WriteInternalError(w, r, err)
		return
	}
	apiresponse.WriteJSON(w, r, http.StatusCreated, u, nil, nil)
}

func (h *APIHandler) GetProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := validation.Var(id, "required,uuid"); err != nil {
		validation.WriteError(w, r, err)
		return
	}
	u, err := h.svc.GetProduct(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			apiresponse.WriteProblem(w, r, http.StatusNotFound,
				apiresponse.ProblemTypeURI(r, "not-found"),
				"Not Found",
				"No product exists for the given id.",
				nil,
			)
			return
		}
		apiresponse.WriteInternalError(w, r, err)
		return
	}
	apiresponse.WriteJSON(w, r, http.StatusOK, u, nil, nil)
}

func (h *APIHandler) ListProducts(w http.ResponseWriter, r *http.Request) {
	page, limit := parsePageLimit(r)
	lq := struct {
		Page  int `validate:"gte=1"`
		Limit int `validate:"gte=1,lte=100"`
	}{Page: page, Limit: limit}
	if err := validation.V.Struct(lq); err != nil {
		validation.WriteError(w, r, err)
		return
	}
	products, total, err := h.svc.ListProductsPaged(r.Context(), page, limit)
	if err != nil {
		h.log.Error("list products", zap.Error(err))
		apiresponse.WriteInternalError(w, r, err)
		return
	}
	offset := (page - 1) * limit
	hasMore := int64(offset+len(products)) < total
	pagination := &apiresponse.PaginationMeta{
		Page:    page,
		Limit:   limit,
		Total:   total,
		HasMore: hasMore,
	}
	links := apiresponse.PageLinks(r, page, limit, hasMore)
	apiresponse.WriteJSON(w, r, http.StatusOK, products, pagination, links)
}

func parsePageLimit(r *http.Request) (page, limit int) {
	page = 1
	limit = 20
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
