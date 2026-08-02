package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/nemesis-project/api-nemesis/internal/contact/domain"
	"github.com/nemesis-project/api-nemesis/internal/contact/usecase"
	"github.com/nemesis-project/api-nemesis/internal/infrastructure/middleware"
	"github.com/nemesis-project/api-nemesis/pkg/response"
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
		response.WriteError(w, http.StatusInternalServerError, "Failed to fetch contacts")
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Contacts retrieved successfully",
		"data":    contacts,
	})
}

func (h *ContactHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)

	var input domain.CreateContactInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	contact, err := h.uc.Create(r.Context(), userID, input)
	if err != nil {
		if errors.Is(err, usecase.ErrNameRequired) || errors.Is(err, usecase.ErrPhoneRequired) {
			response.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		response.WriteError(w, http.StatusInternalServerError, "Failed to create contact")
		return
	}

	response.WriteJSON(w, http.StatusCreated, map[string]interface{}{
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
		response.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	contact, err := h.uc.Update(r.Context(), userID, contactID, input)
	if err != nil {
		if errors.Is(err, usecase.ErrContactNotFound) {
			response.WriteError(w, http.StatusNotFound, err.Error())
			return
		}
		response.WriteError(w, http.StatusInternalServerError, "Failed to update contact")
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]interface{}{
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
			response.WriteError(w, http.StatusNotFound, err.Error())
			return
		}
		response.WriteError(w, http.StatusInternalServerError, "Failed to delete contact")
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Contact deleted successfully",
	})
}

func (h *ContactHandler) Link(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	contactID := chi.URLParam(r, "id")

	var input struct {
		LinkedUserID string `json:"linked_user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if input.LinkedUserID == "" {
		response.WriteError(w, http.StatusBadRequest, "linked_user_id is required")
		return
	}

	contact, err := h.uc.Link(r.Context(), userID, contactID, input.LinkedUserID)
	if err != nil {
		if errors.Is(err, usecase.ErrContactNotFound) {
			response.WriteError(w, http.StatusNotFound, err.Error())
			return
		}
		response.WriteError(w, http.StatusInternalServerError, "Failed to link contact")
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Contact linked successfully",
		"data":    contact,
	})
}

// GetPending lista las solicitudes de vínculo pendientes de aceptación del
// usuario observador autenticado.
func (h *ContactHandler) GetPending(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)

	contacts, err := h.uc.GetPending(r.Context(), userID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "Failed to fetch pending contacts")
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Pending contacts retrieved successfully",
		"data":    contacts,
	})
}

// AcceptLink permite al usuario vinculado aceptar la solicitud y convertirse
// en observador verificado.
func (h *ContactHandler) AcceptLink(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	contactID := chi.URLParam(r, "id")

	contact, err := h.uc.AcceptLink(r.Context(), contactID, userID)
	if err != nil {
		if errors.Is(err, usecase.ErrContactNotFound) {
			response.WriteError(w, http.StatusNotFound, err.Error())
			return
		}
		response.WriteError(w, http.StatusInternalServerError, "Failed to accept contact")
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Contact request accepted",
		"data":    contact,
	})
}

// RejectLink permite al usuario vinculado rechazar la solicitud y desvincularse
// sin eliminar el contacto de la víctima.
func (h *ContactHandler) RejectLink(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	contactID := chi.URLParam(r, "id")

	if err := h.uc.RejectLink(r.Context(), contactID, userID); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "Failed to reject contact")
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Contact request rejected",
	})
}
