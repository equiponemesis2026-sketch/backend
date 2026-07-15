package usecase

import (
	"context"
	"github.com/nemesis-back/internal/telemetry/domain"
)

type telemetryUseCase struct {
	telemetryRepo domain.TelemetryRepository
}

func NewTelemetryUseCase(repo domain.TelemetryRepository) domain.TelemetryUseCase {
	return &telemetryUseCase{
		telemetryRepo: repo,
	}
}

func (u *telemetryUseCase) Ingest(ctx context.Context, telemetry *domain.Telemetry) error {
	// TODO: Implement telemetry ingest logic & alert triggering check
	return nil
}
