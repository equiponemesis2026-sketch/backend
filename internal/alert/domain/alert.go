package domain

import (
	"context"
	"time"
)

// AlertType identifica el origen de la alerta de emergencia.
type AlertType string

const (
	AlertTypeSOS      AlertType = "sos"
	AlertTypeCoercion AlertType = "coercion"
	AlertTypeAI       AlertType = "ai"
)

// AlertStatus describe el ciclo de vida de una alerta.
type AlertStatus string

const (
	AlertStatusActive   AlertStatus = "active"
	AlertStatusResolved AlertStatus = "resolved"
)

// Alert representa una emergencia activada por una víctima.
type Alert struct {
	ID               string      `json:"alert_id" bson:"_id"`
	UserID           string      `json:"user_id" bson:"user_id"`
	Type             AlertType   `json:"type" bson:"type"`
	Status           AlertStatus `json:"status" bson:"status"`
	Latitude         float64     `json:"latitude" bson:"latitude"`
	Longitude        float64     `json:"longitude" bson:"longitude"`
	BatteryLevel     int         `json:"battery_level,omitempty" bson:"battery_level,omitempty"`
	Speed            float64     `json:"speed,omitempty" bson:"speed,omitempty"`
	TriggerSource    string      `json:"trigger_source" bson:"trigger_source"`
	HeartRate        int         `json:"heart_rate,omitempty" bson:"heart_rate,omitempty"`
	RiskScore        float64     `json:"risk_score,omitempty" bson:"risk_score,omitempty"`
	DistressDetected bool        `json:"distress_detected" bson:"distress_detected"`
	CreatedAt        time.Time   `json:"created_at" bson:"created_at"`
	ResolvedAt       *time.Time  `json:"resolved_at,omitempty" bson:"resolved_at,omitempty"`
}

// SOSInput encapsula los datos de una alerta SOS entrante desde el
// smartwatch o la app móvil. La decisión de disparar la alerta (p. ej. un
// incremento de ritmo cardíaco por encima del umbral del wearable) la toma
// el dispositivo; HeartRate solo registra el valor detectado en ese momento
// para el expediente forense y para auditar falsos positivos.
type SOSInput struct {
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	BatteryLevel  int     `json:"battery_level"`
	Speed         float64 `json:"speed"`
	TriggerSource string  `json:"trigger_source"`
	HeartRate     int     `json:"heart_rate,omitempty"`
}

// CoercionInput encapsula los datos de la entrada de PIN bajo coacción.
type CoercionInput struct {
	PIN          string  `json:"pin"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	BatteryLevel int     `json:"battery_level"`
}

// TelemetryPacket representa un paquete de telemetría GPS en vivo enviado
// por el emisor (smartwatch) y retransmitido a los observadores.
type TelemetryPacket struct {
	AlertID      string  `json:"alert_id"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	Speed        float64 `json:"speed"`
	BatteryLevel int     `json:"battery_level"`
	Timestamp    int64   `json:"timestamp"`
}

// AlertRepository define el contrato de persistencia para alertas.
type AlertRepository interface {
	Create(ctx context.Context, alert *Alert) error
	FindByID(ctx context.Context, id string) (*Alert, error)
	FindByUserID(ctx context.Context, userID string) ([]*Alert, error)
	FindActiveByUserID(ctx context.Context, userID string) (*Alert, error)
	FindActiveByUserIDs(ctx context.Context, userIDs []string) ([]*Alert, error)
	Resolve(ctx context.Context, id string) error
	SetType(ctx context.Context, id string, alertType AlertType) error
	UpdateRiskMetrics(ctx context.Context, id string, score float64, distress bool) error
}

// AlertUseCase define la lógica de negocio del motor de alertas.
type AlertUseCase interface {
	CreateSOS(ctx context.Context, victimID string, input SOSInput) (*Alert, error)
	CreateCoercion(ctx context.Context, victimID string, input CoercionInput) (*Alert, error)
	ResolveAlert(ctx context.Context, alertID string, userID string) (*Alert, error)
	GetByID(ctx context.Context, id string, viewerID string) (*Alert, error)
	GetByUserID(ctx context.Context, userID string) ([]*Alert, error)
	GetObserving(ctx context.Context, observerID string) ([]*Alert, error)
}
