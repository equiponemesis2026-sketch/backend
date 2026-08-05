package usecase

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/google/uuid"

	notifDomain "github.com/nemesis-project/api-nemesis/internal/notifications/domain"
	"github.com/nemesis-project/api-nemesis/internal/token/domain"
)

var (
	ErrPairingCodeExpired       = errors.New("pairing code has expired")
	ErrPairingCodeNotFound      = errors.New("pairing code not found")
	ErrPairingCodeAlreadyActive = errors.New("a pairing code is already active for this user")
	ErrDeviceAlreadyPaired      = errors.New("device is already paired with another user")
	ErrDeviceNotFound           = errors.New("device not found")
)

type tokenUseCase struct {
	repo        domain.DeviceRepository
	tokenIssuer domain.TokenIssuer
	notifier    notifDomain.PushNotifier
}

func NewTokenUseCase(repo domain.DeviceRepository) domain.TokenUseCase {
	return &tokenUseCase{repo: repo}
}

// SetTokenIssuer inyecta el emisor de JWT (opcional). Sin él, PairDevice
// registra el dispositivo pero no devuelve sesión (comportamiento anterior).
func (t *tokenUseCase) SetTokenIssuer(issuer domain.TokenIssuer) {
	t.tokenIssuer = issuer
}

// SetNotifier inyecta el notificador push (opcional) para avisar a los
// demás dispositivos de la cuenta cuando se empareja uno nuevo.
func (t *tokenUseCase) SetNotifier(notifier notifDomain.PushNotifier) {
	t.notifier = notifier
}

// GeneratePairingCode crea un código de emparejamiento nuevo. Si el usuario
// ya tiene uno vigente (no expirado), lo rechaza en vez de emitir otro: el
// código funciona como credencial de login, así que mantener varios activos
// a la vez solo amplía la ventana de exposición si alguno se filtra.
func (t *tokenUseCase) GeneratePairingCode(ctx context.Context, input domain.GenerateCodeRequest) (*domain.PairingCode, error) {
	active, err := t.repo.FindActivePairingCodeByUserID(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to check active pairing code: %w", err)
	}
	if active != nil {
		return nil, ErrPairingCodeAlreadyActive
	}

	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	code := "NMS-"
	for i := 0; i < 3; i++ {
		if i > 0 {
			code += "-"
		}
		for j := 0; j < 3; j++ {
			n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
			if err != nil {
				return nil, fmt.Errorf("failed to generate secure random: %w", err)
			}
			code += string(charset[n.Int64()])
		}
	}
	expiresAt := time.Now().Add(5 * time.Minute).UTC()
	pairingCode := &domain.PairingCode{
		Code:      code,
		UserID:    input.UserID,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().UTC(),
	}

	if err := t.repo.SavePairingCode(ctx, pairingCode); err != nil {
		return nil, fmt.Errorf("failed to store pairing code: %w", err)
	}

	return pairingCode, nil
}

// PairDevice valida el código de emparejamiento y registra el dispositivo.
// El código es la única credencial: no requiere que el llamante ya tenga
// una sesión (un wearable no puede teclear correo/contraseña). Por eso el
// código debe ser de un solo uso, expirar rápido (5 min, ver
// GeneratePairingCode) y generarse con crypto/rand: quien lo posea dentro
// de esa ventana puede iniciar sesión como el dueño de la cuenta.
func (t *tokenUseCase) PairDevice(ctx context.Context, input domain.PairingRequest) (*domain.PairDeviceResult, error) {
	pairingCode, err := t.repo.FindByPairingCode(ctx, input.PairingCode)
	if err != nil {
		return nil, fmt.Errorf("failed to find pairing code: %w", err)
	}
	if pairingCode == nil {
		return nil, ErrPairingCodeNotFound
	}

	if time.Now().UTC().After(pairingCode.ExpiresAt) {
		return nil, ErrPairingCodeExpired
	}

	// user_id aquí es el ID de negocio sin el prefijo "usr_" (convención del
	// subsistema de dispositivos); el resto de la API sí lo usa con prefijo.
	userID := pairingCode.UserID

	existingDevice, err := t.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing device: %w", err)
	}
	if existingDevice != nil {
		return nil, ErrDeviceAlreadyPaired
	}

	newDeviceID := fmt.Sprintf("dev_%s", uuid.New().String())
	newDevice := &domain.Device{
		ID:          newDeviceID,
		UserID:      userID,
		PairingCode: input.PairingCode,
		Platform:    input.Platform,
		DeviceModel: input.DeviceModel,
		DeviceOS:    input.DeviceOS,
		Serial:      input.Serial,
		FCMToken:    "",
		PairedAt:    time.Now().UTC(),
		CreatedAt:   time.Now().UTC(),
	}

	if err := t.repo.Save(ctx, newDevice); err != nil {
		return nil, fmt.Errorf("failed to save device: %w", err)
	}

	_ = t.repo.DeletePairingCode(ctx, input.PairingCode)

	result := &domain.PairDeviceResult{Device: newDevice}

	if t.tokenIssuer != nil {
		token, err := t.tokenIssuer.IssueSessionToken(ctx, "usr_"+userID)
		if err != nil {
			return nil, fmt.Errorf("failed to issue session token: %w", err)
		}
		result.Token = token
	}

	t.notifyDevicePaired(ctx, userID, newDevice)

	return result, nil
}

// notifyDevicePaired avisa por push a los demás dispositivos ya vinculados
// de la cuenta cuando se empareja uno nuevo (equivalente a los avisos de
// "nuevo inicio de sesión" de Netflix/Google). Best-effort: nunca bloquea
// ni falla el emparejamiento, y es la única señal que tiene la víctima si
// alguien más emparejó un dispositivo con un código que vio u obtuvo.
func (t *tokenUseCase) notifyDevicePaired(ctx context.Context, userID string, device *domain.Device) {
	if t.notifier == nil {
		return
	}

	devices, err := t.repo.FindAllDevicesByUserID(ctx, userID)
	if err != nil {
		slog.Warn("token: failed to fetch devices for pairing notification", "user_id", userID, "error", err)
		return
	}

	var targets []notifDomain.DeviceTarget
	for _, d := range devices {
		if d.ID == device.ID || d.FCMToken == "" {
			continue
		}
		targets = append(targets, notifDomain.DeviceTarget{Token: d.FCMToken, DeviceID: d.ID})
	}
	if len(targets) == 0 {
		return
	}

	payload := notifDomain.PushPayload{
		Title: "Nuevo dispositivo vinculado",
		Body:  fmt.Sprintf("Se vinculó un %s a tu cuenta Némesis. Si no fuiste tú, revisa tu seguridad.", device.Platform),
		Type:  "device_paired",
	}
	if err := t.notifier.SendCriticalPush(ctx, payload, targets); err != nil {
		slog.Warn("token: failed to notify device paired", "user_id", userID, "error", err)
	}
}

func (t *tokenUseCase) RegisterFCMToken(ctx context.Context, input domain.FCMTokenRequest, userID string) error {
	device, err := t.repo.FindByID(ctx, input.DeviceID)
	if err != nil {
		return fmt.Errorf("failed to find device: %w", err)
	}
	if device == nil {
		return ErrDeviceNotFound
	}

	if device.UserID != userID {
		return ErrDeviceNotFound
	}

	if err := t.repo.UpdateFCMToken(ctx, input.DeviceID, input.FCMToken); err != nil {
		return fmt.Errorf("failed to update fcm token: %w", err)
	}

	return nil
}
