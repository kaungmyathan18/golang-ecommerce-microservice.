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
	svc     *service.CatalogService
	log     *zap.Logger
	metrics *observability.Metrics
}

func NewAPIHandler(svc *service.CatalogService, log *zap.Logger, m *observability.Metrics) *APIHandler {
	return &APIHandler{svc: svc, log: log, metrics: m}
}

type createCategoryReq struct {
	Name string `json:"name" validate:"required,min=1,max=200"`
}

type updateCategoryReq struct {
	Name string `json:"name" validate:"required,min=1,max=200"`
}

type createProductReq struct {
	Name        string  `json:"name" validate:"required,min=1,max=200"`
	Description string  `json:"description" validate:"max=2000"`
	Price       float64 `json:"price" validate:"required,gte=0"`
	CategoryID  string  `json:"category_id" validate:"required,uuid"`
	Stock       int     `json:"stock" validate:"gte=0"`
}

type updateProductReq struct {
	Name        string  `json:"name" validate:"required,min=1,max=200"`
	Description string  `json:"description" validate:"max=2000"`
	Price       float64 `json:"price" validate:"required,gte=0"`
	CategoryID  string  `json:"category_id" validate:"required,uuid"`
	Stock       int     `json:"stock" validate:"gte=0"`
}

func (h *APIHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var req createCategoryReq
	if err := validation.DecodeJSON(r, &req); err != nil {
		validation.WriteError(w, r, err)
		return
	}
	c, err := h.svc.CreateCategory(r.Context(), req.Name)
	if err != nil {
		if errors.Is(err, repository.ErrDuplicateName) {
			apiresponse.WriteProblem(w, r, http.StatusConflict, apiresponse.ProblemTypeURI(r, "duplicate-name"), "Conflict", "Category name already exists.", nil)
			return
		}
		h.log.Warn("create category", zap.Error(err))
		apiresponse.WriteInternalError(w, r, err)
		return
	}
	apiresponse.WriteJSON(w, r, http.StatusCreated, c, nil, nil)
}

func (h *APIHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := h.svc.ListCategories(r.Context())
	if err != nil {
		apiresponse.WriteInternalError(w, r, err)
		return
	}
	apiresponse.WriteJSON(w, r, http.StatusOK, cats, nil, nil)
}

func (h *APIHandler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := validation.Var(id, "required,uuid"); err != nil {
		validation.WriteError(w, r, err)
		return
	}
	var req updateCategoryReq
	if err := validation.DecodeJSON(r, &req); err != nil {
		validation.WriteError(w, r, err)
		return
	}
	c, err := h.svc.UpdateCategory(r.Context(), id, req.Name)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			apiresponse.WriteProblem(w, r, http.StatusNotFound, apiresponse.ProblemTypeURI(r, "not-found"), "Not Found", "Category not found.", nil)
			return
		}
		if errors.Is(err, repository.ErrDuplicateName) {
			apiresponse.WriteProblem(w, r, http.StatusConflict, apiresponse.ProblemTypeURI(r, "duplicate-name"), "Conflict", "Category name already exists.", nil)
			return
		}
		apiresponse.WriteInternalError(w, r, err)
		return
	}
	apiresponse.WriteJSON(w, r, http.StatusOK, c, nil, nil)
}

func (h *APIHandler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := validation.Var(id, "required,uuid"); err != nil {
		validation.WriteError(w, r, err)
		return
	}
	if err := h.svc.DeleteCategory(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			apiresponse.WriteProblem(w, r, http.StatusNotFound, apiresponse.ProblemTypeURI(r, "not-found"), "Not Found", "Category not found.", nil)
			return
		}
		if errors.Is(err, repository.ErrCategoryInUse) {
			apiresponse.WriteProblem(w, r, http.StatusConflict, apiresponse.ProblemTypeURI(r, "category-in-use"), "Conflict", "Category has products.", nil)
			return
		}
		apiresponse.WriteInternalError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *APIHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var req createProductReq
	if err := validation.DecodeJSON(r, &req); err != nil {
		validation.WriteError(w, r, err)
		return
	}
	p, err := h.svc.CreateProduct(r.Context(), req.Name, req.Description, req.Price, req.CategoryID, req.Stock)
	if err != nil {
		h.log.Warn("create product", zap.Error(err))
		apiresponse.WriteInternalError(w, r, err)
		return
	}
	apiresponse.WriteJSON(w, r, http.StatusCreated, p, nil, nil)
}

func (h *APIHandler) GetProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := validation.Var(id, "required,uuid"); err != nil {
		validation.WriteError(w, r, err)
		return
	}
	p, err := h.svc.GetProduct(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			apiresponse.WriteProblem(w, r, http.StatusNotFound, apiresponse.ProblemTypeURI(r, "not-found"), "Not Found", "Product not found.", nil)
			return
		}
		apiresponse.WriteInternalError(w, r, err)
		return
	}
	apiresponse.WriteJSON(w, r, http.StatusOK, p, nil, nil)
}

func (h *APIHandler) ListProducts(w http.ResponseWriter, r *http.Request) {
	page, limit := parsePageLimit(r)
	products, total, err := h.svc.ListProducts(r.Context(), page, limit)
	if err != nil {
		apiresponse.WriteInternalError(w, r, err)
		return
	}
	offset := (page - 1) * limit
	hasMore := int64(offset+len(products)) < total
	pagination := &apiresponse.PaginationMeta{Page: page, Limit: limit, Total: total, HasMore: hasMore}
	links := apiresponse.PageLinks(r, page, limit, hasMore)
	apiresponse.WriteJSON(w, r, http.StatusOK, products, pagination, links)
}

func (h *APIHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := validation.Var(id, "required,uuid"); err != nil {
		validation.WriteError(w, r, err)
		return
	}
	var req updateProductReq
	if err := validation.DecodeJSON(r, &req); err != nil {
		validation.WriteError(w, r, err)
		return
	}
	p, err := h.svc.UpdateProduct(r.Context(), id, req.Name, req.Description, req.Price, req.CategoryID, req.Stock)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			apiresponse.WriteProblem(w, r, http.StatusNotFound, apiresponse.ProblemTypeURI(r, "not-found"), "Not Found", "Product not found.", nil)
			return
		}
		apiresponse.WriteInternalError(w, r, err)
		return
	}
	apiresponse.WriteJSON(w, r, http.StatusOK, p, nil, nil)
}

func (h *APIHandler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := validation.Var(id, "required,uuid"); err != nil {
		validation.WriteError(w, r, err)
		return
	}
	if err := h.svc.DeleteProduct(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			apiresponse.WriteProblem(w, r, http.StatusNotFound, apiresponse.ProblemTypeURI(r, "not-found"), "Not Found", "Product not found.", nil)
			return
		}
		apiresponse.WriteInternalError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
