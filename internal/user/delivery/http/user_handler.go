package http

import (
	"github.com/gin-gonic/gin"
	"github.com/nemesis-back/internal/user/domain"
)

type UserHandler struct {
	userUseCase domain.UserUseCase
}

func NewUserHandler(r *gin.Engine, uc domain.UserUseCase) {
	handler := &UserHandler{
		userUseCase: uc,
	}

	api := r.Group("/api/v1/users")
	{
		api.POST("/register", handler.Register)
		api.POST("/login", handler.Login)
	}
}

func (h *UserHandler) Register(c *gin.Context) {
	// TODO: Implement registration
}

func (h *UserHandler) Login(c *gin.Context) {
	// TODO: Implement authentication
}
