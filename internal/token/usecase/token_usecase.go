package usecase

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"

	"github.com/nemesis-project/api-nemesis/internal/token/domain"
)

var (
	ErrPairingCodeExpired    = errors.New("pairing code has expired")
	ErrPairingCodeNotFound   = errors.New("pairing code not found")
	ErrDeviceAlreadyPaired   = errors.New("device is already paired with another user")
)

type tokenUseCase struct {
	repo domain.DeviceRepository
}

func NewTokenUseCase(repo domain.DeviceRepository) domain.TokenUseCase {
	return &tokenUseCase{repo: repo}
}

func (t *tokenUseCase) GeneratePairingCode(ctx context.Context, input domain.GenerateCodeRequest) (*domain.PairingCode, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	rand.Seed(time.Now().UnixNano())
	code := "NMS-"
	for i := 0; i < 3; i++ {
		if i > 0 {
			code += "-"
		}
		for j := 0; j < 3; j++ {
			n := rand.Intn(len(charset))
			code += string(charset[n])
		}
	}
	expiresAt := time.Now().Add(5 * time.Minute).UTC()
	pairingCode := &domain.PairingCode{
		Code:       code,
		UserID:     input.UserID,
		ExpiresAt:  expiresAt,
		CreatedAt:  time.Now().UTC(),
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
		return nil, errors.New("pairing code does not belong to this user")
	}

	existingDevice, err := t.repo.FindByID(ctx, userID)
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

	return newDevice, nil
}