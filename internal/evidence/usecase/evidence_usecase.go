package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	alertDomain "github.com/nemesis-project/api-nemesis/internal/alert/domain"
	"github.com/nemesis-project/api-nemesis/internal/evidence/domain"
)

var (
	ErrReportNotFound  = errors.New("alert not found")
	ErrReportForbidden = errors.New("forbidden")
)

// maxPathPoints limita los puntos de ruta embebidos en el expediente para
// no inflar el documento.
const maxPathPoints = 500

type evidenceUseCase struct {
	alertRepo     domain.AlertLookup
	telemetryRepo domain.TelemetryLookup
	chunkRepo     domain.AcousticLookup
	evidenceRepo  domain.EvidenceReportRepository
	contactAuth   domain.ContactAuth
}

// NewEvidenceUseCase crea el caso de uso del reporte forense.
func NewEvidenceUseCase(
	alertRepo domain.AlertLookup,
	telemetryRepo domain.TelemetryLookup,
	chunkRepo domain.AcousticLookup,
	evidenceRepo domain.EvidenceReportRepository,
	contactAuth domain.ContactAuth,
) domain.EvidenceUseCase {
	return &evidenceUseCase{
		alertRepo:     alertRepo,
		telemetryRepo: telemetryRepo,
		chunkRepo:     chunkRepo,
		evidenceRepo:  evidenceRepo,
		contactAuth:   contactAuth,
	}
}

// Generate consolida el expediente forense del incidente. Solo la víctima o
// un observador con IsVerified=true pueden solicitarlo.
func (uc *evidenceUseCase) Generate(ctx context.Context, alertID string, viewerID string) (*domain.EvidenceReport, error) {
	alert, err := uc.alertRepo.FindByID(ctx, alertID)
	if err != nil {
		return nil, err
	}
	if alert == nil {
		return nil, ErrReportNotFound
	}

	allowed := alert.UserID == viewerID
	if !allowed {
		verified, err := uc.contactAuth.IsObserverVerified(ctx, viewerID, alert.UserID)
		if err != nil {
			return nil, err
		}
		allowed = verified
	}
	if !allowed {
		return nil, ErrReportForbidden
	}

	// Acceso validado. Si el expediente ya existe es inmutable: se devuelve.
	existing, err := uc.evidenceRepo.FindByID(ctx, alertID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	telemetry, err := uc.telemetryRepo.FindByAlertID(ctx, alertID)
	if err != nil {
		return nil, err
	}

	acoustic, err := uc.chunkRepo.AcousticSummary(ctx, alertID)
	if err != nil {
		return nil, err
	}

	report := &domain.EvidenceReport{
		ID: alertID,
		Alert: domain.AlertMetadata{
			AlertID:        alert.ID,
			Type:           alert.Type,
			TriggerSource:  alert.TriggerSource,
			HeartRate:      alert.HeartRate,
			UserID:         alert.UserID,
			StartLatitude:  alert.Latitude,
			StartLongitude: alert.Longitude,
			RiskScore:      alert.RiskScore,
			CreatedAt:      alert.CreatedAt,
			ResolvedAt:     alert.ResolvedAt,
		},
		TelemetryCount: len(telemetry),
		Path:           buildPath(telemetry),
		Acoustic:       *acoustic,
		CreatedAt:      time.Now().UTC(),
	}

	if len(telemetry) > 0 {
		start := telemetry[0].Timestamp
		end := telemetry[len(telemetry)-1].Timestamp
		report.TelemetryStart = &start
		report.TelemetryEnd = &end
	}

	// Hash de integridad sha256 sobre una representación canónica.
	sum, err := hashReport(report)
	if err != nil {
		return nil, err
	}
	report.SHA256 = sum

	if err := uc.evidenceRepo.Save(ctx, report); err != nil {
		// Carrera TOCTOU: otra solicitud insertó el expediente primero.
		// Es inmutable, así que devolvemos el que quedó guardado.
		if errors.Is(err, domain.ErrReportAlreadyExists) {
			return uc.evidenceRepo.FindByID(ctx, alertID)
		}
		return nil, err
	}
	return report, nil
}

// buildPath limita el historial de telemetría a los primeros maxPathPoints.
func buildPath(records []*alertDomain.TelemetryRecord) []domain.TelemetryPoint {
	n := len(records)
	if n > maxPathPoints {
		n = maxPathPoints
	}
	path := make([]domain.TelemetryPoint, 0, n)
	for i := 0; i < n; i++ {
		rec := records[i]
		path = append(path, domain.TelemetryPoint{
			Latitude:     rec.Latitude,
			Longitude:    rec.Longitude,
			Speed:        rec.Speed,
			BatteryLevel: rec.BatteryLevel,
			Timestamp:    rec.Timestamp,
		})
	}
	return path
}

// hashReport calcula el sha256 del JSON del reporte sin el campo hash.
func hashReport(report *domain.EvidenceReport) (string, error) {
	// Serializar sin el campo SHA256 para un hash estable.
	clone := *report
	clone.SHA256 = ""
	data, err := json.Marshal(clone)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return sumToHex(sum[:]), nil
}

func sumToHex(sum []byte) string {
	const hexDigits = "0123456789abcdef"
	out := make([]byte, len(sum)*2)
	for i, b := range sum {
		out[i*2] = hexDigits[b>>4]
		out[i*2+1] = hexDigits[b&0x0f]
	}
	return string(out)
}
