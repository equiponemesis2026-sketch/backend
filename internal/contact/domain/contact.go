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

type ContactRepository interface {
	Create(ctx context.Context, contact *Contact) error
	FindByID(ctx context.Context, id string, userID string) (*Contact, error)
	FindAllByUserID(ctx context.Context, userID string) ([]*Contact, error)
	Update(ctx context.Context, contact *Contact) error
	Delete(ctx context.Context, id string, userID string) error
}

type ContactUseCase interface {
	Create(ctx context.Context, userID string, input CreateContactInput) (*Contact, error)
	GetAll(ctx context.Context, userID string) ([]*Contact, error)
	Update(ctx context.Context, userID string, contactID string, input UpdateContactInput) (*Contact, error)
	Delete(ctx context.Context, userID string, contactID string) error
}
