package usecase_test

import (
	"context"
	"sync"
	"testing"

	"github.com/nemesis-project/api-nemesis/internal/alert/domain"
	"github.com/nemesis-project/api-nemesis/internal/alert/services/ws"
	"github.com/nemesis-project/api-nemesis/internal/alert/usecase"
	contactDomain "github.com/nemesis-project/api-nemesis/internal/contact/domain"
	notifDomain "github.com/nemesis-project/api-nemesis/internal/notifications/domain"
	deviceDomain "github.com/nemesis-project/api-nemesis/internal/token/domain"
	userDomain "github.com/nemesis-project/api-nemesis/internal/user/domain"
)

type fakeAlertRepo struct {
	alerts       map[string]*domain.Alert
	byUserIDs    []string
	activeResult []*domain.Alert
}

func (f *fakeAlertRepo) Create(_ context.Context, a *domain.Alert) error {
	if f.alerts == nil {
		f.alerts = make(map[string]*domain.Alert)
	}
	f.alerts[a.ID] = a
	return nil
}

func (f *fakeAlertRepo) FindByID(_ context.Context, id string) (*domain.Alert, error) {
	if f.alerts == nil {
		return nil, nil
	}
	return f.alerts[id], nil
}

func (f *fakeAlertRepo) FindActiveByUserID(_ context.Context, userID string) (*domain.Alert, error) {
	if f.alerts == nil {
		return nil, nil
	}
	var latest *domain.Alert
	for _, a := range f.alerts {
		if a.UserID == userID && a.Status == domain.AlertStatusActive {
			if latest == nil || a.CreatedAt.After(latest.CreatedAt) {
				latest = a
			}
		}
	}
	return latest, nil
}

func (f *fakeAlertRepo) FindActiveByUserIDs(_ context.Context, userIDs []string) ([]*domain.Alert, error) {
	f.byUserIDs = userIDs
	var result []*domain.Alert
	for _, a := range f.activeResult {
		for _, id := range userIDs {
			if a.UserID == id {
				result = append(result, a)
			}
		}
	}
	return result, nil
}

func (f *fakeAlertRepo) Resolve(_ context.Context, id string) error {
	if f.alerts != nil {
		if a, ok := f.alerts[id]; ok {
			a.Status = domain.AlertStatusResolved
		}
	}
	return nil
}

func (f *fakeAlertRepo) SetType(_ context.Context, id string, alertType domain.AlertType) error {
	if f.alerts != nil {
		if a, ok := f.alerts[id]; ok {
			a.Type = alertType
		}
	}
	return nil
}

type fakeContactRepo struct {
	contacts       []*contactDomain.Contact // para FindAllByUserID
	linkedResult   []*contactDomain.Contact
	linkedReceived string
}

func (f *fakeContactRepo) Create(_ context.Context, _ *contactDomain.Contact) error { return nil }
func (f *fakeContactRepo) FindByID(_ context.Context, _, _ string) (*contactDomain.Contact, error) {
	return nil, nil
}
func (f *fakeContactRepo) FindAllByUserID(_ context.Context, _ string) ([]*contactDomain.Contact, error) {
	return f.contacts, nil
}
func (f *fakeContactRepo) FindAllByLinkedUserID(_ context.Context, linkedUserID string) ([]*contactDomain.Contact, error) {
	f.linkedReceived = linkedUserID
	return f.linkedResult, nil
}
func (f *fakeContactRepo) FindAllPendingByLinkedUserID(_ context.Context, _ string) ([]*contactDomain.Contact, error) {
	return nil, nil
}
func (f *fakeContactRepo) FindByIDForLinkedUser(_ context.Context, _, _ string) (*contactDomain.Contact, error) {
	return nil, nil
}
func (f *fakeContactRepo) SetVerified(_ context.Context, _, _ string, _ bool) (bool, error) {
	return true, nil
}
func (f *fakeContactRepo) UnlinkContact(_ context.Context, _, _ string) error       { return nil }
func (f *fakeContactRepo) Update(_ context.Context, _ *contactDomain.Contact) error { return nil }
func (f *fakeContactRepo) Delete(_ context.Context, _, _ string) error              { return nil }
func (f *fakeContactRepo) LinkContact(_ context.Context, _, _, _ string) error      { return nil }
func (f *fakeContactRepo) LinkPendingContacts(_ context.Context, _, _, _ string) error {
	return nil
}

