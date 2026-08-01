package integration_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"

	contactHttp "github.com/nemesis-project/api-nemesis/internal/contact/delivery/http"
	contactDomain "github.com/nemesis-project/api-nemesis/internal/contact/domain"
	contactUsecase "github.com/nemesis-project/api-nemesis/internal/contact/usecase"
	"github.com/nemesis-project/api-nemesis/internal/infrastructure/middleware"
	userDomain "github.com/nemesis-project/api-nemesis/internal/user/domain"
	userHttp "github.com/nemesis-project/api-nemesis/internal/user/delivery/http"
	"github.com/nemesis-project/api-nemesis/internal/user/usecase"
)

// --- Mock: UserRepository ---

type mockUserRepo struct {
	users map[string]*userDomain.User
}

func (m *mockUserRepo) Create(_ context.Context, u *userDomain.User) error {
	if m.users == nil {
		m.users = make(map[string]*userDomain.User)
	}
	copy := *u
	m.users[u.Email] = &copy
	return nil
}

func (m *mockUserRepo) FindByEmail(_ context.Context, email string) (*userDomain.User, error) {
	if m.users == nil {
		return nil, nil
	}
	u, ok := m.users[email]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func (m *mockUserRepo) FindByID(_ context.Context, id string) (*userDomain.User, error) {
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

// --- Mock: ContactRepository ---

type mockContactRepo struct {
	contacts map[string]*contactDomain.Contact
}

func (m *mockContactRepo) Create(_ context.Context, c *contactDomain.Contact) error {
	if m.contacts == nil {
		m.contacts = make(map[string]*contactDomain.Contact)
	}
	m.contacts[c.ID] = c
	return nil
}

func (m *mockContactRepo) FindByID(_ context.Context, id, userID string) (*contactDomain.Contact, error) {
	if m.contacts == nil {
		return nil, nil
	}
	c, ok := m.contacts[id]
	if !ok || c.UserID != userID {
		return nil, nil
	}
	return c, nil
}

func (m *mockContactRepo) FindAllByUserID(_ context.Context, userID string) ([]*contactDomain.Contact, error) {
	if m.contacts == nil {
		return []*contactDomain.Contact{}, nil
	}
	var result []*contactDomain.Contact
	for _, c := range m.contacts {
		if c.UserID == userID {
			result = append(result, c)
		}
	}
	return result, nil
}

func (m *mockContactRepo) Update(_ context.Context, c *contactDomain.Contact) error {
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

// --- Test Server ---

type testServer struct {
	*httptest.Server
	userRepo    *mockUserRepo
	contactRepo *mockContactRepo
	jwtSecret   string
}

func newTestServer() *testServer {
	jwtSecret := "integration-test-secret"

	userRepo := &mockUserRepo{}
	userUC := usecase.NewUserUseCase(userRepo, jwtSecret, 1*time.Hour)
	userHandler := userHttp.NewUserHandler(userUC)

	contactRepo := &mockContactRepo{}
	contactUC := contactUsecase.NewContactUseCase(contactRepo)
	contactHandler := contactHttp.NewContactHandler(contactUC)

	r := chi.NewRouter()
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","version":"0.1.0","mongodb":"up"}`))
	})

	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Post("/register", userHandler.Register)
		r.Post("/login", userHandler.Login)
	})

	r.Route("/api/v1/contacts", func(r chi.Router) {
		r.Use(middleware.JWTAuth(jwtSecret))
		r.Get("/", contactHandler.GetAll)
		r.Post("/", contactHandler.Create)
		r.Put("/{id}", contactHandler.Update)
		r.Delete("/{id}", contactHandler.Delete)
	})

	return &testServer{
		Server:      httptest.NewServer(r),
		userRepo:    userRepo,
		contactRepo: contactRepo,
		jwtSecret:   jwtSecret,
	}
}
