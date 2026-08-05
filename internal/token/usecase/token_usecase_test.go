package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	notifDomain "github.com/nemesis-project/api-nemesis/internal/notifications/domain"
	"github.com/nemesis-project/api-nemesis/internal/token/domain"
)

type fakeDeviceRepo struct {
	codes   map[string]*domain.PairingCode
	devices map[string]*domain.Device // by userID
	byID    map[string]*domain.Device
	deleted []string
}

func newFakeDeviceRepo() *fakeDeviceRepo {
	return &fakeDeviceRepo{
		codes:   make(map[string]*domain.PairingCode),
		devices: make(map[string]*domain.Device),
		byID:    make(map[string]*domain.Device),
	}
}

func (f *fakeDeviceRepo) FindByPairingCode(_ context.Context, code string) (*domain.PairingCode, error) {
	return f.codes[code], nil
}
func (f *fakeDeviceRepo) FindActivePairingCodeByUserID(_ context.Context, userID string) (*domain.PairingCode, error) {
	for _, c := range f.codes {
		if c.UserID == userID && time.Now().Before(c.ExpiresAt) {
			return c, nil
		}
	}
	return nil, nil
}
func (f *fakeDeviceRepo) FindByID(_ context.Context, id string) (*domain.Device, error) {
	return f.byID[id], nil
}
func (f *fakeDeviceRepo) FindByUserID(_ context.Context, userID string) (*domain.Device, error) {
	return f.devices[userID], nil
}
func (f *fakeDeviceRepo) FindAllDevicesByUserID(_ context.Context, userID string) ([]*domain.Device, error) {
	var out []*domain.Device
	for _, d := range f.byID {
		if d.UserID == userID && d.FCMToken != "" {
			out = append(out, d)
		}
	}
	return out, nil
}
func (f *fakeDeviceRepo) Save(_ context.Context, device *domain.Device) error {
	f.devices[device.UserID] = device
	f.byID[device.ID] = device
	return nil
}
func (f *fakeDeviceRepo) SavePairingCode(_ context.Context, code *domain.PairingCode) error {
	f.codes[code.Code] = code
	return nil
}
func (f *fakeDeviceRepo) DeletePairingCode(_ context.Context, code string) error {
	f.deleted = append(f.deleted, code)
	delete(f.codes, code)
	return nil
}
func (f *fakeDeviceRepo) UpdateFCMToken(_ context.Context, deviceID string, token string) error {
	if d, ok := f.byID[deviceID]; ok {
		d.FCMToken = token
	}
	return nil
}

type fakeIssuer struct {
	calledUserID string
	token        *domain.SessionToken
	err          error
}

func (f *fakeIssuer) IssueSessionToken(_ context.Context, userID string) (*domain.SessionToken, error) {
	f.calledUserID = userID
	return f.token, f.err
}

type fakeNotifier struct {
	calls   int
	targets []notifDomain.DeviceTarget
}

func (f *fakeNotifier) SendCriticalPush(_ context.Context, _ notifDomain.PushPayload, targets []notifDomain.DeviceTarget) error {
	f.calls++
	f.targets = targets
	return nil
}

// TestGeneratePairingCode_BlocksWhileActiveCodeExists fija que no se pueden
// acumular varios códigos vigentes para la misma cuenta: cada código activo
// es una ventana de login sin contraseña, así que solo debe existir uno.
func TestGeneratePairingCode_BlocksWhileActiveCodeExists(t *testing.T) {
	repo := newFakeDeviceRepo()
	repo.codes["NMS-AAA-AAA-AAA"] = &domain.PairingCode{
		Code: "NMS-AAA-AAA-AAA", UserID: "u1", ExpiresAt: time.Now().Add(time.Minute),
	}
	uc := NewTokenUseCase(repo)

	_, err := uc.GeneratePairingCode(context.Background(), domain.GenerateCodeRequest{UserID: "u1", Platform: "wearos"})
	if !errors.Is(err, ErrPairingCodeAlreadyActive) {
		t.Fatalf("expected ErrPairingCodeAlreadyActive, got %v", err)
	}
}

