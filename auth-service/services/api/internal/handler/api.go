package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/kaungmyathan18/golang-ecommerce-microservice/auth-service/internal/apiresponse"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/auth-service/internal/repository"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/auth-service/internal/token"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/auth-service/internal/validation"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/auth-service/services/api/internal/service"

	"go.uber.org/zap"
)

type APIHandler struct {
	svc    *service.AuthService
	tokens *token.Issuer
	log    *zap.Logger
}

func NewAPIHandler(svc *service.AuthService, tokens *token.Issuer, log *zap.Logger) *APIHandler {
	return &APIHandler{svc: svc, tokens: tokens, log: log}
}

type registerReq struct {
	Email    string `json:"email" validate:"required,email"`
	Name     string `json:"name" validate:"required,min=2,max=100"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

type loginReq struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

func (h *APIHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := validation.DecodeJSON(r, &req); err != nil {
		validation.WriteError(w, r, err)
		return
	}
	resp, err := h.svc.Register(r.Context(), req.Email, req.Name, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrEmailTaken) {
			apiresponse.WriteProblem(w, r, http.StatusConflict, apiresponse.ProblemTypeURI(r, "email-taken"), "Conflict", "Email already registered.", nil)
			return
		}
		if err.Error() == "invalid input" {
			apiresponse.WriteProblem(w, r, http.StatusBadRequest, apiresponse.ProblemTypeURI(r, "validation-error"), "Bad Request", err.Error(), nil)
			return
		}
		apiresponse.WriteInternalError(w, r, err)
		return
	}
	apiresponse.WriteJSON(w, r, http.StatusCreated, resp, nil, nil)
}

func (h *APIHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := validation.DecodeJSON(r, &req); err != nil {
		validation.WriteError(w, r, err)
		return
	}
	resp, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			apiresponse.WriteProblem(w, r, http.StatusUnauthorized, apiresponse.ProblemTypeURI(r, "invalid-credentials"), "Unauthorized", "Invalid email or password.", nil)
			return
		}
		apiresponse.WriteInternalError(w, r, err)
		return
	}
	apiresponse.WriteJSON(w, r, http.StatusOK, resp, nil, nil)
}

func (h *APIHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims, err := h.parseBearer(r)
	if err != nil {
		apiresponse.WriteProblem(w, r, http.StatusUnauthorized, apiresponse.ProblemTypeURI(r, "unauthorized"), "Unauthorized", "Invalid or missing token.", nil)
		return
	}
	user, err := h.svc.Me(r.Context(), claims.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			apiresponse.WriteProblem(w, r, http.StatusNotFound, apiresponse.ProblemTypeURI(r, "not-found"), "Not Found", "User not found.", nil)
			return
		}
		apiresponse.WriteInternalError(w, r, err)
		return
	}
	apiresponse.WriteJSON(w, r, http.StatusOK, user, nil, nil)
}

func (h *APIHandler) parseBearer(r *http.Request) (*token.Claims, error) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return nil, errors.New("missing bearer token")
	}
	return h.tokens.Parse(strings.TrimPrefix(auth, "Bearer "))
}
