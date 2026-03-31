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

type fakePATStore struct {
	byID      map[string]*models.PersonalAccessToken
	byHash    map[string]*models.PersonalAccessToken
	createErr error
	revokeErr error
	touchErr  error
	lastUsed  map[string]time.Time
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

func newFakePATStore(tokens ...*models.PersonalAccessToken) *fakePATStore {
	store := &fakePATStore{
		byID:     make(map[string]*models.PersonalAccessToken),
		byHash:   make(map[string]*models.PersonalAccessToken),
		lastUsed: make(map[string]time.Time),
	}

	for _, token := range tokens {
		store.byID[token.ID] = token
		store.byHash[token.TokenHash] = token
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

func (f *fakePATStore) Create(token *models.PersonalAccessToken) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.byID[token.ID] = token
	f.byHash[token.TokenHash] = token
	return nil
}

func (f *fakePATStore) GetByHash(tokenHash string) (*models.PersonalAccessToken, error) {
	if token, ok := f.byHash[tokenHash]; ok {
		return token, nil
	}
	return nil, nil
}

func (f *fakePATStore) GetByID(tokenID string) (*models.PersonalAccessToken, error) {
	if token, ok := f.byID[tokenID]; ok {
		return token, nil
	}
	return nil, nil
}

func (f *fakePATStore) RevokeByIDForUser(userID, tokenID string, at time.Time) error {
	if f.revokeErr != nil {
		return f.revokeErr
	}
	token, ok := f.byID[tokenID]
	if !ok || token.UserID != userID {
		return nil
	}
	token.RevokedAt = &at
	return nil
}

func (f *fakePATStore) TouchLastUsed(tokenID string, at time.Time) error {
	if f.touchErr != nil {
		return f.touchErr
	}
	token, ok := f.byID[tokenID]
	if !ok {
		return nil
	}
	f.lastUsed[tokenID] = at
	token.LastUsedAt = &at
	return nil
}

func testJWTConfig() config.JWTConfig {
	return config.JWTConfig{
		Secret:    "unit-test-secret",
		Issuer:    "lazyops-backend-test",
		ExpiresIn: time.Hour,
	}
}

func testPATConfig() config.PATConfig {
	return config.PATConfig{
		ExpiresIn: 30 * 24 * time.Hour,
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
	patStore := newFakePATStore()
	service := NewAuthService(store, patStore, testJWTConfig(), testPATConfig())

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
	patStore := newFakePATStore()
	service := NewAuthService(store, patStore, testJWTConfig(), testPATConfig())

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
	patStore := newFakePATStore()
	service := NewAuthService(store, patStore, testJWTConfig(), testPATConfig())

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
	patStore := newFakePATStore()
	service := NewAuthService(store, patStore, testJWTConfig(), testPATConfig())

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
	patStore := newFakePATStore()
	service := NewAuthService(store, patStore, testJWTConfig(), testPATConfig())

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
	patStore := newFakePATStore()
	service := NewAuthService(store, patStore, cfg, testPATConfig())

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

func TestAuthServiceCLILoginIssuesPATAndStoresHash(t *testing.T) {
	userStore := newFakeUserStore(&models.User{
		ID:           "usr_test",
		DisplayName:  "Jane Doe",
		Email:        "jane@example.com",
		PasswordHash: hashedPassword(t, "StrongPass1"),
		Role:         RoleOperator,
		Status:       "active",
	})
	patStore := newFakePATStore()
	service := NewAuthService(userStore, patStore, testJWTConfig(), testPATConfig())

	result, err := service.CLILogin(CLILoginCommand{
		AuthFlow:   "password",
		Email:      "jane@example.com",
		Password:   "StrongPass1",
		DeviceName: "  MacBook Pro  ",
	})
	if err != nil {
		t.Fatalf("cli login returned error: %v", err)
	}
	if result.Token == "" {
		t.Fatal("expected PAT token to be returned")
	}
	if result.TokenID == "" || !strings.HasPrefix(result.TokenID, "pat_") {
		t.Fatalf("expected PAT token id, got %q", result.TokenID)
	}
	if result.ExpiresAt == nil {
		t.Fatal("expected PAT expiry to be returned")
	}

	record, err := patStore.GetByID(result.TokenID)
	if err != nil {
		t.Fatalf("get PAT by id: %v", err)
	}
	if record == nil {
		t.Fatal("expected PAT record to be created")
	}
	if record.TokenHash != hashOpaqueToken(result.Token) {
		t.Fatal("expected PAT hash to match opaque token")
	}
	if record.TokenHash == result.Token {
		t.Fatal("expected PAT to be stored as hash, not plaintext")
	}
	if record.Name != "MacBook Pro" {
		t.Fatalf("expected normalized device name, got %q", record.Name)
	}
	if record.UserID != "usr_test" {
		t.Fatalf("expected PAT user id usr_test, got %q", record.UserID)
	}
	if record.TokenPrefix == "" || !strings.HasPrefix(result.Token, record.TokenPrefix) {
		t.Fatalf("expected PAT prefix to come from token, got %q", record.TokenPrefix)
	}
}

func TestAuthServiceParsePATUpdatesLastUsedAndReturnsCLIClaims(t *testing.T) {
	rawToken := "lop_pat_unit_test_token"
	expiresAt := time.Now().UTC().Add(time.Hour)
	userStore := newFakeUserStore(&models.User{
		ID:           "usr_test",
		DisplayName:  "Jane Doe",
		Email:        "jane@example.com",
		PasswordHash: hashedPassword(t, "StrongPass1"),
		Role:         RoleOperator,
		Status:       "active",
	})
	patStore := newFakePATStore(&models.PersonalAccessToken{
		ID:          "pat_test",
		UserID:      "usr_test",
		Name:        "MacBook Pro",
		TokenHash:   hashOpaqueToken(rawToken),
		TokenPrefix: "lop_pat_unit_tes",
		ExpiresAt:   &expiresAt,
		CreatedAt:   time.Now().UTC().Add(-time.Minute),
	})
	service := NewAuthService(userStore, patStore, testJWTConfig(), testPATConfig())

	claims, err := service.ParseToken(rawToken)
	if err != nil {
		t.Fatalf("parse PAT: %v", err)
	}
	if claims.AuthKind != "cli_pat" {
		t.Fatalf("expected cli_pat auth kind, got %q", claims.AuthKind)
	}
	if claims.TokenID != "pat_test" {
		t.Fatalf("expected PAT token id pat_test, got %q", claims.TokenID)
	}
	if claims.UserID != "usr_test" {
		t.Fatalf("expected user id usr_test, got %q", claims.UserID)
	}
	if _, ok := patStore.lastUsed["pat_test"]; !ok {
		t.Fatal("expected last_used_at to be updated")
	}
}

func TestAuthServiceRevokePATRejectsOwnershipMismatch(t *testing.T) {
	userStore := newFakeUserStore()
	patStore := newFakePATStore(&models.PersonalAccessToken{
		ID:          "pat_other",
		UserID:      "usr_other",
		Name:        "Other Device",
		TokenHash:   hashOpaqueToken("lop_pat_other"),
		TokenPrefix: "lop_pat_other",
	})
	service := NewAuthService(userStore, patStore, testJWTConfig(), testPATConfig())

	_, err := service.RevokePAT(RevokePATCommand{
		UserID:  "usr_test",
		TokenID: "pat_other",
	})
	if !errors.Is(err, ErrTokenAccessDenied) {
		t.Fatalf("expected ErrTokenAccessDenied, got %v", err)
	}
}

func TestAuthServiceRevokedPATNoLongerWorks(t *testing.T) {
	rawToken := "lop_pat_revocable"
	expiresAt := time.Now().UTC().Add(time.Hour)
	userStore := newFakeUserStore(&models.User{
		ID:           "usr_test",
		DisplayName:  "Jane Doe",
		Email:        "jane@example.com",
		PasswordHash: hashedPassword(t, "StrongPass1"),
		Role:         RoleOperator,
		Status:       "active",
	})
	patStore := newFakePATStore(&models.PersonalAccessToken{
		ID:          "pat_test",
		UserID:      "usr_test",
		Name:        "MacBook Pro",
		TokenHash:   hashOpaqueToken(rawToken),
		TokenPrefix: "lop_pat_revocab",
		ExpiresAt:   &expiresAt,
	})
	service := NewAuthService(userStore, patStore, testJWTConfig(), testPATConfig())

	result, err := service.RevokePAT(RevokePATCommand{
		UserID:  "usr_test",
		TokenID: "pat_test",
	})
	if err != nil {
		t.Fatalf("revoke PAT: %v", err)
	}
	if !result.Revoked {
		t.Fatal("expected revoke result to confirm token was revoked")
	}

	_, err = service.ParseToken(rawToken)
	if !errors.Is(err, ErrTokenRevoked) {
		t.Fatalf("expected ErrTokenRevoked after revoke, got %v", err)
	}
}
