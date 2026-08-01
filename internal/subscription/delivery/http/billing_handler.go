package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/nemesis-project/api-nemesis/internal/infrastructure/middleware"
	"github.com/nemesis-project/api-nemesis/internal/subscription/domain"
	"github.com/nemesis-project/api-nemesis/internal/subscription/usecase"
	"github.com/nemesis-project/api-nemesis/pkg/response"
)

type BillingHandler struct {
	uc domain.BillingUseCase
}

func NewBillingHandler(uc domain.BillingUseCase) *BillingHandler {
	return &BillingHandler{uc: uc}
}

func (h *BillingHandler) CreateCheckout(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)

	var req domain.CheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.PriceID == "" || req.SuccessURL == "" || req.CancelURL == "" {
		response.WriteError(w, http.StatusBadRequest, "price_id, success_url and cancel_url are required")
		return
	}

	result, err := h.uc.CreateCheckoutSession(r.Context(), userID, req)
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidPriceID) {
			response.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		response.WriteError(w, http.StatusInternalServerError, "Failed to create checkout session")
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Checkout session created",
		"data":    result,
	})
}

func (h *BillingHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "Failed to read body")
		return
	}

	signature := r.Header.Get("Stripe-Signature")
	if signature == "" {
		response.WriteError(w, http.StatusBadRequest, "Missing Stripe-Signature header")
		return
	}

	if err := h.uc.HandleWebhook(r.Context(), payload, signature); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]string{
		"status": "success",
	})
}

func (h *BillingHandler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)

	sub, err := h.uc.GetSubscription(r.Context(), userID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "Failed to get subscription")
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Subscription retrieved",
		"data":    sub,
	})
}