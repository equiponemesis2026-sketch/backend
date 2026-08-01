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
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "Missing or invalid user in token")
		return
	}

	normalizedUserID, err := normalizeUserID(userID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Invalid user_id format")
		return
	}

	if input.Platform == "" {
		writeError(w, http.StatusBadRequest, "platform is required")
		return
	}

	pairingCode, err := h.uc.GeneratePairingCode(r.Context(), domain.GenerateCodeRequest{
		UserID:   normalizedUserID,
		Platform: input.Platform,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to generate pairing code")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Pairing code generated",
		"data":    pairingCode,
	})
}

func (h *TokenHandler) PairDevice(w http.ResponseWriter, r *http.Request) {
	var input domain.PairingRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "Missing or invalid user in token")
		return
	}

	normalizedUserID, err := normalizeUserID(userID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Invalid user_id format")
		return
	}

	device, err := h.uc.PairDevice(r.Context(), input, normalizedUserID)
	if err != nil {
		if errors.Is(err, usecase.ErrPairingCodeExpired) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, usecase.ErrPairingCodeNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Device paired successfully",
		"data":    device,
	})
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{
		"status":  "error",
		"message": message,
	})
}