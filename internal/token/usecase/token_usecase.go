package usecase

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"

	"github.com/nemesis-project/api-nemesis/internal/token/domain"
)

var (
	ErrPairingCodeExpired  = errors.New("pairing code has expired")
	ErrPairingCodeNotFound = errors.New("pairing code not found")
	ErrPairingCodeMismatch = errors.New("pairing code does not belong to this user")
	ErrDeviceAlreadyPaired = errors.New("device is already paired with another user")
	ErrDeviceNotFound      = errors.New("device not found")
)

type tokenUseCase struct {
	repo domain.DeviceRepository
}

func NewTokenUseCase(repo domain.DeviceRepository) domain.TokenUseCase {
	return &tokenUseCase{repo: repo}
}

func (t *tokenUseCase) GeneratePairingCode(ctx context.Context, input domain.GenerateCodeRequest) (*domain.PairingCode, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	code := "NMS-"
	for i := 0; i < 3; i++ {
		if i > 0 {
			code += "-"
		}
		for j := 0; j < 3; j++ {
			n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
			if err != nil {
				return nil, fmt.Errorf("failed to generate secure random: %w", err)
			}
			code += string(charset[n.Int64()])
		}
	}
	expiresAt := time.Now().Add(5 * time.Minute).UTC()
	pairingCode := &domain.PairingCode{
		Code:      code,
		UserID:    input.UserID,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().UTC(),
	}

	if err := t.repo.SavePairingCode(ctx, pairingCode); err != nil {
		return nil, fmt.Errorf("failed to store pairing code: %w", err)
	}

	return pairingCode, nil
}

func (t *tokenUseCase) PairDevice(ctx context.Context, input domain.PairingRequest, userID string) (*domain.Device, error) {
	pairingCode, err := t.repo.FindByPairingCode(ctx, input.PairingCode)
	if err != nil {
		return nil, fmt.Errorf("failed to find pairing code: %w", err)
	}
	if pairingCode == nil {
		return nil, ErrPairingCodeNotFound
	}

	if time.Now().UTC().After(pairingCode.ExpiresAt) {
		return nil, ErrPairingCodeExpired
	}

	if pairingCode.UserID != userID {
		return nil, ErrPairingCodeMismatch
	}

	existingDevice, err := t.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing device: %w", err)
	}
	if existingDevice != nil {
		return nil, ErrDeviceAlreadyPaired
	}

	newDeviceID := fmt.Sprintf("dev_%s", uuid.New().String())
	newDevice := &domain.Device{
		ID:          newDeviceID,
		UserID:      userID,
		PairingCode: input.PairingCode,
		Platform:    input.Platform,
		DeviceModel: input.DeviceModel,
		DeviceOS:    input.DeviceOS,
		Serial:      input.Serial,
		FCMToken:    "",
		PairedAt:    time.Now().UTC(),
		CreatedAt:   time.Now().UTC(),
	}

	if err := t.repo.Save(ctx, newDevice); err != nil {
		return nil, fmt.Errorf("failed to save device: %w", err)
	}

	_ = t.repo.DeletePairingCode(ctx, input.PairingCode)

	return newDevice, nil
}

func (t *tokenUseCase) RegisterFCMToken(ctx context.Context, input domain.FCMTokenRequest, userID string) error {
	device, err := t.repo.FindByID(ctx, input.DeviceID)
	if err != nil {
		return fmt.Errorf("failed to find device: %w", err)
	}
	if device == nil {
		return ErrDeviceNotFound
	}

	if device.UserID != userID {
		return ErrDeviceNotFound
	}

	if err := t.repo.UpdateFCMToken(ctx, input.DeviceID, input.FCMToken); err != nil {
		return fmt.Errorf("failed to update fcm token: %w", err)
	}

	return nil
}
