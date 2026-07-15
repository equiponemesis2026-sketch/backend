package usecase

import (
	"context"
	"github.com/nemesis-back/internal/alert/domain"
)

type alertUseCase struct {
	alertRepo domain.AlertRepository
}

func NewAlertUseCase(repo domain.AlertRepository) domain.AlertUseCase {
	return &alertUseCase{
		alertRepo: repo,
	}
}

func (u *alertUseCase) TriggerAlert(ctx context.Context, userID string, alertType string) (*domain.Alert, error) {
	// TODO: Implement alert trigger logic
	return nil, nil
}

func (u *alertUseCase) VerifyCoercionPIN(ctx context.Context, userID string, pin string) (bool, error) {
	// TODO: Implement constant-time timing safe coercion PIN verification
	return false, nil
}
