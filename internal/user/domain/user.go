package domain

import (
	"context"
	"time"
)

// User representa la entidad de usuario en el sistema Némesis.
// Mantiene tags bson para MongoDB y json para serialización externa.
type User struct {
	ID        string    `json:"user_id" bson:"_id"`
	Name      string    `json:"name" bson:"name"`
	Email     string    `json:"email" bson:"email"`
	Password  string    `json:"password,omitempty" bson:"password"` // omitempty por seguridad
	Phone     string    `json:"phone" bson:"phone"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
}

// TokenResponse representa el payload de respuesta en una autenticación exitosa.
type TokenResponse struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresIn   int64     `json:"expires_in"` // Duración en segundos
	User        UserBasic `json:"user"`
}

// UserBasic contiene información pública no sensible del usuario.
type UserBasic struct {
	ID    string `json:"user_id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// RegisterInput encapsula los datos requeridos para registrar un usuario.
type RegisterInput struct {
	Name     string `json:"name" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	Phone    string `json:"phone" validate:"required"`
}

// LoginInput encapsula los datos requeridos para el inicio de sesión.
type LoginInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// UserRepository define el contrato de persistencia para el módulo de usuarios.
type UserRepository interface {
	Create(ctx context.Context, user *User) error
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByID(ctx context.Context, id string) (*User, error)
}

// UserUseCase define la lógica de negocio para registro y autenticación.
type UserUseCase interface {
	Register(ctx context.Context, input RegisterInput) (*User, error)
	Login(ctx context.Context, input LoginInput) (*TokenResponse, error)
}
