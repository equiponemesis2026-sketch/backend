package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/nemesis-project/api-nemesis/internal/contact/domain"
	userDomain "github.com/nemesis-project/api-nemesis/internal/user/domain"
)

var (
	ErrContactNotFound = errors.New("contact not found")
	ErrNameRequired    = errors.New("name is required")
	ErrPhoneRequired   = errors.New("phone is required")
)

type contactUseCase struct {
	repo     domain.ContactRepository
	userRepo userDomain.UserRepository
}

func NewContactUseCase(repo domain.ContactRepository, userRepo userDomain.UserRepository) domain.ContactUseCase {
	return &contactUseCase{repo: repo, userRepo: userRepo}
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
	// en Némesis, se vincula automáticamente como observador.
	if linked, err := uc.findLinkedUser(ctx, input.Email, input.Phone); err != nil {
		return nil, fmt.Errorf("failed to resolve linked user: %w", err)
	} else if linked != "" {
		contact.LinkedUserID = linked
	}

	if err := uc.repo.Create(ctx, contact); err != nil {
		return nil, fmt.Errorf("failed to persist contact: %w", err)
	}

	return contact, nil
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
