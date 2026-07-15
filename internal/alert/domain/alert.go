package domain

import "context"

type Alert struct {
	ID        string `json:"id" bson:"_id,omitempty"`
	UserID    string `json:"user_id" bson:"user_id"`
	Type      string `json:"type" bson:"type"` // e.g., "PANIC", "SILENT_COERCION"
	Status    string `json:"status" bson:"status"` // e.g., "ACTIVE", "RESOLVED"
	CreatedAt int64  `json:"created_at" bson:"created_at"`
}

type AlertRepository interface {
	Create(ctx context.Context, alert *Alert) error
	GetByID(ctx context.Context, id string) (*Alert, error)
	Update(ctx context.Context, alert *Alert) error
}

type AlertUseCase interface {
	TriggerAlert(ctx context.Context, userID string, alertType string) (*Alert, error)
	VerifyCoercionPIN(ctx context.Context, userID string, pin string) (bool, error)
}
