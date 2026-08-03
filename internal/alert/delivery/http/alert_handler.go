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

	var input domain.SOSInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	alert, err := h.uc.CreateSOS(r.Context(), victimID, input)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "Failed to create alert")
		return
	}

	response.WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"status":  "success",
		"message": "Alert created and observers notified",
		"data":    alert,
	})
}

// CreateCoercion maneja la entrada de PIN bajo coacción. Responde SIEMPRE
// con el mismo HTTP 200 engañoso cuando el PIN es válido, de modo que un
// agresor no pueda distinguir entre cancelación real y código rojo.
func (h *AlertHandler) CreateCoercion(w http.ResponseWriter, r *http.Request) {
	victimID := r.Context().Value(middleware.UserIDKey).(string)

	var input domain.CoercionInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.WriteJSON(w, http.StatusBadRequest, map[string]string{
			"status":  "error",
			"message": "Invalid request",
		})
		return
	}

	if _, err := h.uc.CreateCoercion(r.Context(), victimID, input); err != nil {
		// Respuesta genérica idéntica para no filtrar información al agresor.
		response.WriteJSON(w, http.StatusBadRequest, map[string]string{
			"status":  "error",
			"message": "Invalid request",
		})
		return
	}

	// Éxito fingido: idéntico para PIN real (resuelto) y PIN de coerción.
	response.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status": "resolved",
	})
}

// Resolve permite a la víctima cancelar explícitamente una alerta activa.
func (h *AlertHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	alertID := chi.URLParam(r, "id")

	alert, err := h.uc.ResolveAlert(r.Context(), alertID, userID)
	if err != nil {
		if errors.Is(err, usecase.ErrAlertNotFound) {
			response.WriteError(w, http.StatusNotFound, err.Error())
			return
		}
		response.WriteError(w, http.StatusInternalServerError, "Failed to resolve alert")
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Alert resolved",
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

// GetOwn lista las alertas propias del usuario autenticado (historial).
func (h *AlertHandler) GetOwn(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)

	alerts, err := h.uc.GetByUserID(r.Context(), userID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "Failed to fetch alerts")
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Alerts retrieved successfully",
		"data":    alerts,
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
