package domain

import (
	"context"
	"time"
)

// Role describe el rol inicial del usuario en Némesis.
type Role string

const (
	RoleVictim   Role = "victim"
	RoleObserver Role = "observer"
)

// IsValid devuelve true si el rol es uno de los permitidos.
func (r Role) IsValid() bool {
	return r == RoleVictim || r == RoleObserver
}

// User representa la entidad de usuario en el sistema Némesis.
// Mantiene tags bson para MongoDB y json para serialización externa.
type User struct {
	ID              string    `json:"user_id" bson:"_id"`
	Name            string    `json:"name" bson:"name"`
	Email           string    `json:"email" bson:"email"`
	Password        string    `json:"password,omitempty" bson:"password"` // omitempty por seguridad
	Phone           string    `json:"phone" bson:"phone"`
	Role            Role      `json:"role" bson:"role"`
	RealPINHash     string    `json:"-" bson:"real_pin_hash,omitempty"`
	CoercionPINHash string    `json:"-" bson:"coercion_pin_hash,omitempty"`
	CreatedAt       time.Time `json:"created_at" bson:"created_at"`
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
	Role  Role   `json:"role"`
}

// RegisterInput encapsula los datos requeridos para registrar un usuario.
// Role es opcional: si se omite, se asigna RoleVictim por defecto.
type RegisterInput struct {
	Name     string `json:"name" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	Phone    string `json:"phone" validate:"required"`
	Role     Role   `json:"role,omitempty"`
}

// LoginInput encapsula los datos requeridos para el inicio de sesión.
type LoginInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// SecurityPinsInput encapsula los PINs de seguridad (real y de coerción).
// Ambos deben tener entre 4 y 6 dígitos y ser distintos entre sí.
type SecurityPinsInput struct {
	RealPIN     string `json:"real_pin" validate:"required,len=4,min=4,max=6,nefield=CoercionPIN"`
	CoercionPIN string `json:"coercion_pin" validate:"required,len=4,min=4,max=6"`
}

// PinMatch describe el resultado de verificar un PIN contra los hashes del usuario.
type PinMatch int

const (
	PinNone     PinMatch = iota // no coincide con ninguno
	PinReal                     // PIN real: cancelar/desactivar alerta
	PinCoercion                 // PIN de coerción: modo código rojo silencioso
)

// UserRepository define el contrato de persistencia para el módulo de usuarios.
type UserRepository interface {
	Create(ctx context.Context, user *User) error
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByPhone(ctx context.Context, phone string) (*User, error)
	FindByID(ctx context.Context, id string) (*User, error)
	UpdateSecurityPins(ctx context.Context, userID string, realPINHash string, coercionPINHash string) error
}

// UserUseCase define la lógica de negocio para registro y autenticación.
type UserUseCase interface {
	Register(ctx context.Context, input RegisterInput) (*User, error)
	Login(ctx context.Context, input LoginInput) (*TokenResponse, error)
	SetSecurityPins(ctx context.Context, userID string, input SecurityPinsInput) error
	VerifyPin(ctx context.Context, userID string, pin string) (PinMatch, error)
}
