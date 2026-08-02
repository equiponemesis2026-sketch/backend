package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/nemesis-project/api-nemesis/internal/infrastructure/middleware"
	"github.com/nemesis-project/api-nemesis/internal/user/domain"
	"github.com/nemesis-project/api-nemesis/internal/user/usecase"
	"github.com/nemesis-project/api-nemesis/pkg/response"
)

// UserHandler expone los endpoints HTTP del módulo de usuarios.
type UserHandler struct {
	uc domain.UserUseCase
}

// NewUserHandler inyecta el caso de uso en el controlador HTTP.
func NewUserHandler(uc domain.UserUseCase) *UserHandler {
	return &UserHandler{uc: uc}
}

// Register maneja POST /api/v1/auth/register
func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	var input domain.RegisterInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if input.Name == "" || input.Email == "" || input.Password == "" || input.Phone == "" {
		response.WriteError(w, http.StatusBadRequest, "All fields (name, email, password, phone) are required")
		return
	}

	user, err := h.uc.Register(r.Context(), input)
	if err != nil {
		if errors.Is(err, usecase.ErrUserAlreadyExists) {
			response.WriteError(w, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, usecase.ErrInvalidInput) {
			response.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		response.WriteError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	response.WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"status":  "success",
		"message": "User registered successfully",
		"data":    user,
	})
}

// Login maneja POST /api/v1/auth/login
func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	var input domain.LoginInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if input.Email == "" || input.Password == "" {
		response.WriteError(w, http.StatusBadRequest, "Email and password are required")
		return
	}

	tokenResp, err := h.uc.Login(r.Context(), input)
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidCredentials) {
			response.WriteError(w, http.StatusUnauthorized, err.Error())
			return
		}
		response.WriteError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Login successful",
		"data":    tokenResp,
	})
}

// SetSecurityPins maneja PUT /api/v1/user/security/pins
func (h *UserHandler) SetSecurityPins(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)

	var input domain.SecurityPinsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.uc.SetSecurityPins(r.Context(), userID, input); err != nil {
		if errors.Is(err, usecase.ErrInvalidInput) {
			response.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		response.WriteError(w, http.StatusInternalServerError, "Failed to set security pins")
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Security pins configured successfully",
	})
}
