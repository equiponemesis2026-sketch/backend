package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/nemesis-project/api-nemesis/internal/alert/domain"
	wsHub "github.com/nemesis-project/api-nemesis/internal/alert/services/ws"
	contactDomain "github.com/nemesis-project/api-nemesis/internal/contact/domain"
	notifDomain "github.com/nemesis-project/api-nemesis/internal/notifications/domain"
	deviceDomain "github.com/nemesis-project/api-nemesis/internal/token/domain"
	userDomain "github.com/nemesis-project/api-nemesis/internal/user/domain"
)

var (
	ErrAlertNotFound    = errors.New("alert not found")
	ErrAlertTypeInvalid = errors.New("invalid alert type")
	ErrInvalidPin       = errors.New("invalid pin")
	ErrNoActiveAlert    = errors.New("no active alert")
)

// PinVerifier permite validar el PIN ingresado por la víctima.
type PinVerifier interface {
	VerifyPin(ctx context.Context, userID string, pin string) (userDomain.PinMatch, error)
}

// TelemetryHub abstrae el hub de WebSocket para el usecase.
type TelemetryHub interface {
	OpenChannel(alertID string)
	Close() // no usado directamente; se cierra por el app lifecycle
}

type alertUseCase struct {
	alertRepo   domain.AlertRepository
	contactRepo contactDomain.ContactRepository
	deviceRepo  deviceDomain.DeviceRepository
	notifier    notifDomain.PushNotifier
	pinVerifier PinVerifier
	hub         *wsHub.Hub
}

// NewAlertUseCase inicializa el caso de uso con todas sus dependencias.
func NewAlertUseCase(
	alertRepo domain.AlertRepository,
	contactRepo contactDomain.ContactRepository,
	deviceRepo deviceDomain.DeviceRepository,
	notifier notifDomain.PushNotifier,
	pinVerifier PinVerifier,
	hub *wsHub.Hub,
) domain.AlertUseCase {
	return &alertUseCase{
		alertRepo:   alertRepo,
		contactRepo: contactRepo,
		deviceRepo:  deviceRepo,
		notifier:    notifier,
		pinVerifier: pinVerifier,
		hub:         hub,
	}
}

// CreateSOS registra una alerta SOS activa, abre su canal de telemetría y
// notifica a los observadores vinculados por FCM multicast.
func (uc *alertUseCase) CreateSOS(ctx context.Context, victimID string, input domain.SOSInput) (*domain.Alert, error) {
	alert := &domain.Alert{
		ID:            fmt.Sprintf("alt_%s", uuid.New().String()),
		UserID:        victimID,
		Type:          domain.AlertTypeSOS,
		Status:        domain.AlertStatusActive,
		Latitude:      input.Latitude,
		Longitude:     input.Longitude,
		BatteryLevel:  input.BatteryLevel,
		Speed:         input.Speed,
		TriggerSource: input.TriggerSource,
		HeartRate:     input.HeartRate,
		CreatedAt:     time.Now().UTC(),
	}

	if err := uc.alertRepo.Create(ctx, alert); err != nil {
		return nil, fmt.Errorf("failed to persist alert: %w", err)
	}

	uc.hub.OpenChannel(alert.ID)

	if err := uc.notifyObservers(ctx, alert, false); err != nil {
		// Best-effort: la alerta ya quedó persistida y el canal abierto.
		// Un fallo del push no debe reportar error a la víctima.
		slog.Warn("alert: push notification failed after SOS persisted", "alert_id", alert.ID, "error", err)
	}

	return alert, nil
}

