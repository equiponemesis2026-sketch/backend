package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/nemesis-project/api-nemesis/internal/contact/domain"
	"github.com/nemesis-project/api-nemesis/internal/contact/usecase"
	userDomain "github.com/nemesis-project/api-nemesis/internal/user/domain"
)

type mockContactRepo struct {
	contacts map[string]*domain.Contact
}

func (m *mockContactRepo) Create(_ context.Context, c *domain.Contact) error {
	if m.contacts == nil {
		m.contacts = make(map[string]*domain.Contact)
	}
	m.contacts[c.ID] = c
	return nil
}

func (m *mockContactRepo) FindByID(_ context.Context, id, userID string) (*domain.Contact, error) {
	if m.contacts == nil {
		return nil, nil
	}
	c, ok := m.contacts[id]
	if !ok || c.UserID != userID {
		return nil, nil
	}
	return c, nil
}

func (m *mockContactRepo) FindAllByUserID(_ context.Context, userID string) ([]*domain.Contact, error) {
	if m.contacts == nil {
		return []*domain.Contact{}, nil
	}
	var result []*domain.Contact
	for _, c := range m.contacts {
		if c.UserID == userID {
			result = append(result, c)
		}
	}
	return result, nil
}

func (m *mockContactRepo) FindAllByLinkedUserID(_ context.Context, linkedUserID string) ([]*domain.Contact, error) {
	if m.contacts == nil {
		return []*domain.Contact{}, nil
	}
	var result []*domain.Contact
	for _, c := range m.contacts {
		if c.LinkedUserID == linkedUserID {
			result = append(result, c)
		}
	}
	return result, nil
}

func (m *mockContactRepo) Update(_ context.Context, c *domain.Contact) error {
	if m.contacts == nil {
		return nil
	}
	m.contacts[c.ID] = c
	return nil
}

func (m *mockContactRepo) Delete(_ context.Context, id, userID string) error {
	if m.contacts == nil {
		return nil
	}
	c, ok := m.contacts[id]
	if ok && c.UserID == userID {
		delete(m.contacts, id)
	}
	return nil
}

func (m *mockContactRepo) LinkContact(_ context.Context, contactID, userID, linkedUserID string) error {
	if m.contacts == nil {
		return nil
	}
	c, ok := m.contacts[contactID]
	if !ok || c.UserID != userID {
		return nil
	}
	c.LinkedUserID = linkedUserID
	return nil
}

func (m *mockContactRepo) LinkPendingContacts(_ context.Context, email, phone, userID string) error {
	if m.contacts == nil {
		return nil
	}
	for _, c := range m.contacts {
		if c.LinkedUserID == "" && (c.Email == email || c.Phone == phone) {
			c.LinkedUserID = userID
		}
	}
	return nil
}

type mockUserRepo struct {
	byEmail map[string]string
	byPhone map[string]string
}

func (m *mockUserRepo) Create(_ context.Context, _ *userDomain.User) error { return nil }

func (m *mockUserRepo) FindByEmail(_ context.Context, email string) (*userDomain.User, error) {
	if m == nil || m.byEmail == nil {
		return nil, nil
	}
	if id, ok := m.byEmail[email]; ok {
		return &userDomain.User{ID: id}, nil
	}
	return nil, nil
}

func (m *mockUserRepo) FindByPhone(_ context.Context, phone string) (*userDomain.User, error) {
	if m == nil || m.byPhone == nil {
		return nil, nil
	}
	if id, ok := m.byPhone[phone]; ok {
		return &userDomain.User{ID: id}, nil
	}
	return nil, nil
}

func (m *mockUserRepo) FindByID(_ context.Context, _ string) (*userDomain.User, error) {
	return nil, nil
}

func newTestContactUseCase() (domain.ContactUseCase, *mockContactRepo, *mockUserRepo) {
	repo := &mockContactRepo{}
	userRepo := &mockUserRepo{}
	uc := usecase.NewContactUseCase(repo, userRepo)
	return uc, repo, userRepo
}

