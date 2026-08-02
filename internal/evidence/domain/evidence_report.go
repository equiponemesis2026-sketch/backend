package domain

import (
	"context"
	"errors"
	"time"

	alertDomain "github.com/nemesis-project/api-nemesis/internal/alert/domain"
	audioDomain "github.com/nemesis-project/api-nemesis/internal/audio/domain"
)

// ErrReportAlreadyExists indica que el expediente ya fue insertado por otra
// solicitud concurrente (carrera TOCTOU sobre el _id = alert_id).
var ErrReportAlreadyExists = errors.New("evidence report already exists")

// TelemetryPoint es un punto GPS del historial de ruta.
type TelemetryPoint struct {
	Latitude     float64 `json:"latitude" bson:"latitude"`
	Longitude    float64 `json:"longitude" bson:"longitude"`
	Speed        float64 `json:"speed,omitempty" bson:"speed,omitempty"`
	BatteryLevel int     `json:"battery_level,omitempty" bson:"battery_level,omitempty"`
	Timestamp    int64   `json:"timestamp" bson:"timestamp"`
}

// AlertMetadata resume los datos del incidente en el expediente.
type AlertMetadata struct {
	AlertID        string                `json:"alert_id" bson:"alert_id"`
	Type           alertDomain.AlertType `json:"type" bson:"type"`
	TriggerSource  string                `json:"trigger_source" bson:"trigger_source"`
	UserID         string                `json:"user_id" bson:"user_id"`
	StartLatitude  float64               `json:"start_latitude" bson:"start_latitude"`
	StartLongitude float64               `json:"start_longitude" bson:"start_longitude"`
	RiskScore      float64               `json:"risk_score,omitempty" bson:"risk_score,omitempty"`
	CreatedAt      time.Time             `json:"created_at" bson:"created_at"`
	ResolvedAt     *time.Time            `json:"resolved_at,omitempty" bson:"resolved_at,omitempty"`
}

// EvidenceReport es el expediente forense inmutable de un incidente.
type EvidenceReport struct {
	ID             string                      `json:"alert_id" bson:"_id"`
	Alert          AlertMetadata               `json:"alert" bson:"alert"`
	TelemetryCount int                         `json:"telemetry_count" bson:"telemetry_count"`
	TelemetryStart *int64                      `json:"telemetry_start,omitempty" bson:"telemetry_start,omitempty"`
	TelemetryEnd   *int64                      `json:"telemetry_end,omitempty" bson:"telemetry_end,omitempty"`
	Path           []TelemetryPoint            `json:"path,omitempty" bson:"path,omitempty"`
	Acoustic       audioDomain.AcousticSummary `json:"acoustic" bson:"acoustic"`
	SHA256         string                      `json:"sha256" bson:"sha256"`
	CreatedAt      time.Time                   `json:"created_at" bson:"created_at"`
}

// EvidenceReportRepository define el contrato de persistencia de expedientes.
type EvidenceReportRepository interface {
	Save(ctx context.Context, report *EvidenceReport) error
	FindByID(ctx context.Context, alertID string) (*EvidenceReport, error)
}

// EvidenceUseCase define la lógica de negocio del reporte forense.
type EvidenceUseCase interface {
	Generate(ctx context.Context, alertID string, viewerID string) (*EvidenceReport, error)
}

// AlertLookup permite consultar la alerta del incidente. Lo implementa el
// repositorio de alertas.
type AlertLookup interface {
	FindByID(ctx context.Context, id string) (*alertDomain.Alert, error)
}

// TelemetryLookup recupera el historial GPS de la alerta.
type TelemetryLookup interface {
	FindByAlertID(ctx context.Context, alertID string) ([]*alertDomain.TelemetryRecord, error)
}

// AcousticLookup agrega el resumen del procesamiento acústico.
type AcousticLookup interface {
	AcousticSummary(ctx context.Context, alertID string) (*audioDomain.AcousticSummary, error)
}

// ContactAuth valida que el observador esté verificado para el incidente.
type ContactAuth interface {
	IsObserverVerified(ctx context.Context, observerID string, victimID string) (bool, error)
}
