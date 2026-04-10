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

type fakeGitHubProvider struct {
	authURL     string
	lastState   string
	accessToken string
	identity    *GitHubIdentity
	exchangeErr error
	fetchErr    error
}

type fakeGitHubInstallationSyncer struct {
	calls []SyncGitHubInstallationsCommand
	err   error
}

func (f *fakeGitHubInstallationSyncer) SyncInstallations(ctx context.Context, cmd SyncGitHubInstallationsCommand) (*GitHubInstallationSyncResult, error) {
	f.calls = append(f.calls, cmd)
	if f.err != nil {
		return nil, f.err
	}
	return &GitHubInstallationSyncResult{}, nil
}

func (f *fakeGitHubProvider) AuthorizationURL(state string) string {
	f.lastState = state
	query := url.Values{}
	query.Set("state", state)
	if f.authURL == "" {
		f.authURL = "https://github.com/login/oauth/authorize"
	}
	return f.authURL + "?" + query.Encode()
}

func (f *fakeGitHubProvider) ExchangeCode(ctx context.Context, code string) (string, error) {
	if f.exchangeErr != nil {
		return "", f.exchangeErr
	}
	return f.accessToken, nil
}

func (f *fakeGitHubProvider) FetchIdentity(ctx context.Context, accessToken string) (*GitHubIdentity, error) {
	if f.fetchErr != nil {
		return nil, f.fetchErr
	}
	return f.identity, nil
}

func testGitHubOAuthConfig() config.GitHubOAuthConfig {
	return config.GitHubOAuthConfig{
		Enabled:            true,
		ClientID:           "github-client-id",
		ClientSecret:       "github-client-secret",
		CallbackURL:        "http://localhost:8080/api/v1/auth/oauth/github/callback",
		SuccessRedirectURL: "http://localhost:3000/auth/github/success",
		FailureRedirectURL: "http://localhost:3000/login",
		StateTTL:           10 * time.Minute,
	}
}

func newGitHubOAuthServiceForTest(
	userStore *fakeUserStore,
	identityStore *fakeOAuthIdentityStore,
	patStore *fakePATStore,
	provider *fakeGitHubProvider,
) *GitHubOAuthService {
	authService := NewAuthService(userStore, patStore, testJWTConfig(), testPATConfig())
	return NewGitHubOAuthService(
		userStore,
		identityStore,
		authService,
		provider,
		testJWTConfig().Secret,
		testGitHubOAuthConfig(),
	)
}

func TestGitHubOAuthServiceSuccessfulLogin(t *testing.T) {
	userStore := newFakeUserStore()
	identityStore := newFakeOAuthIdentityStore()
	patStore := newFakePATStore()
	provider := &fakeGitHubProvider{
		accessToken: "github-access-token",
		identity: &GitHubIdentity{
			Subject:   "github-subject-1",
			Login:     "janedoe",
			Email:     "jane@example.com",
			Name:      "Jane Doe",
			AvatarURL: "https://example.com/avatar.png",
		},
	}
	service := newGitHubOAuthServiceForTest(userStore, identityStore, patStore, provider)

	start, err := service.Start()
	if err != nil {
		t.Fatalf("start github oauth: %v", err)
	}
	if start.StateNonce == "" {
		t.Fatal("expected state nonce")
	}
	if provider.lastState == "" || !strings.Contains(start.AuthorizationURL, "state=") {
		t.Fatal("expected authorization URL with signed state")
	}

	result, err := service.HandleCallback(context.Background(), GitHubOAuthCallbackInput{
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

	authService := NewAuthService(userStore, patStore, testJWTConfig(), testPATConfig())
	claims, err := authService.ParseToken(result.AuthResult.AccessToken)
	if err != nil {
		t.Fatalf("parse issued web session: %v", err)
	}
	if claims.AuthKind != "web_session" {
		t.Fatalf("expected web_session, got %q", claims.AuthKind)
	}
}

func TestGitHubOAuthServiceAutoSyncsInstallationsOnCallback(t *testing.T) {
	userStore := newFakeUserStore()
	identityStore := newFakeOAuthIdentityStore()
	patStore := newFakePATStore()
	provider := &fakeGitHubProvider{
		accessToken: "github-access-token",
		identity: &GitHubIdentity{
			Subject: "github-subject-1",
			Login:   "janedoe",
			Email:   "jane@example.com",
			Name:    "Jane Doe",
		},
	}
	service := newGitHubOAuthServiceForTest(userStore, identityStore, patStore, provider)
	syncer := &fakeGitHubInstallationSyncer{}
	service.WithInstallationSync(syncer)

	start, err := service.Start()
	if err != nil {
		t.Fatalf("start github oauth: %v", err)
	}

	result, err := service.HandleCallback(context.Background(), GitHubOAuthCallbackInput{
		State:      provider.lastState,
		StateNonce: start.StateNonce,
		Code:       "callback-code",
	})
	if err != nil {
		t.Fatalf("handle callback: %v", err)
	}

	if result == nil || result.AuthResult == nil {
		t.Fatal("expected callback result with auth")
	}
	if len(syncer.calls) != 1 {
		t.Fatalf("expected one installation sync call, got %d", len(syncer.calls))
	}
	if syncer.calls[0].UserID == "" {
		t.Fatal("expected sync call to include user id")
	}
	if syncer.calls[0].GitHubAccessToken != "github-access-token" {
		t.Fatalf("expected callback access token to be forwarded, got %q", syncer.calls[0].GitHubAccessToken)
	}
}

func TestGitHubOAuthServiceRejectsStateMismatch(t *testing.T) {
	userStore := newFakeUserStore()
	identityStore := newFakeOAuthIdentityStore()
	patStore := newFakePATStore()
	provider := &fakeGitHubProvider{
		accessToken: "github-access-token",
		identity: &GitHubIdentity{
			Subject: "github-subject-1",
			Login:   "janedoe",
			Email:   "jane@example.com",
			Name:    "Jane Doe",
		},
	}
	service := newGitHubOAuthServiceForTest(userStore, identityStore, patStore, provider)

	if _, err := service.Start(); err != nil {
		t.Fatalf("start github oauth: %v", err)
	}

	_, err := service.HandleCallback(context.Background(), GitHubOAuthCallbackInput{
		State:      provider.lastState,
		StateNonce: "wrong-nonce",
		Code:       "callback-code",
	})
	if !errors.Is(err, ErrInvalidOAuthState) {
		t.Fatalf("expected ErrInvalidOAuthState, got %v", err)
	}
}

func TestGitHubOAuthServiceRejectsRevokedIdentity(t *testing.T) {
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
		Provider:        GitHubOAuthProviderName,
		ProviderSubject: "github-subject-1",
		Email:           "jane@example.com",
		RevokedAt:       &now,
	})
	patStore := newFakePATStore()
	provider := &fakeGitHubProvider{
		accessToken: "github-access-token",
		identity: &GitHubIdentity{
			Subject: "github-subject-1",
			Login:   "janedoe",
			Email:   "jane@example.com",
			Name:    "Jane Doe",
		},
	}
	service := newGitHubOAuthServiceForTest(userStore, identityStore, patStore, provider)
	start, err := service.Start()
	if err != nil {
		t.Fatalf("start github oauth: %v", err)
	}

	_, err = service.HandleCallback(context.Background(), GitHubOAuthCallbackInput{
		State:      provider.lastState,
		StateNonce: start.StateNonce,
		Code:       "callback-code",
	})
	if !errors.Is(err, ErrRevokedOAuthIdentity) {
		t.Fatalf("expected ErrRevokedOAuthIdentity, got %v", err)
	}
}

