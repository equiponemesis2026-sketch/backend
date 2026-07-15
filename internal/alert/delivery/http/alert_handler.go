package http

import (
	"github.com/gin-gonic/gin"
	"github.com/nemesis-back/internal/alert/domain"
)

type AlertHandler struct {
	alertUseCase domain.AlertUseCase
}

func NewAlertHandler(r *gin.Engine, uc domain.AlertUseCase) {
	handler := &AlertHandler{
		alertUseCase: uc,
	}

	api := r.Group("/api/v1/alerts")
	{
		api.POST("/trigger", handler.Trigger)
		api.POST("/verify-pin", handler.VerifyPIN)
	}
}

func (h *AlertHandler) Trigger(c *gin.Context) {
	// TODO: Implement HTTP Trigger
}

func (h *AlertHandler) VerifyPIN(c *gin.Context) {
	// TODO: Implement HTTP PIN Verification
}