// CreateCoercion gestiona la entrada de PIN bajo coacción. El endpoint
// siempre responde éxito de forma idéntica, pero el comportamiento difiere:
//   - PIN real   -> resuelve la alerta activa (peligro pasó / falsa alarma).
//   - PIN coerción -> NO apaga nada: escala la alerta a "coercion", enciende
//     la telemetría WS y despacha push silencioso de máxima prioridad.
//   - PIN inválido -> error (el handler responde de forma genérica).
func (uc *alertUseCase) CreateCoercion(ctx context.Context, victimID string, input domain.CoercionInput) (*domain.Alert, error) {
	match, err := uc.pinVerifier.VerifyPin(ctx, victimID, input.PIN)
	if err != nil {
		return nil, fmt.Errorf("failed to verify pin: %w", err)
	}

	if match == userDomain.PinReal {
		active, err := uc.alertRepo.FindActiveByUserID(ctx, victimID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch active alert: %w", err)
		}
		if active == nil {
			return nil, ErrNoActiveAlert
		}
		return uc.ResolveAlert(ctx, active.ID, victimID)
	}

	if match != userDomain.PinCoercion {
		return nil, ErrInvalidPin
	}

	// Escalar la alerta activa a coercion (mantener mismo alert_id y canal WS).
	alert, err := uc.alertRepo.FindActiveByUserID(ctx, victimID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch active alert: %w", err)
	}

	if alert == nil {
		alert = &domain.Alert{
			ID:           fmt.Sprintf("alt_%s", uuid.New().String()),
			UserID:       victimID,
			Type:         domain.AlertTypeCoercion,
			Status:       domain.AlertStatusActive,
			Latitude:     input.Latitude,
			Longitude:    input.Longitude,
			BatteryLevel: input.BatteryLevel,
			CreatedAt:    time.Now().UTC(),
		}
		if err := uc.alertRepo.Create(ctx, alert); err != nil {
			return nil, fmt.Errorf("failed to persist alert: %w", err)
		}
	} else {
		if err := uc.alertRepo.SetType(ctx, alert.ID, domain.AlertTypeCoercion); err != nil {
			return nil, fmt.Errorf("failed to escalate alert: %w", err)
		}
		alert.Type = domain.AlertTypeCoercion
		if input.Latitude != 0 || input.Longitude != 0 {
			alert.Latitude = input.Latitude
			alert.Longitude = input.Longitude
		}
		if input.BatteryLevel > 0 {
			alert.BatteryLevel = input.BatteryLevel
		}
	}

	uc.hub.OpenChannel(alert.ID)

	if err := uc.notifyObservers(ctx, alert, true); err != nil {
		// Best-effort: la escalada a coercion ya quedó persistida.
		slog.Warn("alert: push notification failed after coercion persisted", "alert_id", alert.ID, "error", err)
	}

	return alert, nil
}

// ResolveAlert resuelve la alerta, cierra su canal de telemetría y notifica
// a los observadores que la emergencia terminó.
func (uc *alertUseCase) ResolveAlert(ctx context.Context, alertID string, userID string) (*domain.Alert, error) {
	alert, err := uc.alertRepo.FindByID(ctx, alertID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch alert: %w", err)
	}
	if alert == nil {
		return nil, ErrAlertNotFound
	}
	if alert.UserID != userID {
		return nil, ErrAlertNotFound
	}

	if err := uc.alertRepo.Resolve(ctx, alertID); err != nil {
		return nil, fmt.Errorf("failed to resolve alert: %w", err)
	}

	now := time.Now().UTC()
	alert.Status = domain.AlertStatusResolved
	alert.ResolvedAt = &now

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
		if c.IsVerified && c.UserID == alert.UserID {
			return alert, nil
		}
	}

	return nil, ErrAlertNotFound
}

// GetByUserID lista las alertas propias de la víctima (historial del panel web).
func (uc *alertUseCase) GetByUserID(ctx context.Context, userID string) ([]*domain.Alert, error) {
	alerts, err := uc.alertRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch alerts: %w", err)
	}
	return alerts, nil
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
		if c.IsVerified {
			victimIDs = append(victimIDs, c.UserID)
		}
	}

	return uc.alertRepo.FindActiveByUserIDs(ctx, victimIDs)
}

// notifyObservers resuelve la red de apoyo y envía el push multicast.
// Si silent es true, el push se despacha como silencioso (canal propio).
func (uc *alertUseCase) notifyObservers(ctx context.Context, alert *domain.Alert, silent bool) error {
	contacts, err := uc.contactRepo.FindAllByUserID(ctx, alert.UserID)
	if err != nil {
		return fmt.Errorf("failed to fetch contacts: %w", err)
	}

	title := "ALERTA SOS"
	body := "Emergencia activa en tu red de apoyo. Ábrelo ahora."
	if alert.Type == domain.AlertTypeCoercion {
		title = "Código Rojo"
		body = "Tu ser querido está bajo coacción. Emergencia silenciosa activa."
	}

	payload := notifDomain.PushPayload{
		Title:     title,
		Body:      body,
		AlertID:   alert.ID,
		Type:      string(alert.Type),
		Latitude:  alert.Latitude,
		Longitude: alert.Longitude,
		Silent:    silent,
		ChannelID: "coercion_alerts",
	}

	seen := make(map[string]struct{})
	var targets []notifDomain.DeviceTarget

	for _, contact := range contacts {
		if !contact.IsVerified || contact.LinkedUserID == "" {
			continue
		}
		if _, dup := seen[contact.LinkedUserID]; dup {
			continue
		}
		seen[contact.LinkedUserID] = struct{}{}

		// Los devices guardan el user_id normalizado sin prefijo "usr_",
		// mientras que LinkedUserID conserva el prefijo completo.
		observerID := strings.TrimPrefix(contact.LinkedUserID, "usr_")

		devices, err := uc.deviceRepo.FindAllDevicesByUserID(ctx, observerID)
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

	slog.Info("SOS: resolving observers",
		"alert_id", alert.ID,
		"type", alert.Type,
		"linked_observers", len(seen),
		"push_targets", len(targets),
		"silent", silent,
	)

	if len(targets) == 0 {
		return nil
	}

	return uc.notifier.SendCriticalPush(ctx, payload, targets)
}
