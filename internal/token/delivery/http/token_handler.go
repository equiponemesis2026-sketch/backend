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
		if errors.Is(err, usecase.ErrPairingCodeAlreadyActive) {
			response.WriteError(w, http.StatusConflict, err.Error())
			return
		}
		response.WriteError(w, http.StatusInternalServerError, "Failed to generate pairing code")
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Pairing code generated",
		"data":    pairingCode,
	})
}

// PairDevice registra un dispositivo a partir de un código de emparejamiento
// válido. Es una ruta pública: el código actúa como credencial de login
// (equivalente a vincular una Smart TV) para dispositivos que no pueden
// teclear correo/contraseña. Si hay un emisor de JWT configurado, la
// respuesta incluye access_token para que la app quede autenticada de una vez.
func (h *TokenHandler) PairDevice(w http.ResponseWriter, r *http.Request) {
	var input domain.PairingRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if input.PairingCode == "" {
		response.WriteError(w, http.StatusBadRequest, "pairing_code is required")
		return
	}

	result, err := h.uc.PairDevice(r.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrPairingCodeExpired):
			response.WriteError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, usecase.ErrPairingCodeNotFound):
			response.WriteError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, usecase.ErrDeviceAlreadyPaired):
			response.WriteError(w, http.StatusConflict, err.Error())
		default:
			response.WriteError(w, http.StatusInternalServerError, "Failed to pair device")
		}
		return
	}

	data := map[string]interface{}{"device": result.Device}
	if result.Token != nil {
		data["access_token"] = result.Token.AccessToken
		data["token_type"] = result.Token.TokenType
		data["expires_in"] = result.Token.ExpiresIn
	}

	response.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Device paired successfully",
		"data":    data,
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
