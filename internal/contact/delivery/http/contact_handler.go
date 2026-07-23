package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/nemesis-project/api-nemesis/internal/contact/domain"
	"github.com/nemesis-project/api-nemesis/internal/contact/usecase"
	"github.com/nemesis-project/api-nemesis/internal/infrastructure/middleware"
)

type ContactHandler struct {
	uc domain.ContactUseCase
}

func NewContactHandler(uc domain.ContactUseCase) *ContactHandler {
	return &ContactHandler{uc: uc}
}

func (h *ContactHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)

	contacts, err := h.uc.GetAll(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to fetch contacts")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Contacts retrieved successfully",
		"data":    contacts,
	})
}

func (h *ContactHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)

	var input domain.CreateContactInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	contact, err := h.uc.Create(r.Context(), userID, input)
	if err != nil {
		if errors.Is(err, usecase.ErrNameRequired) || errors.Is(err, usecase.ErrPhoneRequired) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to create contact")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"status":  "success",
		"message": "Contact created successfully",
		"data":    contact,
	})
}

func (h *ContactHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	contactID := chi.URLParam(r, "id")

	var input domain.UpdateContactInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	contact, err := h.uc.Update(r.Context(), userID, contactID, input)
	if err != nil {
		if errors.Is(err, usecase.ErrContactNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to update contact")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Contact updated successfully",
		"data":    contact,
	})
}

func (h *ContactHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	contactID := chi.URLParam(r, "id")

	err := h.uc.Delete(r.Context(), userID, contactID)
	if err != nil {
		if errors.Is(err, usecase.ErrContactNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to delete contact")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Contact deleted successfully",
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
