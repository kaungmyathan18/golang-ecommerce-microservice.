package handler

import (
	"errors"
	"net/http"

	"github.com/kaungmyathan18/golang-ecommerce-microservice/payment-service/internal/apiresponse"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/payment-service/internal/repository"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/payment-service/internal/validation"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/payment-service/services/api/internal/service"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type APIHandler struct {
	svc *service.PaymentService
	log *zap.Logger
}

func NewAPIHandler(svc *service.PaymentService, log *zap.Logger) *APIHandler {
	return &APIHandler{svc: svc, log: log}
}

type initiatePaymentReq struct {
	OrderID string  `json:"order_id" validate:"required,uuid"`
	UserID  string  `json:"user_id" validate:"required,uuid"`
	Amount  float64 `json:"amount" validate:"required,gte=0"`
}

func (h *APIHandler) InitiatePayment(w http.ResponseWriter, r *http.Request) {
	var req initiatePaymentReq
	if err := validation.DecodeJSON(r, &req); err != nil {
		validation.WriteError(w, r, err)
		return
	}
	p, err := h.svc.InitiatePayment(r.Context(), req.OrderID, req.UserID, req.Amount)
	if err != nil {
		apiresponse.WriteInternalError(w, r, err)
		return
	}
	apiresponse.WriteJSON(w, r, http.StatusCreated, p, nil, nil)
}

func (h *APIHandler) GetPayment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := validation.Var(id, "required,uuid"); err != nil {
		validation.WriteError(w, r, err)
		return
	}
	p, err := h.svc.GetPayment(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			apiresponse.WriteProblem(w, r, http.StatusNotFound, apiresponse.ProblemTypeURI(r, "not-found"), "Not Found", "Payment not found.", nil)
			return
		}
		apiresponse.WriteInternalError(w, r, err)
		return
	}
	apiresponse.WriteJSON(w, r, http.StatusOK, p, nil, nil)
}
