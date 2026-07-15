package ws

import (
	"github.com/gin-gonic/gin"
	"github.com/nemesis-back/internal/alert/domain"
)

type AlertWebSocketHandler struct {
	alertUseCase domain.AlertUseCase
}

func NewAlertWebSocketHandler(r *gin.Engine, uc domain.AlertUseCase) {
	handler := &AlertWebSocketHandler{
		alertUseCase: uc,
	}

	r.GET("/ws/alerts", handler.HandleWS)
}

func (h *AlertWebSocketHandler) HandleWS(c *gin.Context) {
	// TODO: Implement WebSocket Hub upgrade and stream
}
