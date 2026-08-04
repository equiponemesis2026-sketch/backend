package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/nemesis-project/api-nemesis/internal/contact/domain"
	notifDomain "github.com/nemesis-project/api-nemesis/internal/notifications/domain"
	deviceDomain "github.com/nemesis-project/api-nemesis/internal/token/domain"
	userDomain "github.com/nemesis-project/api-nemesis/internal/user/domain"
)

var (
	ErrContactNotFound = errors.New("contact not found")
	ErrNameRequired    = errors.New("name is required")
	ErrPhoneRequired   = errors.New("phone is required")
)

type contactUseCase struct {
	repo       domain.ContactRepository
	userRepo   userDomain.UserRepository
	notifier   notifDomain.PushNotifier
	deviceRepo deviceDomain.DeviceRepository
}

func NewContactUseCase(repo domain.ContactRepository, userRepo userDomain.UserRepository) domain.ContactUseCase {
	return &contactUseCase{repo: repo, userRepo: userRepo}
}

// SetSupportNotifier inyecta el notificador de red de apoyo (opcional). Permite
// avisar por push al usuario vinculado cuando alguien lo agrega como contacto
// de confianza para que acepte (o rechace) la solicitud de observación.
func (uc *contactUseCase) SetSupportNotifier(notifier notifDomain.PushNotifier, deviceRepo deviceDomain.DeviceRepository) {
	uc.notifier = notifier
	uc.deviceRepo = deviceRepo
}

func (uc *contactUseCase) Create(ctx context.Context, userID string, input domain.CreateContactInput) (*domain.Contact, error) {
	if input.Name == "" {
		return nil, ErrNameRequired
	}
	if input.Phone == "" {
		return nil, ErrPhoneRequired
	}

	contact := &domain.Contact{
		ID:           fmt.Sprintf("cnt_%s", uuid.New().String()),
		UserID:       userID,
		Name:         input.Name,
		Phone:        input.Phone,
		Email:        input.Email,
		Relationship: input.Relationship,
		IsVerified:   false,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	// Auto-link: si el email o teléfono coincide con un usuario registrado
	// en Némesis, se vincula automáticamente como observador. El vínculo
	// queda en estado pendiente (IsVerified=false): el usuario vinculado debe
	// aceptarlo explícitamente antes de poder observar alertas y telemetría.
	if linked, err := uc.findLinkedUser(ctx, input.Email, input.Phone); err != nil {
		return nil, fmt.Errorf("failed to resolve linked user: %w", err)
	} else if linked != "" {
		contact.LinkedUserID = linked
	}

	if err := uc.repo.Create(ctx, contact); err != nil {
		return nil, fmt.Errorf("failed to persist contact: %w", err)
	}

	if contact.LinkedUserID != "" {
		uc.notifyContactRequest(ctx, contact)
	}

	return contact, nil
}

// notifyContactRequest avisa por push al usuario vinculado que debe aceptar
// (o rechazar) la solicitud para convertirse en observador. Es best-effort:
// un fallo de notificación nunca debe impedir guardar el contacto.
func (uc *contactUseCase) notifyContactRequest(ctx context.Context, contact *domain.Contact) {
	if uc.notifier == nil || uc.deviceRepo == nil {
		return
	}

	observerID := strings.TrimPrefix(contact.LinkedUserID, "usr_")
	devices, err := uc.deviceRepo.FindAllDevicesByUserID(ctx, observerID)
	if err != nil {
		slog.Warn("contact: failed to fetch observer devices", "contact_id", contact.ID, "error", err)
		return
	}

	var targets []notifDomain.DeviceTarget
	for _, d := range devices {
		if d.FCMToken == "" {
			continue
		}
		targets = append(targets, notifDomain.DeviceTarget{Token: d.FCMToken, DeviceID: d.ID})
	}
	if len(targets) == 0 {
		return
	}

	payload := notifDomain.PushPayload{
		Title:   "Solicitud de red de apoyo",
		Body:    fmt.Sprintf("%s quiere que seas su contacto de confianza. Acéptalo para ver sus alertas.", contact.Name),
		AlertID: "",
		Type:    "contact_request",
	}

	if err := uc.notifier.SendCriticalPush(ctx, payload, targets); err != nil {
		slog.Warn("contact: failed to notify contact request", "contact_id", contact.ID, "error", err)
	}
}

// findLinkedUser busca un usuario registrado por email o teléfono.
func (uc *contactUseCase) findLinkedUser(ctx context.Context, email string, phone string) (string, error) {
	if email != "" {
		u, err := uc.userRepo.FindByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
		if err != nil {
			return "", err
		}
		if u != nil {
			return u.ID, nil
		}
	}
	if phone != "" {
		u, err := uc.userRepo.FindByPhone(ctx, strings.TrimSpace(phone))
		if err != nil {
			return "", err
		}
		if u != nil {
			return u.ID, nil
		}
	}
	return "", nil
}

func (uc *contactUseCase) Link(ctx context.Context, userID string, contactID string, linkedUserID string) (*domain.Contact, error) {
	contact, err := uc.repo.FindByID(ctx, contactID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch contact: %w", err)
	}
	if contact == nil {
		return nil, ErrContactNotFound
	}

	if err := uc.repo.LinkContact(ctx, contactID, userID, linkedUserID); err != nil {
		return nil, fmt.Errorf("failed to link contact: %w", err)
	}

	contact.LinkedUserID = linkedUserID
	contact.UpdatedAt = time.Now().UTC()
	return contact, nil
}

func (uc *contactUseCase) GetAll(ctx context.Context, userID string) ([]*domain.Contact, error) {
	contacts, err := uc.repo.FindAllByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch contacts: %w", err)
	}
	return contacts, nil
}

