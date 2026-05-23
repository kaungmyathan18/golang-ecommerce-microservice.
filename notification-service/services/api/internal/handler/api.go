package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/kaungmyathan18/golang-ecommerce-microservice/notification-service/internal/apiresponse"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/notification-service/internal/repository"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/notification-service/internal/observability"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/notification-service/internal/validation"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/notification-service/services/api/internal/service"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type APIHandler struct {
	svc     *service.UserService
	log     *zap.Logger
	metrics *observability.Metrics
}

func NewAPIHandler(svc *service.UserService, log *zap.Logger, m *observability.Metrics) *APIHandler {
	return &APIHandler{svc: svc, log: log, metrics: m}
}

type createUserReq struct {
	Email string `json:"email" validate:"required,email,max=320"`
	Name  string `json:"name" validate:"required,min=1,max=200"`
}

func (h *APIHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserReq
	if err := validation.DecodeJSON(r, &req); err != nil {
		validation.WriteError(w, r, err)
		return
	}
	u, err := h.svc.CreateUser(r.Context(), req.Email, req.Name)
	if err != nil {
		if errors.Is(err, repository.ErrDuplicateEmail) {
			apiresponse.WriteProblem(w, r, http.StatusConflict,
				apiresponse.ProblemTypeURI(r, "duplicate-email"),
				"Conflict",
				"A user with this email already exists.",
				nil,
			)
			return
		}
		h.log.Warn("create user", zap.Error(err))
		apiresponse.WriteInternalError(w, r, err)
		return
	}
	apiresponse.WriteJSON(w, r, http.StatusCreated, u, nil, nil)
}

func (h *APIHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := validation.Var(id, "required,uuid"); err != nil {
		validation.WriteError(w, r, err)
		return
	}
	u, err := h.svc.GetUser(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			apiresponse.WriteProblem(w, r, http.StatusNotFound,
				apiresponse.ProblemTypeURI(r, "not-found"),
				"Not Found",
				"No user exists for the given id.",
				nil,
			)
			return
		}
		apiresponse.WriteInternalError(w, r, err)
		return
	}
	apiresponse.WriteJSON(w, r, http.StatusOK, u, nil, nil)
}

func (h *APIHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	page, limit := parsePageLimit(r)
	lq := struct {
		Page  int `validate:"gte=1"`
		Limit int `validate:"gte=1,lte=100"`
	}{Page: page, Limit: limit}
	if err := validation.V.Struct(lq); err != nil {
		validation.WriteError(w, r, err)
		return
	}
	users, total, err := h.svc.ListUsersPaged(r.Context(), page, limit)
	if err != nil {
		h.log.Error("list users", zap.Error(err))
		apiresponse.WriteInternalError(w, r, err)
		return
	}
	offset := (page - 1) * limit
	hasMore := int64(offset+len(users)) < total
	pagination := &apiresponse.PaginationMeta{
		Page:    page,
		Limit:   limit,
		Total:   total,
		HasMore: hasMore,
	}
	links := apiresponse.PageLinks(r, page, limit, hasMore)
	apiresponse.WriteJSON(w, r, http.StatusOK, users, pagination, links)
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
