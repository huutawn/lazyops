package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"lazyops-server/internal/config"
	"lazyops-server/internal/models"
)

type fakeUserStore struct {
	byID      map[string]*models.User
	byEmail   map[string]*models.User
	createErr error
	touchErr  error
	lastLogin map[string]time.Time
}

func newFakeUserStore(users ...*models.User) *fakeUserStore {
	store := &fakeUserStore{
		byID:      make(map[string]*models.User),
		byEmail:   make(map[string]*models.User),
		lastLogin: make(map[string]time.Time),
	}

	for _, user := range users {
		store.byID[user.ID] = user
		store.byEmail[strings.ToLower(user.Email)] = user
	}

	return store
}

func (f *fakeUserStore) Create(user *models.User) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.byID[user.ID] = user
	f.byEmail[strings.ToLower(user.Email)] = user
	return nil
}

func (f *fakeUserStore) GetByEmail(email string) (*models.User, error) {
	if user, ok := f.byEmail[strings.ToLower(email)]; ok {
		return user, nil
	}
	return nil, nil
}

func (f *fakeUserStore) GetByID(id string) (*models.User, error) {
	if user, ok := f.byID[id]; ok {
		return user, nil
	}
	return nil, nil
}

func (f *fakeUserStore) TouchLastLogin(userID string, at time.Time) error {
	if f.touchErr != nil {
		return f.touchErr
	}
	user, ok := f.byID[userID]
	if !ok {
		return nil
	}
	f.lastLogin[userID] = at
	user.LastLoginAt = &at
	return nil
}

func testJWTConfig() config.JWTConfig {
	return config.JWTConfig{
		Secret:    "unit-test-secret",
		Issuer:    "lazyops-backend-test",
		ExpiresIn: time.Hour,
	}
}

func hashedPassword(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	return string(hash)
}

func TestAuthServiceRegisterNormalizesAndDefaults(t *testing.T) {
	store := newFakeUserStore()
	service := NewAuthService(store, testJWTConfig())

	result, err := service.Register(RegisterCommand{
		Name:     "  Jane   Doe  ",
		Email:    "  Jane@Example.com ",
		Password: "StrongPass1",
	})
	if err != nil {
		t.Fatalf("register returned error: %v", err)
	}

	user, err := store.GetByEmail("jane@example.com")
	if err != nil {
		t.Fatalf("lookup user: %v", err)
	}
	if user == nil {
		t.Fatal("expected user to be created")
	}
	if !strings.HasPrefix(user.ID, "usr_") {
		t.Fatalf("expected prefixed user id, got %q", user.ID)
	}
	if user.DisplayName != "Jane Doe" {
		t.Fatalf("expected normalized display name, got %q", user.DisplayName)
	}
	if user.Email != "jane@example.com" {
		t.Fatalf("expected normalized email, got %q", user.Email)
	}
	if user.Role != RoleViewer {
		t.Fatalf("expected default viewer role, got %q", user.Role)
	}
	if user.Status != "active" {
		t.Fatalf("expected active status, got %q", user.Status)
	}
	if result.User.ID != user.ID {
		t.Fatalf("expected result user id %q, got %q", user.ID, result.User.ID)
	}
}

func TestAuthServiceRegisterRejectsInvalidInput(t *testing.T) {
	store := newFakeUserStore()
	service := NewAuthService(store, testJWTConfig())

	_, err := service.Register(RegisterCommand{
		Name:     "",
		Email:    "bad-email",
		Password: "StrongPass1",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestAuthServiceRegisterRejectsWeakPassword(t *testing.T) {
	store := newFakeUserStore()
	service := NewAuthService(store, testJWTConfig())

	_, err := service.Register(RegisterCommand{
		Name:     "Jane Doe",
		Email:    "jane@example.com",
		Password: "weakpass",
	})
	if !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("expected ErrWeakPassword, got %v", err)
	}
}

func TestAuthServiceLoginRejectsWrongPassword(t *testing.T) {
	store := newFakeUserStore(&models.User{
		ID:           "usr_test",
		DisplayName:  "Jane Doe",
		Email:        "jane@example.com",
		PasswordHash: hashedPassword(t, "StrongPass1"),
		Role:         RoleViewer,
		Status:       "active",
	})
	service := NewAuthService(store, testJWTConfig())

	_, err := service.Login(LoginCommand{
		Email:    "jane@example.com",
		Password: "WrongPass1",
	})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthServiceLoginRejectsDisabledAccount(t *testing.T) {
	store := newFakeUserStore(&models.User{
		ID:           "usr_test",
		DisplayName:  "Jane Doe",
		Email:        "jane@example.com",
		PasswordHash: hashedPassword(t, "StrongPass1"),
		Role:         RoleViewer,
		Status:       "disabled",
	})
	service := NewAuthService(store, testJWTConfig())

	_, err := service.Login(LoginCommand{
		Email:    "jane@example.com",
		Password: "StrongPass1",
	})
	if !errors.Is(err, ErrAccountDisabled) {
		t.Fatalf("expected ErrAccountDisabled, got %v", err)
	}
}

func TestAuthServiceLoginUpdatesLastLoginAndIssuesWebSessionClaims(t *testing.T) {
	store := newFakeUserStore(&models.User{
		ID:           "usr_test",
		DisplayName:  "Jane Doe",
		Email:        "jane@example.com",
		PasswordHash: hashedPassword(t, "StrongPass1"),
		Role:         RoleOperator,
		Status:       "active",
	})
	cfg := testJWTConfig()
	service := NewAuthService(store, cfg)

	result, err := service.Login(LoginCommand{
		Email:    "jane@example.com",
		Password: "StrongPass1",
	})
	if err != nil {
		t.Fatalf("login returned error: %v", err)
	}

	if _, ok := store.lastLogin["usr_test"]; !ok {
		t.Fatal("expected last login timestamp to be updated")
	}

	claims, err := service.ParseToken(result.AccessToken)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.AuthKind != "web_session" {
		t.Fatalf("expected web_session auth kind, got %q", claims.AuthKind)
	}
	if claims.UserID != "usr_test" {
		t.Fatalf("expected user id usr_test, got %q", claims.UserID)
	}
	if claims.Issuer != cfg.Issuer {
		t.Fatalf("expected issuer %q, got %q", cfg.Issuer, claims.Issuer)
	}
	if claims.Subject != "usr_test" {
		t.Fatalf("expected subject usr_test, got %q", claims.Subject)
	}
	if claims.ID == "" {
		t.Fatal("expected session id to be populated")
	}
}
