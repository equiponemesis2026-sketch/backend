package domain

import "context"

type User struct {
	ID           string   `json:"id" bson:"_id,omitempty"`
	Email        string   `json:"email" bson:"email"`
	PasswordHash string   `json:"-" bson:"password_hash"`
	Role         string   `json:"role" bson:"role"` // "VICTIM", "OBSERVER", "ADMIN"
	CoercionPIN  string   `json:"-" bson:"coercion_pin"`
	RealPIN      string   `json:"-" bson:"real_pin"`
	PublicKey    string   `json:"public_key" bson:"public_key"` // Zero-Trust: public key for client-side decryption
	Observers    []string `json:"observers" bson:"observers"`     // List of Observer User IDs
	CreatedAt    int64    `json:"created_at" bson:"created_at"`
}

type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Update(ctx context.Context, user *User) error
}

type UserUseCase interface {
	Register(ctx context.Context, user *User, password string) error
	Authenticate(ctx context.Context, email, password string) (string, error) // Returns JWT token
}
