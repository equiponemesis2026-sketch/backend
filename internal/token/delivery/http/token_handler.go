package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/nemesis-project/api-nemesis/internal/infrastructure/middleware"
	"github.com/nemesis-project/api-nemesis/internal/token/domain"
	"github.com/nemesis-project/api-nemesis/internal/token/usecase"
)

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

	if input.UserID == "" || input.Platform == "" {
		writeError(w, http.StatusBadRequest, "user_id and platform are required")
		return
	}

	if _, err := uuid.Parse(input.UserID); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid user_id format")
		return
	}

	pairingCode, err := h.uc.GeneratePairingCode(r.Context(), input)
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

	if _, err := uuid.Parse(userID); err != nil {
		writeError(w, http.StatusUnauthorized, "Invalid user_id format")
		return
	}

	device, err := h.uc.PairDevice(r.Context(), input, userID)
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