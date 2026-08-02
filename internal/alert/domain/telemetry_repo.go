package domain

import (
	"context"
	"time"
)

// TelemetryRecord representa un punto de telemetría GPS persistido durante
// una alerta. Alimenta el historial de ruta del reporte forense.
type TelemetryRecord struct {
	AlertID      string    `json:"alert_id" bson:"alert_id"`
	UserID       string    `json:"user_id" bson:"user_id"`
	Latitude     float64   `json:"latitude" bson:"latitude"`
	Longitude    float64   `json:"longitude" bson:"longitude"`
	Speed        float64   `json:"speed,omitempty" bson:"speed,omitempty"`
	BatteryLevel int       `json:"battery_level,omitempty" bson:"battery_level,omitempty"`
	Timestamp    int64     `json:"timestamp" bson:"timestamp"`
	ReceivedAt   time.Time `json:"received_at" bson:"received_at"`
}

// TelemetryRepository define el contrato de persistencia de telemetría.
type TelemetryRepository interface {
	Save(ctx context.Context, record *TelemetryRecord) error
	FindByAlertID(ctx context.Context, alertID string) ([]*TelemetryRecord, error)
}
