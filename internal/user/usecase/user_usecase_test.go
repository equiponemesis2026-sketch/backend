package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/nemesis-project/api-nemesis/internal/user/domain"
	"github.com/nemesis-project/api-nemesis/internal/user/usecase"
)

type mockUserRepo struct {
	users map[string]*domain.User
}

func (m *mockUserRepo) Create(_ context.Context, u *domain.User) error {
	if m.users == nil {
		m.users = make(map[string]*domain.User)
	}
	copy := *u
	m.users[u.Email] = &copy
	return nil
}

func (m *mockUserRepo) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	if m.users == nil {
		return nil, nil
	}
	u, ok := m.users[email]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func (m *mockUserRepo) FindByPhone(_ context.Context, _ string) (*domain.User, error) {
	return nil, nil
}

func (m *mockUserRepo) FindByID(_ context.Context, id string) (*domain.User, error) {
	if m.users == nil {
		return nil, nil
	}
	for _, u := range m.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, nil
}

func newTestUseCase() (domain.UserUseCase, *mockUserRepo) {
	repo := &mockUserRepo{}
	uc := usecase.NewUserUseCase(repo, "test-secret", 1*time.Hour)
	return uc, repo
}

func TestRegister_Success(t *testing.T) {
	uc, _ := newTestUseCase()

	input := domain.RegisterInput{
		Name:     "Test User",
		Email:    "test@example.com",
		Password: "securepass123",
		Phone:    "555-0100",
	}

	user, err := uc.Register(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if user.ID == "" {
		t.Error("expected non-empty user ID")
	}
	if user.Name != input.Name {
		t.Errorf("expected name %q, got %q", input.Name, user.Name)
	}
	if user.Email != input.Email {
		t.Errorf("expected email %q, got %q", input.Email, user.Email)
	}
	if user.Phone != input.Phone {
		t.Errorf("expected phone %q, got %q", input.Phone, user.Phone)
	}
	if user.Password != "" {
		t.Error("password should be empty in response")
	}
	if user.CreatedAt.IsZero() {
		t.Error("expected non-zero created_at")
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	uc, _ := newTestUseCase()

	input := domain.RegisterInput{
		Name:     "Test User",
		Email:    "dupe@example.com",
		Password: "securepass123",
		Phone:    "555-0100",
	}

	_, err := uc.Register(context.Background(), input)
	if err != nil {
		t.Fatalf("first register should succeed: %v", err)
	}

	_, err = uc.Register(context.Background(), input)
	if err != usecase.ErrUserAlreadyExists {
		t.Errorf("expected ErrUserAlreadyExists, got %v", err)
	}
}

func TestLogin_Success(t *testing.T) {
	uc, _ := newTestUseCase()

	regInput := domain.RegisterInput{
		Name:     "Test User",
		Email:    "login@example.com",
		Password: "securepass123",
		Phone:    "555-0100",
	}

	_, err := uc.Register(context.Background(), regInput)
	if err != nil {
		t.Fatalf("register should succeed: %v", err)
	}

	tokenResp, err := uc.Login(context.Background(), domain.LoginInput{
		Email:    "login@example.com",
		Password: "securepass123",
	})
	if err != nil {
		t.Fatalf("login should succeed: %v", err)
	}

	if tokenResp.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
	if tokenResp.TokenType != "Bearer" {
		t.Errorf("expected token type Bearer, got %q", tokenResp.TokenType)
	}
	if tokenResp.ExpiresIn <= 0 {
		t.Error("expected positive expires_in")
	}
	if tokenResp.User.Email != "login@example.com" {
		t.Errorf("expected email login@example.com, got %q", tokenResp.User.Email)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	uc, _ := newTestUseCase()

	regInput := domain.RegisterInput{
		Name:     "Test User",
		Email:    "wrongpw@example.com",
		Password: "securepass123",
		Phone:    "555-0100",
	}

	_, err := uc.Register(context.Background(), regInput)
	if err != nil {
		t.Fatalf("register should succeed: %v", err)
	}

	_, err = uc.Login(context.Background(), domain.LoginInput{
		Email:    "wrongpw@example.com",
		Password: "wrongpassword",
	})
	if err != usecase.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	uc, _ := newTestUseCase()

	_, err := uc.Login(context.Background(), domain.LoginInput{
		Email:    "nonexistent@example.com",
		Password: "somepassword",
	})
	if err != usecase.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}