type fakeDeviceRepo struct {
	devices map[string][]*deviceDomain.Device
}

func (f *fakeDeviceRepo) FindByPairingCode(_ context.Context, _ string) (*deviceDomain.PairingCode, error) {
	return nil, nil
}
func (f *fakeDeviceRepo) FindByID(_ context.Context, _ string) (*deviceDomain.Device, error) {
	return nil, nil
}
func (f *fakeDeviceRepo) FindByUserID(_ context.Context, _ string) (*deviceDomain.Device, error) {
	return nil, nil
}
func (f *fakeDeviceRepo) FindAllDevicesByUserID(_ context.Context, userID string) ([]*deviceDomain.Device, error) {
	if f.devices == nil {
		return []*deviceDomain.Device{}, nil
	}
	return f.devices[userID], nil
}
func (f *fakeDeviceRepo) Save(_ context.Context, _ *deviceDomain.Device) error { return nil }
func (f *fakeDeviceRepo) SavePairingCode(_ context.Context, _ *deviceDomain.PairingCode) error {
	return nil
}
func (f *fakeDeviceRepo) DeletePairingCode(_ context.Context, _ string) error { return nil }
func (f *fakeDeviceRepo) UpdateFCMToken(_ context.Context, _, _ string) error { return nil }

type fakeNotifier struct {
	mu     sync.Mutex
	calls  int
	tokens []string
	lastP  notifDomain.PushPayload
}

func (f *fakeNotifier) SendCriticalPush(_ context.Context, payload notifDomain.PushPayload, targets []notifDomain.DeviceTarget) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.lastP = payload
	for _, t := range targets {
		f.tokens = append(f.tokens, t.Token)
	}
	return nil
}

type fakePinVerifier struct {
	result userDomain.PinMatch
	err    error
}

func (f *fakePinVerifier) VerifyPin(_ context.Context, _ string, _ string) (userDomain.PinMatch, error) {
	return f.result, f.err
}

func newTestUC(
	alertRepo *fakeAlertRepo,
	contactRepo *fakeContactRepo,
	deviceRepo *fakeDeviceRepo,
	notifier *fakeNotifier,
	pinVerifier *fakePinVerifier,
) domain.AlertUseCase {
	hub := ws.NewHub(context.Background())
	return usecase.NewAlertUseCase(alertRepo, contactRepo, deviceRepo, notifier, pinVerifier, hub)
}

