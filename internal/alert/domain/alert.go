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
	ID            string      `json:"alert_id" bson:"_id"`
	UserID        string      `json:"user_id" bson:"user_id"`
	Type          AlertType   `json:"type" bson:"type"`
	Status        AlertStatus `json:"status" bson:"status"`
	Latitude      float64     `json:"latitude" bson:"latitude"`
	Longitude     float64     `json:"longitude" bson:"longitude"`
	TriggerSource string      `json:"trigger_source" bson:"trigger_source"`
	CreatedAt     time.Time   `json:"created_at" bson:"created_at"`
	ResolvedAt    *time.Time  `json:"resolved_at,omitempty" bson:"resolved_at,omitempty"`
}

// CreateAlertInput encapsula los datos de una alerta SOS entrante.
type CreateAlertInput struct {
	Type          AlertType `json:"type"`
	Latitude      float64   `json:"latitude"`
	Longitude     float64   `json:"longitude"`
	TriggerSource string    `json:"trigger_source"`
}

// AlertRepository define el contrato de persistencia para alertas.
type AlertRepository interface {
	Create(ctx context.Context, alert *Alert) error
	FindByID(ctx context.Context, id string) (*Alert, error)
	FindActiveByUserIDs(ctx context.Context, userIDs []string) ([]*Alert, error)
}

// AlertUseCase define la lógica de negocio del motor de alertas.
type AlertUseCase interface {
	CreateAlert(ctx context.Context, victimID string, input CreateAlertInput) (*Alert, error)
	GetByID(ctx context.Context, id string, viewerID string) (*Alert, error)
	GetObserving(ctx context.Context, observerID string) ([]*Alert, error)
}
