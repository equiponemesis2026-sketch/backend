package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/nemesis-project/api-nemesis/internal/alert/domain"
	contactDomain "github.com/nemesis-project/api-nemesis/internal/contact/domain"
	notifDomain "github.com/nemesis-project/api-nemesis/internal/notifications/domain"
	deviceDomain "github.com/nemesis-project/api-nemesis/internal/token/domain"
)

var (
	ErrAlertNotFound    = errors.New("alert not found")
	ErrAlertTypeInvalid = errors.New("invalid alert type")
)

type alertUseCase struct {
	alertRepo   domain.AlertRepository
	contactRepo contactDomain.ContactRepository
	deviceRepo  deviceDomain.DeviceRepository
	notifier    notifDomain.PushNotifier
}

func NewAlertUseCase(
	alertRepo domain.AlertRepository,
	contactRepo contactDomain.ContactRepository,
	deviceRepo deviceDomain.DeviceRepository,
	notifier notifDomain.PushNotifier,
) domain.AlertUseCase {
	return &alertUseCase{
		alertRepo:   alertRepo,
		contactRepo: contactRepo,
		deviceRepo:  deviceRepo,
		notifier:    notifier,
	}
}

// CreateAlert persiste la alerta y dispara push crítico a todos los
// observadores vinculados (multicast FCM a todos sus dispositivos).
func (uc *alertUseCase) CreateAlert(ctx context.Context, victimID string, input domain.CreateAlertInput) (*domain.Alert, error) {
	if input.Type == "" {
		input.Type = domain.AlertTypeSOS
	}
	if input.Type != domain.AlertTypeSOS &&
		input.Type != domain.AlertTypeCoercion &&
		input.Type != domain.AlertTypeAI {
		return nil, ErrAlertTypeInvalid
	}

	alert := &domain.Alert{
		ID:            fmt.Sprintf("alt_%s", uuid.New().String()),
		UserID:        victimID,
		Type:          input.Type,
		Status:        domain.AlertStatusActive,
		Latitude:      input.Latitude,
		Longitude:     input.Longitude,
		TriggerSource: input.TriggerSource,
		CreatedAt:     time.Now().UTC(),
	}

	if err := uc.alertRepo.Create(ctx, alert); err != nil {
		return nil, fmt.Errorf("failed to persist alert: %w", err)
	}

	if err := uc.notifyObservers(ctx, alert); err != nil {
		return nil, fmt.Errorf("failed to notify observers: %w", err)
	}

	return alert, nil
}

// GetByID permite a la víctima o a un observador vinculado ver los detalles de la alerta.
func (uc *alertUseCase) GetByID(ctx context.Context, id string, viewerID string) (*domain.Alert, error) {
	alert, err := uc.alertRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch alert: %w", err)
	}
	if alert == nil {
		return nil, ErrAlertNotFound
	}

	if alert.UserID == viewerID {
		return alert, nil
	}

	contacts, err := uc.contactRepo.FindAllByLinkedUserID(ctx, viewerID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch observer contacts: %w", err)
	}
	for _, c := range contacts {
		if c.UserID == alert.UserID {
			return alert, nil
		}
	}

	return nil, ErrAlertNotFound
}

// GetObserving lista las emergencias activas de las víctimas a las que el
// observador está vinculado (dashboard / mapa en vivo).
func (uc *alertUseCase) GetObserving(ctx context.Context, observerID string) ([]*domain.Alert, error) {
	contacts, err := uc.contactRepo.FindAllByLinkedUserID(ctx, observerID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch observer contacts: %w", err)
	}

	victimIDs := make([]string, 0, len(contacts))
	for _, c := range contacts {
		victimIDs = append(victimIDs, c.UserID)
	}

	return uc.alertRepo.FindActiveByUserIDs(ctx, victimIDs)
}

// notifyObservers resuelve la red de apoyo y envía el push multicast.
func (uc *alertUseCase) notifyObservers(ctx context.Context, alert *domain.Alert) error {
	contacts, err := uc.contactRepo.FindAllByUserID(ctx, alert.UserID)
	if err != nil {
		return fmt.Errorf("failed to fetch contacts: %w", err)
	}

	payload := notifDomain.PushPayload{
		Title:     "ALERTA SOS",
		Body:      "Emergencia activa en tu red de apoyo. Ábrelo ahora.",
		AlertID:   alert.ID,
		Type:      string(alert.Type),
		Latitude:  alert.Latitude,
		Longitude: alert.Longitude,
	}

	seen := make(map[string]struct{})
	var targets []notifDomain.DeviceTarget

	for _, contact := range contacts {
		if contact.LinkedUserID == "" {
			continue
		}
		if _, dup := seen[contact.LinkedUserID]; dup {
			continue
		}
		seen[contact.LinkedUserID] = struct{}{}

		devices, err := uc.deviceRepo.FindAllDevicesByUserID(ctx, contact.LinkedUserID)
		if err != nil {
			return fmt.Errorf("failed to fetch observer devices: %w", err)
		}
		for _, d := range devices {
			if d.FCMToken == "" {
				continue
			}
			targets = append(targets, notifDomain.DeviceTarget{
				Token:    d.FCMToken,
				DeviceID: d.ID,
			})
		}
	}

	if len(targets) == 0 {
		return nil
	}

	return uc.notifier.SendCriticalPush(ctx, payload, targets)
}
