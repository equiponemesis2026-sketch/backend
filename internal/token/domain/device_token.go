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
	Code       string    `json:"code" bson:"code"`
	UserID     string    `json:"user_id" bson:"user_id"`
	ExpiresAt  time.Time `json:"expires_at" bson:"expires_at"`
	CreatedAt  time.Time `json:"created_at" bson:"created_at"`
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

type TokenResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

type DeviceRepository interface {
	FindByPairingCode(ctx context.Context, code string) (*PairingCode, error)
	FindByID(ctx context.Context, id string) (*Device, error)
	Save(ctx context.Context, device *Device) error
	SavePairingCode(ctx context.Context, code *PairingCode) error
}

type TokenUseCase interface {
	GeneratePairingCode(ctx context.Context, input GenerateCodeRequest) (*PairingCode, error)
	PairDevice(ctx context.Context, input PairingRequest, userID string) (*Device, error)
}