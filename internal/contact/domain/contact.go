package domain

import (
	"context"
	"time"
)

type Contact struct {
	ID           string    `json:"contact_id" bson:"_id"`
	UserID       string    `json:"user_id" bson:"user_id"`
	Name         string    `json:"name" bson:"name"`
	Phone        string    `json:"phone" bson:"phone"`
	Email        string    `json:"email" bson:"email"`
	Relationship string    `json:"relationship" bson:"relationship"`
	LinkedUserID string    `json:"linked_user_id,omitempty" bson:"linked_user_id,omitempty"`
	IsVerified   bool      `json:"is_verified" bson:"is_verified"`
	CreatedAt    time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" bson:"updated_at"`
}

type CreateContactInput struct {
	Name         string `json:"name"`
	Phone        string `json:"phone"`
	Email        string `json:"email"`
	Relationship string `json:"relationship"`
}

type UpdateContactInput struct {
	Name         *string `json:"name,omitempty"`
	Phone        *string `json:"phone,omitempty"`
	Email        *string `json:"email,omitempty"`
	Relationship *string `json:"relationship,omitempty"`
}

// ObservedVictim describe un vínculo verificado desde la perspectiva del
// observador: la víctima que lo agregó y cuyo rol de observación aceptó.
type ObservedVictim struct {
	ContactID  string    `json:"contact_id"`
	UserID     string    `json:"user_id"`
	Name       string    `json:"name"`
	Phone      string    `json:"phone"`
	IsVerified bool      `json:"is_verified"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ContactRepository interface {
	Create(ctx context.Context, contact *Contact) error
	FindByID(ctx context.Context, id string, userID string) (*Contact, error)
	FindAllByUserID(ctx context.Context, userID string) ([]*Contact, error)
	FindAllByLinkedUserID(ctx context.Context, linkedUserID string) ([]*Contact, error)
	FindAllPendingByLinkedUserID(ctx context.Context, linkedUserID string) ([]*Contact, error)
	FindByIDForLinkedUser(ctx context.Context, id string, linkedUserID string) (*Contact, error)
	SetVerified(ctx context.Context, contactID string, linkedUserID string, verified bool) (bool, error)
	UnlinkContact(ctx context.Context, contactID string, linkedUserID string) error
	Update(ctx context.Context, contact *Contact) error
	Delete(ctx context.Context, id string, userID string) error
	LinkContact(ctx context.Context, contactID string, userID string, linkedUserID string) error
	LinkPendingContacts(ctx context.Context, email string, phone string, userID string) error
}

type ContactUseCase interface {
	Create(ctx context.Context, userID string, input CreateContactInput) (*Contact, error)
	GetAll(ctx context.Context, userID string) ([]*Contact, error)
	Update(ctx context.Context, userID string, contactID string, input UpdateContactInput) (*Contact, error)
	Delete(ctx context.Context, userID string, contactID string) error
	Link(ctx context.Context, userID string, contactID string, linkedUserID string) (*Contact, error)
	GetPending(ctx context.Context, userID string) ([]*Contact, error)
	AcceptLink(ctx context.Context, contactID string, userID string) (*Contact, error)
	RejectLink(ctx context.Context, contactID string, userID string) error
	GetObserved(ctx context.Context, observerID string) ([]*ObservedVictim, error)
}
