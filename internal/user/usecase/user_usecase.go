package usecase

import (
	"context"
	"github.com/nemesis-back/internal/user/domain"
)

type userUseCase struct {
	userRepo domain.UserRepository
}

func NewUserUseCase(repo domain.UserRepository) domain.UserUseCase {
	return &userUseCase{
		userRepo: repo,
	}
}

func (u *userUseCase) Register(ctx context.Context, user *domain.User, password string) error {
	// TODO: Hash password and store user
	return nil
}

func (u *userUseCase) Authenticate(ctx context.Context, email, password string) (string, error) {
	// TODO: Authenticate user and return signed JWT token
	return "", nil
}
