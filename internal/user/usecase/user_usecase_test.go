package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

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

func (m *mockUserRepo) UpdateSecurityPins(_ context.Context, userID string, realHash string, coercionHash string) error {
	if m.users == nil {
		return nil
	}
	for _, u := range m.users {
		if u.ID == userID {
			u.RealPINHash = realHash
			u.CoercionPINHash = coercionHash
		}
	}
	return nil
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

func TestRegister_DefaultRoleIsVictim(t *testing.T) {
	uc, _ := newTestUseCase()

	user, err := uc.Register(context.Background(), domain.RegisterInput{
		Name:     "Test User",
		Email:    "default-role@example.com",
		Password: "securepass123",
		Phone:    "555-0100",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Role != domain.RoleVictim {
		t.Errorf("expected default role victim, got %q", user.Role)
	}
}

func TestRegister_ExplicitRoleObserver(t *testing.T) {
	uc, _ := newTestUseCase()

	user, err := uc.Register(context.Background(), domain.RegisterInput{
		Name:     "Observer User",
		Email:    "observer@example.com",
		Password: "securepass123",
		Phone:    "555-0100",
		Role:     domain.RoleObserver,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Role != domain.RoleObserver {
		t.Errorf("expected role observer, got %q", user.Role)
	}
}

func TestRegister_InvalidRoleRejected(t *testing.T) {
	uc, _ := newTestUseCase()

	_, err := uc.Register(context.Background(), domain.RegisterInput{
		Name:     "Bad Role",
		Email:    "badrole@example.com",
		Password: "securepass123",
		Phone:    "555-0100",
		Role:     "admin",
	})
	if !errors.Is(err, usecase.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for invalid role, got %v", err)
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
	if tokenResp.User.Role != domain.RoleVictim {
		t.Errorf("expected default role victim in response, got %q", tokenResp.User.Role)
	}

	// El claim "role" debe estar presente en el JWT firmado.
	claims := jwt.MapClaims{}
	_, _, err = jwt.NewParser().ParseUnverified(tokenResp.AccessToken, claims)
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}
	roleClaim, _ := claims["role"].(string)
	if roleClaim != string(domain.RoleVictim) {
		t.Errorf("expected role claim victim, got %q", roleClaim)
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

func TestSetSecurityPins_Success(t *testing.T) {
	uc, repo := newTestUseCase()

	_, err := uc.Register(context.Background(), domain.RegisterInput{
		Name:     "Test User",
		Email:    "pins@example.com",
		Password: "securepass123",
		Phone:    "555-0100",
	})
	if err != nil {
		t.Fatalf("register should succeed: %v", err)
	}

	user := repo.users["pins@example.com"]

	err = uc.SetSecurityPins(context.Background(), user.ID, domain.SecurityPinsInput{
		RealPIN:     "1234",
		CoercionPIN: "9999",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if user.RealPINHash == "" || user.CoercionPINHash == "" {
		t.Fatal("expected both pin hashes to be persisted")
	}
	if user.RealPINHash == "1234" || user.CoercionPINHash == "9999" {
		t.Error("pins must be stored hashed, not plaintext")
	}
}

func TestSetSecurityPins_SamePinsRejected(t *testing.T) {
	uc, _ := newTestUseCase()

	err := uc.SetSecurityPins(context.Background(), "usr_1", domain.SecurityPinsInput{
		RealPIN:     "1234",
		CoercionPIN: "1234",
	})
	if err == nil {
		t.Error("expected error when real and coercion pins are equal")
	}
}

func TestVerifyPin_RealAndCoercion(t *testing.T) {
	uc, repo := newTestUseCase()

	_, err := uc.Register(context.Background(), domain.RegisterInput{
		Name:     "Test User",
		Email:    "verify@example.com",
		Password: "securepass123",
		Phone:    "555-0100",
	})
	if err != nil {
		t.Fatalf("register should succeed: %v", err)
	}

	user := repo.users["verify@example.com"]

	err = uc.SetSecurityPins(context.Background(), user.ID, domain.SecurityPinsInput{
		RealPIN:     "1234",
		CoercionPIN: "9999",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	match, err := uc.VerifyPin(context.Background(), user.ID, "1234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if match != domain.PinReal {
		t.Errorf("expected PinReal for 1234, got %v", match)
	}

	match, err = uc.VerifyPin(context.Background(), user.ID, "9999")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if match != domain.PinCoercion {
		t.Errorf("expected PinCoercion for 9999, got %v", match)
	}

	match, err = uc.VerifyPin(context.Background(), user.ID, "0000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if match != domain.PinNone {
		t.Errorf("expected PinNone for 0000, got %v", match)
	}
}

func TestVerifyPin_UserWithoutPins(t *testing.T) {
	uc, repo := newTestUseCase()

	_, err := uc.Register(context.Background(), domain.RegisterInput{
		Name:     "Test User",
		Email:    "nopins@example.com",
		Password: "securepass123",
		Phone:    "555-0100",
	})
	if err != nil {
		t.Fatalf("register should succeed: %v", err)
	}

	user := repo.users["nopins@example.com"]

	match, err := uc.VerifyPin(context.Background(), user.ID, "1234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if match != domain.PinNone {
		t.Errorf("expected PinNone, got %v", match)
	}
}
