package domain

import (
	"context"
)

// DeviceTarget representa un dispositivo al que enviar un push,
// junto con el ID del device para poder limpiar tokens inválidos.
type DeviceTarget struct {
	Token    string
	DeviceID string
}

// PushPayload contiene los datos de la notificación crítica de alerta.
type PushPayload struct {
	Title     string
	Body      string
	AlertID   string
	Type      string
	Latitude  float64
	Longitude float64
}

// PushNotifier define el contrato para enviar notificaciones push críticas.
type PushNotifier interface {
	SendCriticalPush(ctx context.Context, payload PushPayload, targets []DeviceTarget) error
}