func TestCreateSOS_NotifiesLinkedObservers(t *testing.T) {
	contacts := []*contactDomain.Contact{
		{ID: "c1", UserID: "usr_victim", LinkedUserID: "usr_obs1", IsVerified: true},
		{ID: "c2", UserID: "usr_victim", LinkedUserID: "usr_obs2", IsVerified: true},
		{ID: "c3", UserID: "usr_victim"}, // sin vincular
	}
	devices := map[string][]*deviceDomain.Device{
		"obs1": { // devices guardan user_id normalizado sin prefijo
			{ID: "dev_a", UserID: "obs1", FCMToken: "tok_a"},
			{ID: "dev_b", UserID: "obs1", FCMToken: "tok_b"}, // segundo dispositivo
		},
		"obs2": {
			{ID: "dev_c", UserID: "obs2", FCMToken: "tok_c"},
		},
	}

	alertRepo := &fakeAlertRepo{}
	contactRepo := &fakeContactRepo{contacts: contacts}
	deviceRepo := &fakeDeviceRepo{devices: devices}
	notifier := &fakeNotifier{}
	uc := newTestUC(alertRepo, contactRepo, deviceRepo, notifier, &fakePinVerifier{})

	alert, err := uc.CreateSOS(context.Background(), "usr_victim", domain.SOSInput{
		Latitude:     19.4326,
		Longitude:    -99.1332,
		BatteryLevel: 85,
		Speed:        12.5,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if alert == nil {
		t.Fatal("expected alert, got nil")
	}
	if alert.Type != domain.AlertTypeSOS {
		t.Errorf("expected type sos, got %s", alert.Type)
	}
	if alert.BatteryLevel != 85 {
		t.Errorf("expected battery_level 85, got %d", alert.BatteryLevel)
	}
	if alertRepo.alerts[alert.ID] == nil {
		t.Error("alert was not persisted")
	}
	if notifier.calls != 1 {
		t.Errorf("expected 1 notifier call, got %d", notifier.calls)
	}
	if len(notifier.tokens) != 3 {
		t.Errorf("expected 3 fcm tokens (obs1 x2 + obs2), got %d", len(notifier.tokens))
	}
	if notifier.lastP.Type != "sos" {
		t.Errorf("expected payload type sos, got %s", notifier.lastP.Type)
	}
	if notifier.lastP.Silent {
		t.Error("SOS push should not be silent")
	}
}

func TestCreateSOS_NoLinkedObservers_NoPush(t *testing.T) {
	contactRepo := &fakeContactRepo{contacts: []*contactDomain.Contact{
		{ID: "c1", UserID: "usr_victim"}, // sin vinculo
	}}
	alertRepo := &fakeAlertRepo{}
	deviceRepo := &fakeDeviceRepo{}
	notifier := &fakeNotifier{}
	uc := newTestUC(alertRepo, contactRepo, deviceRepo, notifier, &fakePinVerifier{})

	if _, err := uc.CreateSOS(context.Background(), "usr_victim", domain.SOSInput{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if notifier.calls != 0 {
		t.Errorf("expected no notifier calls, got %d", notifier.calls)
	}
}

func TestCreateCoercion_RealPin_ResolvesActiveAlert(t *testing.T) {
	alertRepo := &fakeAlertRepo{alerts: map[string]*domain.Alert{
		"alt_1": {ID: "alt_1", UserID: "usr_victim", Type: domain.AlertTypeSOS, Status: domain.AlertStatusActive},
	}}
	contactRepo := &fakeContactRepo{}
	deviceRepo := &fakeDeviceRepo{}
	notifier := &fakeNotifier{}
	uc := newTestUC(alertRepo, contactRepo, deviceRepo, notifier, &fakePinVerifier{result: userDomain.PinReal})

	alert, err := uc.CreateCoercion(context.Background(), "usr_victim", domain.CoercionInput{
		PIN: "1234",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if alert.Status != domain.AlertStatusResolved {
		t.Errorf("expected resolved status, got %s", alert.Status)
	}
	if notifier.calls != 0 {
		t.Errorf("real pin resolution should not notify observers, got %d calls", notifier.calls)
	}
}

func TestCreateCoercion_CoercionPin_EscalatesAndSilentPush(t *testing.T) {
	alertRepo := &fakeAlertRepo{alerts: map[string]*domain.Alert{
		"alt_1": {ID: "alt_1", UserID: "usr_victim", Type: domain.AlertTypeSOS, Status: domain.AlertStatusActive},
	}}
	contactRepo := &fakeContactRepo{contacts: []*contactDomain.Contact{
		{ID: "c1", UserID: "usr_victim", LinkedUserID: "usr_obs1", IsVerified: true},
	}}
	deviceRepo := &fakeDeviceRepo{devices: map[string][]*deviceDomain.Device{
		"obs1": {{ID: "dev_a", UserID: "obs1", FCMToken: "tok_a"}},
	}}
	notifier := &fakeNotifier{}
	uc := newTestUC(alertRepo, contactRepo, deviceRepo, notifier, &fakePinVerifier{result: userDomain.PinCoercion})

	alert, err := uc.CreateCoercion(context.Background(), "usr_victim", domain.CoercionInput{
		PIN: "9999",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if alert.ID != "alt_1" {
		t.Errorf("expected same alert id alt_1, got %s", alert.ID)
	}
	if alert.Type != domain.AlertTypeCoercion {
		t.Errorf("expected type escalated to coercion, got %s", alert.Type)
	}
	if alert.Status != domain.AlertStatusActive {
		t.Errorf("coercion must remain active, got %s", alert.Status)
	}
	if notifier.calls != 1 {
		t.Fatalf("expected 1 notifier call, got %d", notifier.calls)
	}
	if !notifier.lastP.Silent {
		t.Error("coercion push must be silent")
	}
	if notifier.lastP.ChannelID != "coercion_alerts" {
		t.Errorf("expected channel coercion_alerts, got %s", notifier.lastP.ChannelID)
	}
}

func TestCreateCoercion_InvalidPin_Rejected(t *testing.T) {
	alertRepo := &fakeAlertRepo{}
	contactRepo := &fakeContactRepo{}
	deviceRepo := &fakeDeviceRepo{}
	notifier := &fakeNotifier{}
	uc := newTestUC(alertRepo, contactRepo, deviceRepo, notifier, &fakePinVerifier{result: userDomain.PinNone})

	if _, err := uc.CreateCoercion(context.Background(), "usr_victim", domain.CoercionInput{PIN: "0000"}); err != usecase.ErrInvalidPin {
		t.Errorf("expected ErrInvalidPin, got %v", err)
	}
}

func TestCreateCoercion_Coercion_NoActiveAlert_CreatesNew(t *testing.T) {
	alertRepo := &fakeAlertRepo{}
	contactRepo := &fakeContactRepo{}
	deviceRepo := &fakeDeviceRepo{}
	notifier := &fakeNotifier{}
	uc := newTestUC(alertRepo, contactRepo, deviceRepo, notifier, &fakePinVerifier{result: userDomain.PinCoercion})

	alert, err := uc.CreateCoercion(context.Background(), "usr_victim", domain.CoercionInput{
		PIN:      "9999",
		Latitude: 19.43,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if alert.Type != domain.AlertTypeCoercion {
		t.Errorf("expected coercion type, got %s", alert.Type)
	}
	if alertRepo.alerts[alert.ID] == nil {
		t.Error("new coercion alert was not persisted")
	}
}

func TestResolveAlert_Success(t *testing.T) {
	alertRepo := &fakeAlertRepo{alerts: map[string]*domain.Alert{
		"alt_1": {ID: "alt_1", UserID: "usr_victim", Type: domain.AlertTypeSOS, Status: domain.AlertStatusActive},
	}}
	contactRepo := &fakeContactRepo{}
	deviceRepo := &fakeDeviceRepo{}
	notifier := &fakeNotifier{}
	uc := newTestUC(alertRepo, contactRepo, deviceRepo, notifier, &fakePinVerifier{})

	alert, err := uc.ResolveAlert(context.Background(), "alt_1", "usr_victim")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if alert.Status != domain.AlertStatusResolved {
		t.Errorf("expected resolved, got %s", alert.Status)
	}
	if alert.ResolvedAt == nil {
		t.Error("expected resolved_at to be set")
	}
}

func TestResolveAlert_DeniesNonOwner(t *testing.T) {
	alertRepo := &fakeAlertRepo{alerts: map[string]*domain.Alert{
		"alt_1": {ID: "alt_1", UserID: "usr_victim", Type: domain.AlertTypeSOS, Status: domain.AlertStatusActive},
	}}
	contactRepo := &fakeContactRepo{}
	deviceRepo := &fakeDeviceRepo{}
	notifier := &fakeNotifier{}
	uc := newTestUC(alertRepo, contactRepo, deviceRepo, notifier, &fakePinVerifier{})

	if _, err := uc.ResolveAlert(context.Background(), "alt_1", "usr_outsider"); err != usecase.ErrAlertNotFound {
		t.Errorf("expected ErrAlertNotFound, got %v", err)
	}
}

func TestGetObserving_QueriesLinkedVictims(t *testing.T) {
	linked := []*contactDomain.Contact{
		{ID: "c1", UserID: "usr_victim_a", LinkedUserID: "usr_obs", IsVerified: true},
	}
	alertRepo := &fakeAlertRepo{activeResult: []*domain.Alert{
		{ID: "alt_1", UserID: "usr_victim_a", Type: domain.AlertTypeSOS, Status: domain.AlertStatusActive},
	}}
	contactRepo := &fakeContactRepo{linkedResult: linked}
	deviceRepo := &fakeDeviceRepo{}
	notifier := &fakeNotifier{}
	uc := newTestUC(alertRepo, contactRepo, deviceRepo, notifier, &fakePinVerifier{})

	alerts, err := uc.GetObserving(context.Background(), "usr_obs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if contactRepo.linkedReceived != "usr_obs" {
		t.Errorf("expected linked query for usr_obs, got %s", contactRepo.linkedReceived)
	}
	if len(alerts) != 1 || alerts[0].ID != "alt_1" {
		t.Errorf("expected 1 active alert, got %+v", alerts)
	}
}

func TestGetByID_DeniesNonObserver(t *testing.T) {
	alertRepo := &fakeAlertRepo{alerts: map[string]*domain.Alert{
		"alt_1": {ID: "alt_1", UserID: "usr_victim", Type: domain.AlertTypeSOS, Status: domain.AlertStatusActive},
	}}
	contactRepo := &fakeContactRepo{linkedResult: nil} // el viewer no está vinculado
	deviceRepo := &fakeDeviceRepo{}
	notifier := &fakeNotifier{}
	uc := newTestUC(alertRepo, contactRepo, deviceRepo, notifier, &fakePinVerifier{})

	if _, err := uc.GetByID(context.Background(), "alt_1", "usr_outsider"); err == nil {
		t.Error("expected error for non-victim, non-observer, got nil")
	}
}

func TestCreateSOS_SkipsUnverifiedObservers(t *testing.T) {
	contacts := []*contactDomain.Contact{
		{ID: "c1", UserID: "usr_victim", LinkedUserID: "usr_obs1", IsVerified: true},
		{ID: "c2", UserID: "usr_victim", LinkedUserID: "usr_obs2", IsVerified: false}, // pendiente
	}
	devices := map[string][]*deviceDomain.Device{
		"obs1": {{ID: "dev_a", UserID: "obs1", FCMToken: "tok_a"}},
		"obs2": {{ID: "dev_b", UserID: "obs2", FCMToken: "tok_b"}},
	}

	alertRepo := &fakeAlertRepo{}
	contactRepo := &fakeContactRepo{contacts: contacts}
	deviceRepo := &fakeDeviceRepo{devices: devices}
	notifier := &fakeNotifier{}
	uc := newTestUC(alertRepo, contactRepo, deviceRepo, notifier, &fakePinVerifier{})

	if _, err := uc.CreateSOS(context.Background(), "usr_victim", domain.SOSInput{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if notifier.calls != 1 {
		t.Errorf("expected 1 notifier call (only verified observer), got %d", notifier.calls)
	}
	if len(notifier.tokens) != 1 || notifier.tokens[0] != "tok_a" {
		t.Errorf("expected only verified observer's token, got %v", notifier.tokens)
	}
}

func TestGetObserving_ExcludesUnverifiedVictims(t *testing.T) {
	linked := []*contactDomain.Contact{
		{ID: "c1", UserID: "usr_victim_a", LinkedUserID: "usr_obs", IsVerified: true},
		{ID: "c2", UserID: "usr_victim_b", LinkedUserID: "usr_obs", IsVerified: false}, // no aceptado
	}
	alertRepo := &fakeAlertRepo{activeResult: []*domain.Alert{
		{ID: "alt_1", UserID: "usr_victim_a", Type: domain.AlertTypeSOS, Status: domain.AlertStatusActive},
		{ID: "alt_2", UserID: "usr_victim_b", Type: domain.AlertTypeSOS, Status: domain.AlertStatusActive},
	}}
	contactRepo := &fakeContactRepo{linkedResult: linked}
	deviceRepo := &fakeDeviceRepo{}
	notifier := &fakeNotifier{}
	uc := newTestUC(alertRepo, contactRepo, deviceRepo, notifier, &fakePinVerifier{})

	alerts, err := uc.GetObserving(context.Background(), "usr_obs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(alertRepo.byUserIDs) != 1 || alertRepo.byUserIDs[0] != "usr_victim_a" {
		t.Errorf("expected only verified victim usr_victim_a in query, got %v", alertRepo.byUserIDs)
	}
	if len(alerts) != 1 || alerts[0].ID != "alt_1" {
		t.Errorf("expected 1 active alert from verified victim, got %+v", alerts)
	}
}

func TestGetByID_DeniesUnverifiedObserver(t *testing.T) {
	alertRepo := &fakeAlertRepo{alerts: map[string]*domain.Alert{
		"alt_1": {ID: "alt_1", UserID: "usr_victim", Type: domain.AlertTypeSOS, Status: domain.AlertStatusActive},
	}}
	// Observador vinculado pero NO verificado (pendiente de aceptación)
	contactRepo := &fakeContactRepo{linkedResult: []*contactDomain.Contact{
		{ID: "c1", UserID: "usr_victim", LinkedUserID: "usr_obs", IsVerified: false},
	}}
	deviceRepo := &fakeDeviceRepo{}
	notifier := &fakeNotifier{}
	uc := newTestUC(alertRepo, contactRepo, deviceRepo, notifier, &fakePinVerifier{})

	if _, err := uc.GetByID(context.Background(), "alt_1", "usr_obs"); err == nil {
		t.Error("expected error for unverified observer, got nil")
	}
}
