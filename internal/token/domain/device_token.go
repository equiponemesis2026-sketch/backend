package domain

import (
	"context"
	"time"
)

type Device struct {
	ID          string    `json:"id" bson:"_id"`
	UserID      string    `json:"user_id" bson:"user_id"`
	PairingCode string    `json:"pairing_code" bson:"pairing_code"`
	Platform    string    `json:"platform" bson:"platform"`
	DeviceModel string    `json:"device_model" bson:"device_model"`
	DeviceOS    string    `json:"device_os" bson:"device_os"`
	Serial      string    `json:"serial_number" bson:"serial_number"`
	FCMToken    string    `json:"fcm_token" bson:"fcm_token"`
	PairedAt    time.Time `json:"paired_at" bson:"paired_at"`
	CreatedAt   time.Time `json:"created_at" bson:"created_at"`
}

type PairingCode struct {
	Code      string    `json:"code" bson:"code"`
	UserID    string    `json:"user_id" bson:"user_id"`
	ExpiresAt time.Time `json:"expires_at" bson:"expires_at"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
}

type PairingRequest struct {
	PairingCode string `json:"pairing_code" binding:"required"`
	DeviceModel string `json:"device_model" binding:"required"`
	DeviceOS    string `json:"device_os" binding:"required"`
	Serial      string `json:"serial_number" binding:"required"`
	Platform    string `json:"platform" binding:"required"`
}

type GenerateCodeRequest struct {
	UserID   string `json:"user_id,omitempty"`
	Platform string `json:"platform" binding:"required"`
}

type FCMTokenRequest struct {
	DeviceID string `json:"device_id" binding:"required"`
	FCMToken string `json:"fcm_token" binding:"required"`
	Platform string `json:"platform"`
}

type TokenResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// SessionToken es el JWT de sesión emitido cuando el emparejamiento por
// código actúa como login (el dispositivo no tiene forma de teclear
// correo/contraseña).
type SessionToken struct {
	AccessToken string
	TokenType   string
	ExpiresIn   int64
}

// TokenIssuer emite un JWT de sesión para un usuario ya identificado por
// otro medio (aquí, el propietario de un código de emparejamiento válido).
// Lo implementa el usecase de usuarios para no duplicar la lógica de firma.
type TokenIssuer interface {
	IssueSessionToken(ctx context.Context, userID string) (*SessionToken, error)
}

// PairDeviceResult agrupa el dispositivo recién vinculado y, si hay un
// TokenIssuer configurado, el JWT de sesión para que la app no tenga que
// pedir correo/contraseña.
type PairDeviceResult struct {
	Device *Device
	Token  *SessionToken
}

type DeviceRepository interface {
	FindByPairingCode(ctx context.Context, code string) (*PairingCode, error)
	FindActivePairingCodeByUserID(ctx context.Context, userID string) (*PairingCode, error)
	FindByID(ctx context.Context, id string) (*Device, error)
	FindByUserID(ctx context.Context, userID string) (*Device, error)
	FindAllDevicesByUserID(ctx context.Context, userID string) ([]*Device, error)
	Save(ctx context.Context, device *Device) error
	SavePairingCode(ctx context.Context, code *PairingCode) error
	DeletePairingCode(ctx context.Context, code string) error
	UpdateFCMToken(ctx context.Context, deviceID string, fcmToken string) error
}

type TokenUseCase interface {
	GeneratePairingCode(ctx context.Context, input GenerateCodeRequest) (*PairingCode, error)
	PairDevice(ctx context.Context, input PairingRequest) (*PairDeviceResult, error)
	RegisterFCMToken(ctx context.Context, input FCMTokenRequest, userID string) error
}
