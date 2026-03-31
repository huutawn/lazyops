package service

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"lazyops-server/internal/config"
	"lazyops-server/internal/models"
)

type fakeOAuthIdentityStore struct {
	byProviderSubject map[string]*models.OAuthIdentity
	createErr         error
	updateErr         error
	created           []*models.OAuthIdentity
	updated           []string
}

type fakeGoogleProvider struct {
	authURL     string
	lastState   string
	accessToken string
	identity    *GoogleIdentity
	exchangeErr error
	fetchErr    error
}

func newFakeOAuthIdentityStore(identities ...*models.OAuthIdentity) *fakeOAuthIdentityStore {
	store := &fakeOAuthIdentityStore{
		byProviderSubject: make(map[string]*models.OAuthIdentity),
	}

	for _, identity := range identities {
		store.byProviderSubject[identity.Provider+":"+identity.ProviderSubject] = identity
	}

	return store
}

func (f *fakeOAuthIdentityStore) Create(identity *models.OAuthIdentity) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.created = append(f.created, identity)
	f.byProviderSubject[identity.Provider+":"+identity.ProviderSubject] = identity
	return nil
}

func (f *fakeOAuthIdentityStore) GetByProviderSubject(provider, subject string) (*models.OAuthIdentity, error) {
	if identity, ok := f.byProviderSubject[provider+":"+subject]; ok {
		return identity, nil
	}
	return nil, nil
}

func (f *fakeOAuthIdentityStore) UpdateProfile(identityID, email, avatarURL string, at time.Time) error {
	if f.updateErr != nil {
		return f.updateErr
	}

	f.updated = append(f.updated, identityID)
	for _, identity := range f.byProviderSubject {
		if identity.ID == identityID {
			identity.Email = email
			identity.AvatarURL = avatarURL
			identity.UpdatedAt = at
		}
	}
	return nil
}

func (f *fakeGoogleProvider) AuthorizationURL(state string) string {
	f.lastState = state
	query := url.Values{}
	query.Set("state", state)
	if f.authURL == "" {
		f.authURL = "https://accounts.google.com/o/oauth2/v2/auth"
	}
	return f.authURL + "?" + query.Encode()
}

func (f *fakeGoogleProvider) ExchangeCode(ctx context.Context, code string) (string, error) {
	if f.exchangeErr != nil {
		return "", f.exchangeErr
	}
	return f.accessToken, nil
}

func (f *fakeGoogleProvider) FetchIdentity(ctx context.Context, accessToken string) (*GoogleIdentity, error) {
	if f.fetchErr != nil {
		return nil, f.fetchErr
	}
	return f.identity, nil
}

func testGoogleOAuthConfig() config.GoogleOAuthConfig {
	return config.GoogleOAuthConfig{
		Enabled:            true,
		ClientID:           "google-client-id",
		ClientSecret:       "google-client-secret",
		CallbackURL:        "http://localhost:8080/api/v1/auth/oauth/google/callback",
		SuccessRedirectURL: "http://localhost:3000/auth/google/success",
		FailureRedirectURL: "http://localhost:3000/login",
		StateTTL:           10 * time.Minute,
	}
}

func newGoogleOAuthServiceForTest(
	userStore *fakeUserStore,
	identityStore *fakeOAuthIdentityStore,
	patStore *fakePATStore,
	provider *fakeGoogleProvider,
) *GoogleOAuthService {
	authService := NewAuthService(userStore, patStore, testJWTConfig(), testPATConfig())
	return NewGoogleOAuthService(
		userStore,
		identityStore,
		authService,
		provider,
		testJWTConfig().Secret,
		testGoogleOAuthConfig(),
	)
}

func TestGoogleOAuthServiceSuccessCreatesSessionAndIdentity(t *testing.T) {
	userStore := newFakeUserStore()
	identityStore := newFakeOAuthIdentityStore()
	patStore := newFakePATStore()
	provider := &fakeGoogleProvider{
		accessToken: "google-access-token",
		identity: &GoogleIdentity{
			Subject:       "google-subject-1",
			Email:         "jane@example.com",
			EmailVerified: true,
			Name:          "Jane Doe",
			AvatarURL:     "https://example.com/avatar.png",
		},
	}
	service := newGoogleOAuthServiceForTest(userStore, identityStore, patStore, provider)

	start, err := service.Start()
	if err != nil {
		t.Fatalf("start google oauth: %v", err)
	}
	if start.StateNonce == "" {
		t.Fatal("expected state nonce")
	}
	if provider.lastState == "" || !strings.Contains(start.AuthorizationURL, "state=") {
		t.Fatal("expected authorization URL with signed state")
	}

	result, err := service.HandleCallback(context.Background(), GoogleOAuthCallbackInput{
		State:      provider.lastState,
		StateNonce: start.StateNonce,
		Code:       "callback-code",
	})
	if err != nil {
		t.Fatalf("handle callback: %v", err)
	}
	if result.AuthResult == nil {
		t.Fatal("expected auth result")
	}
	if len(identityStore.created) != 1 {
		t.Fatalf("expected one oauth identity to be created, got %d", len(identityStore.created))
	}

	user, err := userStore.GetByEmail("jane@example.com")
	if err != nil {
		t.Fatalf("lookup user: %v", err)
	}
	if user == nil {
		t.Fatal("expected user to be created")
	}
	if identityStore.created[0].UserID != user.ID {
		t.Fatalf("expected linked user id %q, got %q", user.ID, identityStore.created[0].UserID)
	}

	authService := NewAuthService(userStore, patStore, testJWTConfig(), testPATConfig())
	claims, err := authService.ParseToken(result.AuthResult.AccessToken)
	if err != nil {
		t.Fatalf("parse issued web session: %v", err)
	}
	if claims.AuthKind != "web_session" {
		t.Fatalf("expected web_session, got %q", claims.AuthKind)
	}
}

