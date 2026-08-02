package usecase_test

import (
	"context"
	"sync"
	"testing"

	"github.com/nemesis-project/api-nemesis/internal/alert/domain"
	"github.com/nemesis-project/api-nemesis/internal/alert/usecase"
	contactDomain "github.com/nemesis-project/api-nemesis/internal/contact/domain"
	notifDomain "github.com/nemesis-project/api-nemesis/internal/notifications/domain"
	deviceDomain "github.com/nemesis-project/api-nemesis/internal/token/domain"
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

func (f *fakeAlertRepo) FindActiveByUserIDs(_ context.Context, userIDs []string) ([]*domain.Alert, error) {
	f.byUserIDs = userIDs
	return f.activeResult, nil
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

func TestCreateAlert_NotifiesLinkedObservers(t *testing.T) {
	contacts := []*contactDomain.Contact{
		{ID: "c1", UserID: "usr_victim", LinkedUserID: "usr_obs1"},
		{ID: "c2", UserID: "usr_victim", LinkedUserID: "usr_obs2"},
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
	uc := usecase.NewAlertUseCase(alertRepo, contactRepo, deviceRepo, notifier)

	alert, err := uc.CreateAlert(context.Background(), "usr_victim", domain.CreateAlertInput{
		Type:      domain.AlertTypeSOS,
		Latitude:  19.4326,
		Longitude: -99.1332,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if alert == nil {
		t.Fatal("expected alert, got nil")
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
}

func TestCreateAlert_NoLinkedObservers_NoPush(t *testing.T) {
	contactRepo := &fakeContactRepo{contacts: []*contactDomain.Contact{
		{ID: "c1", UserID: "usr_victim"}, // sin vinculo
	}}
	alertRepo := &fakeAlertRepo{}
	deviceRepo := &fakeDeviceRepo{}
	notifier := &fakeNotifier{}
	uc := usecase.NewAlertUseCase(alertRepo, contactRepo, deviceRepo, notifier)

	if _, err := uc.CreateAlert(context.Background(), "usr_victim", domain.CreateAlertInput{
		Type: domain.AlertTypeSOS,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if notifier.calls != 0 {
		t.Errorf("expected no notifier calls, got %d", notifier.calls)
	}
}

func TestGetObserving_QueriesLinkedVictims(t *testing.T) {
	linked := []*contactDomain.Contact{
		{ID: "c1", UserID: "usr_victim_a", LinkedUserID: "usr_obs"},
	}
	alertRepo := &fakeAlertRepo{activeResult: []*domain.Alert{
		{ID: "alt_1", UserID: "usr_victim_a", Type: domain.AlertTypeSOS, Status: domain.AlertStatusActive},
	}}
	contactRepo := &fakeContactRepo{linkedResult: linked}
	deviceRepo := &fakeDeviceRepo{}
	notifier := &fakeNotifier{}
	uc := usecase.NewAlertUseCase(alertRepo, contactRepo, deviceRepo, notifier)

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
	uc := usecase.NewAlertUseCase(alertRepo, contactRepo, deviceRepo, notifier)

	if _, err := uc.GetByID(context.Background(), "alt_1", "usr_outsider"); err == nil {
		t.Error("expected error for non-victim, non-observer, got nil")
	}
}