// TestGeneratePairingCode_AllowsAfterExpiry confirma que un código ya
// expirado no bloquea generar uno nuevo.
func TestGeneratePairingCode_AllowsAfterExpiry(t *testing.T) {
	repo := newFakeDeviceRepo()
	repo.codes["NMS-OLD-OLD-OLD"] = &domain.PairingCode{
		Code: "NMS-OLD-OLD-OLD", UserID: "u1", ExpiresAt: time.Now().Add(-time.Minute),
	}
	uc := NewTokenUseCase(repo)

	code, err := uc.GeneratePairingCode(context.Background(), domain.GenerateCodeRequest{UserID: "u1", Platform: "wearos"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code == nil || code.UserID != "u1" {
		t.Fatalf("expected new code for u1, got %+v", code)
	}
}

func TestPairDevice_NotFound(t *testing.T) {
	repo := newFakeDeviceRepo()
	uc := NewTokenUseCase(repo)

	_, err := uc.PairDevice(context.Background(), domain.PairingRequest{PairingCode: "NMS-XXX-XXX-XXX"})
	if !errors.Is(err, ErrPairingCodeNotFound) {
		t.Fatalf("expected ErrPairingCodeNotFound, got %v", err)
	}
}

func TestPairDevice_Expired(t *testing.T) {
	repo := newFakeDeviceRepo()
	repo.codes["NMS-AAA-AAA-AAA"] = &domain.PairingCode{
		Code: "NMS-AAA-AAA-AAA", UserID: "u1", ExpiresAt: time.Now().Add(-time.Minute),
	}
	uc := NewTokenUseCase(repo)

	_, err := uc.PairDevice(context.Background(), domain.PairingRequest{PairingCode: "NMS-AAA-AAA-AAA"})
	if !errors.Is(err, ErrPairingCodeExpired) {
		t.Fatalf("expected ErrPairingCodeExpired, got %v", err)
	}
}

func TestPairDevice_AlreadyPaired(t *testing.T) {
	repo := newFakeDeviceRepo()
	repo.codes["NMS-AAA-AAA-AAA"] = &domain.PairingCode{
		Code: "NMS-AAA-AAA-AAA", UserID: "u1", ExpiresAt: time.Now().Add(time.Minute),
	}
	repo.devices["u1"] = &domain.Device{ID: "dev_existing", UserID: "u1"}
	uc := NewTokenUseCase(repo)

	_, err := uc.PairDevice(context.Background(), domain.PairingRequest{PairingCode: "NMS-AAA-AAA-AAA"})
	if !errors.Is(err, ErrDeviceAlreadyPaired) {
		t.Fatalf("expected ErrDeviceAlreadyPaired, got %v", err)
	}
}

// TestPairDevice_NoAuthRequired fija que el código, por sí solo, basta para
// registrar el dispositivo sin ninguna sesión previa (login estilo Smart TV).
func TestPairDevice_NoAuthRequired(t *testing.T) {
	repo := newFakeDeviceRepo()
	repo.codes["NMS-AAA-AAA-AAA"] = &domain.PairingCode{
		Code: "NMS-AAA-AAA-AAA", UserID: "u1", ExpiresAt: time.Now().Add(time.Minute),
	}
	uc := NewTokenUseCase(repo)

	result, err := uc.PairDevice(context.Background(), domain.PairingRequest{
		PairingCode: "NMS-AAA-AAA-AAA", Platform: "wearos", DeviceModel: "Pixel Watch", DeviceOS: "wearOS 5", Serial: "SN1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Device == nil || result.Device.UserID != "u1" {
		t.Fatalf("expected device linked to u1, got %+v", result.Device)
	}
	if result.Token != nil {
		t.Errorf("expected nil token when no issuer configured, got %+v", result.Token)
	}
	if len(repo.deleted) != 1 || repo.deleted[0] != "NMS-AAA-AAA-AAA" {
		t.Errorf("expected pairing code to be deleted (single use), got %v", repo.deleted)
	}
}

// TestPairDevice_IssuesSessionToken fija el "toque mágico": con un
// TokenIssuer configurado, emparejar devuelve un JWT ya usable, sin pedir
// correo/contraseña. El JWT se emite para el ID con prefijo "usr_" (el que
// usa el resto de la API), no el ID interno del subsistema de dispositivos.
func TestPairDevice_IssuesSessionToken(t *testing.T) {
	repo := newFakeDeviceRepo()
	repo.codes["NMS-AAA-AAA-AAA"] = &domain.PairingCode{
		Code: "NMS-AAA-AAA-AAA", UserID: "u1", ExpiresAt: time.Now().Add(time.Minute),
	}
	issuer := &fakeIssuer{token: &domain.SessionToken{AccessToken: "jwt.token.here", TokenType: "Bearer", ExpiresIn: 86400}}
	uc := NewTokenUseCase(repo)
	uc.(*tokenUseCase).SetTokenIssuer(issuer)

	result, err := uc.PairDevice(context.Background(), domain.PairingRequest{PairingCode: "NMS-AAA-AAA-AAA", Platform: "wearos"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issuer.calledUserID != "usr_u1" {
		t.Errorf("expected issuer called with usr_u1, got %s", issuer.calledUserID)
	}
	if result.Token == nil || result.Token.AccessToken != "jwt.token.here" {
		t.Errorf("expected session token in result, got %+v", result.Token)
	}
}

// TestPairDevice_NotifiesOtherDevices fija el aviso de seguridad: si la
// cuenta ya tiene otro dispositivo con push registrado, se le notifica el
// nuevo emparejamiento (para detectar un código usado por alguien más).
func TestPairDevice_NotifiesOtherDevices(t *testing.T) {
	repo := newFakeDeviceRepo()
	repo.codes["NMS-AAA-AAA-AAA"] = &domain.PairingCode{
		Code: "NMS-AAA-AAA-AAA", UserID: "u1", ExpiresAt: time.Now().Add(time.Minute),
	}
	repo.byID["dev_phone"] = &domain.Device{ID: "dev_phone", UserID: "u1", FCMToken: "tok_phone"}
	notifier := &fakeNotifier{}
	uc := NewTokenUseCase(repo)
	uc.(*tokenUseCase).SetNotifier(notifier)

	_, err := uc.PairDevice(context.Background(), domain.PairingRequest{PairingCode: "NMS-AAA-AAA-AAA", Platform: "wearos"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if notifier.calls != 1 {
		t.Fatalf("expected 1 notification call, got %d", notifier.calls)
	}
	if len(notifier.targets) != 1 || notifier.targets[0].Token != "tok_phone" {
		t.Errorf("expected notification targeting dev_phone, got %+v", notifier.targets)
	}
}
