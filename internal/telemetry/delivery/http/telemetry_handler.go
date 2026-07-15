package http

import (
	"github.com/gin-gonic/gin"
	"github.com/nemesis-back/internal/telemetry/domain"
)

type TelemetryHandler struct {
	telemetryUseCase domain.TelemetryUseCase
}

func NewTelemetryHandler(r *gin.Engine, uc domain.TelemetryUseCase) {
	handler := &TelemetryHandler{
		telemetryUseCase: uc,
	}

	api := r.Group("/api/v1/telemetry")
	{
		api.POST("/ingest", handler.Ingest)
	}
}

func (h *TelemetryHandler) Ingest(c *gin.Context) {
	// TODO: Implement bulk/single ingestion handling
}
