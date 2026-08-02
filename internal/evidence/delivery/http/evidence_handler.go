package http

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/nemesis-project/api-nemesis/internal/evidence/domain"
	"github.com/nemesis-project/api-nemesis/internal/evidence/usecase"
	"github.com/nemesis-project/api-nemesis/internal/infrastructure/middleware"
	"github.com/nemesis-project/api-nemesis/pkg/response"
)

// EvidenceHandler expone el expediente forense post-incidente.
type EvidenceHandler struct {
	uc domain.EvidenceUseCase
}

func NewEvidenceHandler(uc domain.EvidenceUseCase) *EvidenceHandler {
	return &EvidenceHandler{uc: uc}
}

// GetReport devuelve el expediente forense de una alerta.
func (h *EvidenceHandler) GetReport(w http.ResponseWriter, r *http.Request) {
	viewerID := r.Context().Value(middleware.UserIDKey).(string)
	alertID := chi.URLParam(r, "alert_id")

	report, err := h.uc.Generate(r.Context(), alertID, viewerID)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrReportNotFound):
			response.WriteError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, usecase.ErrReportForbidden):
			response.WriteError(w, http.StatusForbidden, err.Error())
		default:
			response.WriteError(w, http.StatusInternalServerError, "Failed to generate evidence report")
		}
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Evidence report generated",
		"data":    report,
	})
}
