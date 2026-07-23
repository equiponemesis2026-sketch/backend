package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/nemesis-project/api-nemesis/internal/user/domain"
	"github.com/nemesis-project/api-nemesis/internal/user/usecase"
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
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if input.Name == "" || input.Email == "" || input.Password == "" || input.Phone == "" {
		writeError(w, http.StatusBadRequest, "All fields (name, email, password, phone) are required")
		return
	}

	user, err := h.uc.Register(r.Context(), input)
	if err != nil {
		if errors.Is(err, usecase.ErrUserAlreadyExists) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"status":  "success",
		"message": "User registered successfully",
		"data":    user,
	})
}

// Login maneja POST /api/v1/auth/login
func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	var input domain.LoginInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if input.Email == "" || input.Password == "" {
		writeError(w, http.StatusBadRequest, "Email and password are required")
		return
	}

	tokenResp, err := h.uc.Login(r.Context(), input)
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Login successful",
		"data":    tokenResp,
	})
}

// writeJSON serializa la respuesta como JSON y establece los encabezados.
func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// writeError envía un mensaje de error como JSON.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{
		"status":  "error",
		"message": message,
	})
}