func TestGitHubOAuthServiceRejectsOwnershipMismatch(t *testing.T) {
	existingUser := &models.User{
		ID:          "usr_existing",
		DisplayName: "Existing Jane",
		Email:       "jane@example.com",
		Role:        RoleOperator,
		Status:      "active",
	}
	userStore := newFakeUserStore(existingUser)
	identityStore := newFakeOAuthIdentityStore(&models.OAuthIdentity{
		ID:              "oid_existing",
		UserID:          existingUser.ID,
		Provider:        GitHubOAuthProviderName,
		ProviderSubject: "github-subject-old",
		Email:           "jane@example.com",
	})
	patStore := newFakePATStore()
	provider := &fakeGitHubProvider{
		accessToken: "github-access-token",
		identity: &GitHubIdentity{
			Subject: "github-subject-new",
			Login:   "janedoe-new",
			Email:   "jane@example.com",
			Name:    "Jane Doe",
		},
	}
	service := newGitHubOAuthServiceForTest(userStore, identityStore, patStore, provider)
	start, err := service.Start()
	if err != nil {
		t.Fatalf("start github oauth: %v", err)
	}

	_, err = service.HandleCallback(context.Background(), GitHubOAuthCallbackInput{
		State:      provider.lastState,
		StateNonce: start.StateNonce,
		Code:       "callback-code",
	})
	if !errors.Is(err, ErrOAuthIdentityOwnershipMismatch) {
		t.Fatalf("expected ErrOAuthIdentityOwnershipMismatch, got %v", err)
	}
}

func TestGitHubOAuthServiceReturnsErrorWhenAutoSyncFails(t *testing.T) {
	userStore := newFakeUserStore()
	identityStore := newFakeOAuthIdentityStore()
	patStore := newFakePATStore()
	provider := &fakeGitHubProvider{
		accessToken: "github-access-token",
		identity: &GitHubIdentity{
			Subject: "github-subject-1",
			Login:   "janedoe",
			Email:   "jane@example.com",
			Name:    "Jane Doe",
		},
	}
	service := newGitHubOAuthServiceForTest(userStore, identityStore, patStore, provider)
	syncer := &fakeGitHubInstallationSyncer{err: ErrGitHubProviderError}
	service.WithInstallationSync(syncer)

	start, err := service.Start()
	if err != nil {
		t.Fatalf("start github oauth: %v", err)
	}

	_, err = service.HandleCallback(context.Background(), GitHubOAuthCallbackInput{
		State:      provider.lastState,
		StateNonce: start.StateNonce,
		Code:       "callback-code",
	})
	if !errors.Is(err, ErrGitHubInstallationsSyncFailed) {
		t.Fatalf("expected ErrGitHubInstallationsSyncFailed, got %v", err)
	}
}
