package service

import (
	"context"
	"log/slog"
	"strconv"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"

	notifDomain "github.com/nemesis-project/api-nemesis/internal/notifications/domain"
	"github.com/nemesis-project/api-nemesis/internal/token/domain"
)

// fcmService envía push críticos vía Firebase Cloud Messaging.
// Si no hay credenciales configuradas, opera en modo degradado (dry-run):
// registra las alertas y simula el envío para no bloquear desarrollo local.
type fcmService struct {
	enabled    bool
	messaging  *messaging.Client
	deviceRepo domain.DeviceRepository
}

// NewService crea el servicio FCM. Si credentialsJSON está vacío, entra en dry-run.
func NewService(ctx context.Context, credentialsJSON string, deviceRepo domain.DeviceRepository) (notifDomain.PushNotifier, error) {
	if credentialsJSON == "" {
		slog.Warn("FCM credentials missing: running in dry-run mode, Push skipped")
		return &fcmService{enabled: false, deviceRepo: deviceRepo}, nil
	}

	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsJSON([]byte(credentialsJSON)))
	if err != nil {
		return nil, err
	}

	msg, err := app.Messaging(ctx)
	if err != nil {
		return nil, err
	}

	return &fcmService{enabled: true, messaging: msg, deviceRepo: deviceRepo}, nil
}

func (s *fcmService) SendCriticalPush(ctx context.Context, payload notifDomain.PushPayload, targets []notifDomain.DeviceTarget) error {
	if !s.enabled {
		slog.Warn("FCM credentials missing: Push skipped", "alert_id", payload.AlertID, "targets", len(targets))
		return nil
	}

	if len(targets) == 0 {
		return nil
	}

	tokens := make([]string, 0, len(targets))
	tokenByDevice := make(map[string]string, len(targets))
	for _, t := range targets {
		if t.Token == "" {
			continue
		}
		tokens = append(tokens, t.Token)
		tokenByDevice[t.Token] = t.DeviceID
	}

	if len(tokens) == 0 {
		return nil
	}

	msg := &messaging.MulticastMessage{
		Tokens: tokens,
		Notification: &messaging.Notification{
			Title: payload.Title,
			Body:  payload.Body,
		},
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				Title: payload.Title,
				Body:  payload.Body,
			},
		},
		Data: map[string]string{
			"alert_id":  payload.AlertID,
			"type":      payload.Type,
			"latitude":  strconv.FormatFloat(payload.Latitude, 'f', -1, 64),
			"longitude": strconv.FormatFloat(payload.Longitude, 'f', -1, 64),
		},
	}

	br, err := s.messaging.SendEachForMulticast(ctx, msg)
	if err != nil {
		return err
	}

	// Limpia tokens que FCM reporta como inválidos (Unregistered/NotRegistered).
	for i, resp := range br.Responses {
		if !resp.Success {
			if messaging.IsUnregistered(resp.Error) || messaging.IsInvalidArgument(resp.Error) {
				deviceID := tokenByDevice[tokens[i]]
				if deviceID != "" {
					slog.Warn("FCM token unregistered, clearing", "device_id", deviceID)
					_ = s.deviceRepo.UpdateFCMToken(ctx, deviceID, "")
				}
			}
		}
	}

	return nil
}
