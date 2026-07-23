package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/nemesis-project/api-nemesis/internal/contact/domain"
)

var (
	ErrContactNotFound = errors.New("contact not found")
	ErrNameRequired    = errors.New("name is required")
	ErrPhoneRequired   = errors.New("phone is required")
)

type contactUseCase struct {
	repo domain.ContactRepository
}

func NewContactUseCase(repo domain.ContactRepository) domain.ContactUseCase {
	return &contactUseCase{repo: repo}
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

	if err := uc.repo.Create(ctx, contact); err != nil {
		return nil, fmt.Errorf("failed to persist contact: %w", err)
	}

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