func TestCreateContact_Success(t *testing.T) {
	uc, _, _ := newTestContactUseCase()

	input := domain.CreateContactInput{
		Name:         "Maria",
		Phone:        "555-0200",
		Email:        "maria@example.com",
		Relationship: "familiar",
	}

	contact, err := uc.Create(context.Background(), "usr_123", input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if contact.ID == "" {
		t.Error("expected non-empty contact ID")
	}
	if contact.UserID != "usr_123" {
		t.Errorf("expected user_id usr_123, got %q", contact.UserID)
	}
	if contact.Name != input.Name {
		t.Errorf("expected name %q, got %q", input.Name, contact.Name)
	}
	if contact.Phone != input.Phone {
		t.Errorf("expected phone %q, got %q", input.Phone, contact.Phone)
	}
	if contact.Relationship != input.Relationship {
		t.Errorf("expected relationship %q, got %q", input.Relationship, contact.Relationship)
	}
	if contact.IsVerified {
		t.Error("new contact should not be verified")
	}
	if contact.CreatedAt.IsZero() {
		t.Error("expected non-zero created_at")
	}
}

func TestCreateContact_MissingName(t *testing.T) {
	uc, _, _ := newTestContactUseCase()

	_, err := uc.Create(context.Background(), "usr_123", domain.CreateContactInput{
		Name:  "",
		Phone: "555-0200",
	})
	if err != usecase.ErrNameRequired {
		t.Errorf("expected ErrNameRequired, got %v", err)
	}
}

func TestCreateContact_MissingPhone(t *testing.T) {
	uc, _, _ := newTestContactUseCase()

	_, err := uc.Create(context.Background(), "usr_123", domain.CreateContactInput{
		Name:  "Maria",
		Phone: "",
	})
	if err != usecase.ErrPhoneRequired {
		t.Errorf("expected ErrPhoneRequired, got %v", err)
	}
}

func TestGetAllContacts_Success(t *testing.T) {
	uc, repo, _ := newTestContactUseCase()

	contact1 := &domain.Contact{ID: "cnt_001", UserID: "usr_123", Name: "Maria", Phone: "555-0200"}
	contact2 := &domain.Contact{ID: "cnt_002", UserID: "usr_123", Name: "Juan", Phone: "555-0300"}
	if err := repo.Create(context.Background(), contact1); err != nil {
		t.Fatalf("failed to seed contact1: %v", err)
	}
	if err := repo.Create(context.Background(), contact2); err != nil {
		t.Fatalf("failed to seed contact2: %v", err)
	}

	contacts, err := uc.GetAll(context.Background(), "usr_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(contacts) != 2 {
		t.Fatalf("expected 2 contacts, got %d", len(contacts))
	}
}

func TestGetAllContacts_Empty(t *testing.T) {
	uc, _, _ := newTestContactUseCase()

	contacts, err := uc.GetAll(context.Background(), "usr_999")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(contacts) != 0 {
		t.Errorf("expected empty slice, got %d items", len(contacts))
	}
}

func TestGetAllContacts_ScopedByUserID(t *testing.T) {
	uc, repo, _ := newTestContactUseCase()

	if err := repo.Create(context.Background(), &domain.Contact{ID: "cnt_001", UserID: "usr_a", Name: "A"}); err != nil {
		t.Fatalf("failed to seed contact: %v", err)
	}
	if err := repo.Create(context.Background(), &domain.Contact{ID: "cnt_002", UserID: "usr_b", Name: "B"}); err != nil {
		t.Fatalf("failed to seed contact: %v", err)
	}

	contacts, err := uc.GetAll(context.Background(), "usr_a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(contacts) != 1 || contacts[0].Name != "A" {
		t.Errorf("expected 1 contact for usr_a, got %d", len(contacts))
	}
}

func TestUpdateContact_Success(t *testing.T) {
	uc, repo, _ := newTestContactUseCase()

	if err := repo.Create(context.Background(), &domain.Contact{
		ID: "cnt_001", UserID: "usr_123", Name: "Maria",
		Phone: "555-0200", Relationship: "familiar", CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("failed to seed contact: %v", err)
	}

	name := "Maria Actualizada"
	rel := "amiga"
	updated, err := uc.Update(context.Background(), "usr_123", "cnt_001", domain.UpdateContactInput{
		Name:         &name,
		Relationship: &rel,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updated.Name != "Maria Actualizada" {
		t.Errorf("expected name Maria Actualizada, got %q", updated.Name)
	}
	if updated.Relationship != "amiga" {
		t.Errorf("expected relationship amiga, got %q", updated.Relationship)
	}
	if updated.Phone != "555-0200" {
		t.Errorf("phone should remain unchanged, got %q", updated.Phone)
	}
}

func TestUpdateContact_NotFound(t *testing.T) {
	uc, _, _ := newTestContactUseCase()

	name := "New Name"
	_, err := uc.Update(context.Background(), "usr_123", "cnt_nonexistent", domain.UpdateContactInput{
		Name: &name,
	})
	if err != usecase.ErrContactNotFound {
		t.Errorf("expected ErrContactNotFound, got %v", err)
	}
}

func TestDeleteContact_Success(t *testing.T) {
	uc, repo, _ := newTestContactUseCase()

	if err := repo.Create(context.Background(), &domain.Contact{ID: "cnt_001", UserID: "usr_123", Name: "Maria", Phone: "555-0200"}); err != nil {
		t.Fatalf("failed to seed contact: %v", err)
	}

	err := uc.Delete(context.Background(), "usr_123", "cnt_001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	contacts, _ := uc.GetAll(context.Background(), "usr_123")
	if len(contacts) != 0 {
		t.Error("expected empty contact list after delete")
	}
}

func TestDeleteContact_NotFound(t *testing.T) {
	uc, _, _ := newTestContactUseCase()

	err := uc.Delete(context.Background(), "usr_123", "cnt_nonexistent")
	if err != usecase.ErrContactNotFound {
		t.Errorf("expected ErrContactNotFound, got %v", err)
	}
}

func TestDeleteContact_WrongUser(t *testing.T) {
	uc, repo, _ := newTestContactUseCase()

	if err := repo.Create(context.Background(), &domain.Contact{ID: "cnt_001", UserID: "usr_a", Name: "Maria", Phone: "555-0200"}); err != nil {
		t.Fatalf("failed to seed contact: %v", err)
	}

	err := uc.Delete(context.Background(), "usr_b", "cnt_001")
	if err != usecase.ErrContactNotFound {
		t.Errorf("expected ErrContactNotFound when wrong user tries to delete, got %v", err)
	}
}
