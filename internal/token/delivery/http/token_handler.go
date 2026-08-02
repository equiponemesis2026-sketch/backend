package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/nemesis-project/api-nemesis/internal/infrastructure/middleware"
	"github.com/nemesis-project/api-nemesis/internal/token/domain"
	"github.com/nemesis-project/api-nemesis/internal/token/usecase"
	"github.com/nemesis-project/api-nemesis/pkg/response"
)

// normalizeUserID valida un user_id aceptando tanto el formato con prefijo
// "usr_" como el UUID puro, devolviendo el ID normalizado (sin prefijo).
func normalizeUserID(raw string) (string, error) {
	id := strings.TrimPrefix(raw, "usr_")
	if _, err := uuid.Parse(id); err != nil {
		return "", err
	}
	return id, nil
}

type TokenHandler struct {
	uc domain.TokenUseCase
}

func NewTokenHandler(uc domain.TokenUseCase) *TokenHandler {
	return &TokenHandler{uc: uc}
}

func (h *TokenHandler) GenerateCode(w http.ResponseWriter, r *http.Request) {
	var input domain.GenerateCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		response.WriteError(w, http.StatusUnauthorized, "Missing or invalid user in token")
		return
	}

	normalizedUserID, err := normalizeUserID(userID)
	if err != nil {
		response.WriteError(w, http.StatusUnauthorized, "Invalid user_id format")
		return
	}

	if input.Platform == "" {
		response.WriteError(w, http.StatusBadRequest, "platform is required")
		return
	}

	pairingCode, err := h.uc.GeneratePairingCode(r.Context(), domain.GenerateCodeRequest{
		UserID:   normalizedUserID,
		Platform: input.Platform,
	})
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "Failed to generate pairing code")
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Pairing code generated",
		"data":    pairingCode,
	})
}

func (h *TokenHandler) PairDevice(w http.ResponseWriter, r *http.Request) {
	var input domain.PairingRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		response.WriteError(w, http.StatusUnauthorized, "Missing or invalid user in token")
		return
	}

	normalizedUserID, err := normalizeUserID(userID)
	if err != nil {
		response.WriteError(w, http.StatusUnauthorized, "Invalid user_id format")
		return
	}

	device, err := h.uc.PairDevice(r.Context(), input, normalizedUserID)
	if err != nil {
		if errors.Is(err, usecase.ErrPairingCodeExpired) {
			response.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, usecase.ErrPairingCodeNotFound) {
			response.WriteError(w, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, usecase.ErrPairingCodeMismatch) {
			response.WriteError(w, http.StatusForbidden, err.Error())
			return
		}
		if errors.Is(err, usecase.ErrDeviceAlreadyPaired) {
			response.WriteError(w, http.StatusConflict, err.Error())
			return
		}
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Device paired successfully",
		"data":    device,
	})
}

func (h *TokenHandler) RegisterFCMToken(w http.ResponseWriter, r *http.Request) {
	var input domain.FCMTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if input.DeviceID == "" || input.FCMToken == "" {
		response.WriteError(w, http.StatusBadRequest, "device_id and fcm_token are required")
		return
	}

	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		response.WriteError(w, http.StatusUnauthorized, "Missing or invalid user in token")
		return
	}

	normalizedUserID, err := normalizeUserID(userID)
	if err != nil {
		response.WriteError(w, http.StatusUnauthorized, "Invalid user_id format")
		return
	}

	if err := h.uc.RegisterFCMToken(r.Context(), input, normalizedUserID); err != nil {
		if errors.Is(err, usecase.ErrDeviceNotFound) {
			response.WriteError(w, http.StatusNotFound, err.Error())
			return
		}
		response.WriteError(w, http.StatusInternalServerError, "Failed to register FCM token")
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "FCM token registered successfully",
	})
}