func (uc *contactUseCase) Update(ctx context.Context, userID string, contactID string, input domain.UpdateContactInput) (*domain.Contact, error) {
	contact, err := uc.repo.FindByID(ctx, contactID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch contact: %w", err)
	}
	if contact == nil {
		return nil, ErrContactNotFound
	}

	if input.Name != nil {
		contact.Name = *input.Name
	}
	if input.Phone != nil {
		contact.Phone = *input.Phone
	}
	if input.Email != nil {
		contact.Email = *input.Email
	}
	if input.Relationship != nil {
		contact.Relationship = *input.Relationship
	}
	contact.UpdatedAt = time.Now().UTC()

	if err := uc.repo.Update(ctx, contact); err != nil {
		return nil, fmt.Errorf("failed to update contact: %w", err)
	}

	return contact, nil
}

func (uc *contactUseCase) Delete(ctx context.Context, userID string, contactID string) error {
	contact, err := uc.repo.FindByID(ctx, contactID, userID)
	if err != nil {
		return fmt.Errorf("failed to fetch contact: %w", err)
	}
	if contact == nil {
		return ErrContactNotFound
	}

	if err := uc.repo.Delete(ctx, contactID, userID); err != nil {
		return fmt.Errorf("failed to delete contact: %w", err)
	}

	return nil
}

// GetPending lista las solicitudes de vínculo pendientes de aceptación para
// el usuario observador (contactos que lo referencian sin estar verificados).
func (uc *contactUseCase) GetPending(ctx context.Context, userID string) ([]*domain.Contact, error) {
	contacts, err := uc.repo.FindAllPendingByLinkedUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pending contacts: %w", err)
	}
	return contacts, nil
}

// AcceptLink marca como verificado un contacto pendiente. Solo el usuario
// vinculado (linked_user_id) puede aceptar; verifica la pertenencia en el repo.
func (uc *contactUseCase) AcceptLink(ctx context.Context, contactID string, userID string) (*domain.Contact, error) {
	matched, err := uc.repo.SetVerified(ctx, contactID, userID, true)
	if err != nil {
		return nil, fmt.Errorf("failed to verify contact: %w", err)
	}
	if !matched {
		return nil, ErrContactNotFound
	}

	contact, err := uc.repo.FindByIDForLinkedUser(ctx, contactID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch contact: %w", err)
	}
	if contact == nil {
		return nil, ErrContactNotFound
	}
	contact.IsVerified = true
	contact.UpdatedAt = time.Now().UTC()
	return contact, nil
}

// RejectLink desvincula al usuario de un contacto pendiente sin eliminar el
// registro de la víctima. Solo el usuario vinculado puede rechazar.
func (uc *contactUseCase) RejectLink(ctx context.Context, contactID string, userID string) error {
	if err := uc.repo.UnlinkContact(ctx, contactID, userID); err != nil {
		return fmt.Errorf("failed to unlink contact: %w", err)
	}
	return nil
}

// GetObserved lista las víctimas a las que el observador está vinculado y
// verificado (aceptó la solicitud). Incluye el nombre real de la víctima para
// mostrarlo en el panel del observador. Se deduplica por víctima.
func (uc *contactUseCase) GetObserved(ctx context.Context, observerID string) ([]*domain.ObservedVictim, error) {
	contacts, err := uc.repo.FindAllByLinkedUserID(ctx, observerID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch observed contacts: %w", err)
	}

	seen := make(map[string]struct{})
	victims := []*domain.ObservedVictim{}
	for _, c := range contacts {
		if !c.IsVerified {
			continue
		}
		if _, dup := seen[c.UserID]; dup {
			continue
		}
		seen[c.UserID] = struct{}{}

		name := c.Name
		if u, err := uc.userRepo.FindByID(ctx, c.UserID); err == nil && u != nil && u.Name != "" {
			name = u.Name
		}

		victims = append(victims, &domain.ObservedVictim{
			ContactID:  c.ID,
			UserID:     c.UserID,
			Name:       name,
			Phone:      c.Phone,
			IsVerified: true,
			CreatedAt:  c.CreatedAt,
			UpdatedAt:  c.UpdatedAt,
		})
	}

	return victims, nil
}