func TestGoogleOAuthServiceRejectsStateMismatch(t *testing.T) {
	userStore := newFakeUserStore()
	identityStore := newFakeOAuthIdentityStore()
	patStore := newFakePATStore()
	provider := &fakeGoogleProvider{
		accessToken: "google-access-token",
		identity: &GoogleIdentity{
			Subject:       "google-subject-1",
			Email:         "jane@example.com",
			EmailVerified: true,
			Name:          "Jane Doe",
		},
	}
	service := newGoogleOAuthServiceForTest(userStore, identityStore, patStore, provider)

	if _, err := service.Start(); err != nil {
		t.Fatalf("start google oauth: %v", err)
	}

	_, err := service.HandleCallback(context.Background(), GoogleOAuthCallbackInput{
		State:      provider.lastState,
		StateNonce: "wrong-nonce",
		Code:       "callback-code",
	})
	if !errors.Is(err, ErrInvalidOAuthState) {
		t.Fatalf("expected ErrInvalidOAuthState, got %v", err)
	}
}

func TestGoogleOAuthServiceRejectsRevokedIdentity(t *testing.T) {
	now := time.Now().UTC()
	userStore := newFakeUserStore(&models.User{
		ID:          "usr_test",
		DisplayName: "Jane Doe",
		Email:       "jane@example.com",
		Role:        RoleViewer,
		Status:      "active",
	})
	identityStore := newFakeOAuthIdentityStore(&models.OAuthIdentity{
		ID:              "oid_test",
		UserID:          "usr_test",
		Provider:        GoogleOAuthProviderName,
		ProviderSubject: "google-subject-1",
		Email:           "jane@example.com",
		RevokedAt:       &now,
	})
	patStore := newFakePATStore()
	provider := &fakeGoogleProvider{
		accessToken: "google-access-token",
		identity: &GoogleIdentity{
			Subject:       "google-subject-1",
			Email:         "jane@example.com",
			EmailVerified: true,
			Name:          "Jane Doe",
		},
	}
	service := newGoogleOAuthServiceForTest(userStore, identityStore, patStore, provider)
	start, err := service.Start()
	if err != nil {
		t.Fatalf("start google oauth: %v", err)
	}

	_, err = service.HandleCallback(context.Background(), GoogleOAuthCallbackInput{
		State:      provider.lastState,
		StateNonce: start.StateNonce,
		Code:       "callback-code",
	})
	if !errors.Is(err, ErrRevokedOAuthIdentity) {
		t.Fatalf("expected ErrRevokedOAuthIdentity, got %v", err)
	}
}

func TestGoogleOAuthServiceLinksExistingEmailWithoutDuplicateUser(t *testing.T) {
	existingUser := &models.User{
		ID:          "usr_existing",
		DisplayName: "Existing Jane",
		Email:       "jane@example.com",
		Role:        RoleOperator,
		Status:      "active",
	}
	userStore := newFakeUserStore(existingUser)
	identityStore := newFakeOAuthIdentityStore()
	patStore := newFakePATStore()
	provider := &fakeGoogleProvider{
		accessToken: "google-access-token",
		identity: &GoogleIdentity{
			Subject:       "google-subject-2",
			Email:         "jane@example.com",
			EmailVerified: true,
			Name:          "Jane From Google",
			AvatarURL:     "https://example.com/avatar-2.png",
		},
	}
	service := newGoogleOAuthServiceForTest(userStore, identityStore, patStore, provider)
	start, err := service.Start()
	if err != nil {
		t.Fatalf("start google oauth: %v", err)
	}

	result, err := service.HandleCallback(context.Background(), GoogleOAuthCallbackInput{
		State:      provider.lastState,
		StateNonce: start.StateNonce,
		Code:       "callback-code",
	})
	if err != nil {
		t.Fatalf("handle callback: %v", err)
	}
	if !result.Linked {
		t.Fatal("expected callback to link existing email user")
	}
	if len(userStore.byID) != 1 {
		t.Fatalf("expected no duplicate user, got %d users", len(userStore.byID))
	}
	if len(identityStore.created) != 1 {
		t.Fatalf("expected one linked identity, got %d", len(identityStore.created))
	}
	if identityStore.created[0].UserID != existingUser.ID {
		t.Fatalf("expected identity to link to existing user %q, got %q", existingUser.ID, identityStore.created[0].UserID)
	}
}
