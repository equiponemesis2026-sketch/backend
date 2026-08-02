package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/nemesis-project/api-nemesis/internal/alert/domain"
	"github.com/nemesis-project/api-nemesis/internal/alert/usecase"
	"github.com/nemesis-project/api-nemesis/internal/infrastructure/middleware"
	"github.com/nemesis-project/api-nemesis/pkg/response"
)

type AlertHandler struct {
	uc domain.AlertUseCase
}

func NewAlertHandler(uc domain.AlertUseCase) *AlertHandler {
	return &AlertHandler{uc: uc}
}

// CreateSOS recibe la activación de la alerta desde el wearable o la app móvil.
func (h *AlertHandler) CreateSOS(w http.ResponseWriter, r *http.Request) {
	victimID := r.Context().Value(middleware.UserIDKey).(string)

	var input domain.CreateAlertInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	alert, err := h.uc.CreateAlert(r.Context(), victimID, input)
	if err != nil {
		if errors.Is(err, usecase.ErrAlertTypeInvalid) {
			response.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		response.WriteError(w, http.StatusInternalServerError, "Failed to create alert")
		return
	}

	response.WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"status":  "success",
		"message": "Alert created and observers notified",
		"data":    alert,
	})
}

// GetByID devuelve los detalles de una alerta para la víctima u observador.
func (h *AlertHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	viewerID := r.Context().Value(middleware.UserIDKey).(string)
	alertID := chi.URLParam(r, "id")

	alert, err := h.uc.GetByID(r.Context(), alertID, viewerID)
	if err != nil {
		if errors.Is(err, usecase.ErrAlertNotFound) {
			response.WriteError(w, http.StatusNotFound, err.Error())
			return
		}
		response.WriteError(w, http.StatusInternalServerError, "Failed to fetch alert")
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Alert retrieved successfully",
		"data":    alert,
	})
}

// GetObserving lista las emergencias activas de las víctimas vinculadas al observador.
func (h *AlertHandler) GetObserving(w http.ResponseWriter, r *http.Request) {
	observerID := r.Context().Value(middleware.UserIDKey).(string)

	alerts, err := h.uc.GetObserving(r.Context(), observerID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "Failed to fetch observing alerts")
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Observing alerts retrieved successfully",
		"data":    alerts,
	})
}
