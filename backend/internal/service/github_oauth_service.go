package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"lazyops-server/internal/config"
	"lazyops-server/internal/models"
	"lazyops-server/pkg/utils"
)

const (
	GitHubOAuthProviderName = "github"
	GitHubOAuthStateCookie  = "lazyops_oauth_github_state"
)

type GitHubIdentity struct {
	Subject   string
	Login     string
	Email     string
	Name      string
	AvatarURL string
}

type GitHubOAuthProvider interface {
	AuthorizationURL(state string) string
	ExchangeCode(ctx context.Context, code string) (string, error)
	FetchIdentity(ctx context.Context, accessToken string) (*GitHubIdentity, error)
}

type GitHubInstallationSyncer interface {
	SyncInstallations(ctx context.Context, cmd SyncGitHubInstallationsCommand) (*GitHubInstallationSyncResult, error)
}

type GitHubOAuthStartResult struct {
	AuthorizationURL string
	StateNonce       string
}

type GitHubOAuthCallbackInput struct {
	State         string
	StateNonce    string
	Code          string
	ProviderError string
}

type GitHubOAuthCallbackResult struct {
	AuthResult *AuthResult
	User       UserProfile
	Linked     bool
}

type githubOAuthStateClaims struct {
	Provider string `json:"provider"`
	Nonce    string `json:"nonce"`
	jwt.RegisteredClaims
}

type GitHubOAuthService struct {
	users       UserStore
	identities  OAuthIdentityStore
	auth        *AuthService
	provider    GitHubOAuthProvider
	installSync GitHubInstallationSyncer
	cfg         config.GitHubOAuthConfig
	stateSecret string
}

func NewGitHubOAuthService(
	users UserStore,
	identities OAuthIdentityStore,
	auth *AuthService,
	provider GitHubOAuthProvider,
	stateSecret string,
	cfg config.GitHubOAuthConfig,
) *GitHubOAuthService {
	return &GitHubOAuthService{
		users:       users,
		identities:  identities,
		auth:        auth,
		provider:    provider,
		cfg:         cfg,
		stateSecret: stateSecret,
	}
}

func (s *GitHubOAuthService) WithInstallationSync(syncer GitHubInstallationSyncer) *GitHubOAuthService {
	s.installSync = syncer
	return s
}

func (s *GitHubOAuthService) Start() (*GitHubOAuthStartResult, error) {
	if !s.isConfigured() {
		return nil, ErrOAuthNotConfigured
	}

	nonce, err := newOAuthNonce()
	if err != nil {
		return nil, err
	}
	state, err := s.signState(nonce)
	if err != nil {
		return nil, err
	}

	return &GitHubOAuthStartResult{
		AuthorizationURL: s.provider.AuthorizationURL(state),
		StateNonce:       nonce,
	}, nil
}

func (s *GitHubOAuthService) HandleCallback(ctx context.Context, input GitHubOAuthCallbackInput) (*GitHubOAuthCallbackResult, error) {
	if !s.isConfigured() {
		return nil, ErrOAuthNotConfigured
	}

	if err := s.validateState(input.State, input.StateNonce); err != nil {
		return nil, err
	}

	if strings.TrimSpace(input.ProviderError) != "" {
		return nil, fmt.Errorf("%w: %s", ErrOAuthProviderFailure, input.ProviderError)
	}
	if strings.TrimSpace(input.Code) == "" {
		return nil, ErrOAuthProviderFailure
	}

	accessToken, err := s.provider.ExchangeCode(ctx, input.Code)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOAuthProviderFailure, err)
	}

	identity, err := s.provider.FetchIdentity(ctx, accessToken)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOAuthProviderFailure, err)
	}

	user, linked, err := s.resolveUser(identity)
	if err != nil {
		return nil, err
	}

	if s.installSync != nil && strings.TrimSpace(accessToken) != "" {
		if _, err := s.installSync.SyncInstallations(ctx, SyncGitHubInstallationsCommand{
			UserID:            user.ID,
			GitHubAccessToken: accessToken,
		}); err != nil {
			// Do not block OAuth login on installation sync failures.
			// Users can retry sync from integrations screen.
		}
	}

	now := time.Now().UTC()
	if err := s.users.TouchLastLogin(user.ID, now); err != nil {
		return nil, err
	}
	user.LastLoginAt = &now

	authResult, err := s.auth.IssueWebSessionForUser(user)
	if err != nil {
		return nil, err
	}

	return &GitHubOAuthCallbackResult{
		AuthResult: authResult,
		User:       ToUserProfile(user),
		Linked:     linked,
	}, nil
}

func (s *GitHubOAuthService) SuccessRedirectURL() string {
	return strings.TrimSpace(s.cfg.SuccessRedirectURL)
}

func (s *GitHubOAuthService) FailureRedirectURL() string {
	return strings.TrimSpace(s.cfg.FailureRedirectURL)
}

func (s *GitHubOAuthService) StateTTL() time.Duration {
	if s.cfg.StateTTL <= 0 {
		return 10 * time.Minute
	}
	return s.cfg.StateTTL
}

func (s *GitHubOAuthService) isConfigured() bool {
	return s.cfg.Enabled &&
		strings.TrimSpace(s.cfg.ClientID) != "" &&
		strings.TrimSpace(s.cfg.ClientSecret) != "" &&
		strings.TrimSpace(s.cfg.CallbackURL) != ""
}

func (s *GitHubOAuthService) signState(nonce string) (string, error) {
	now := time.Now().UTC()
	claims := githubOAuthStateClaims{
		Provider: GitHubOAuthProviderName,
		Nonce:    nonce,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "lazyops-github-oauth-state",
			Subject:   GitHubOAuthProviderName,
			ExpiresAt: jwt.NewNumericDate(now.Add(s.StateTTL())),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.stateSecret))
}

func (s *GitHubOAuthService) validateState(state, nonce string) error {
	if strings.TrimSpace(state) == "" || strings.TrimSpace(nonce) == "" {
		return ErrInvalidOAuthState
	}

	claims := &githubOAuthStateClaims{}
	parsed, err := jwt.ParseWithClaims(
		state,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, ErrInvalidOAuthState
			}
			return []byte(s.stateSecret), nil
		},
		jwt.WithIssuer("lazyops-github-oauth-state"),
		jwt.WithSubject(GitHubOAuthProviderName),
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil || !parsed.Valid {
		return ErrInvalidOAuthState
	}
	if claims.Provider != GitHubOAuthProviderName || claims.Nonce != nonce {
		return ErrInvalidOAuthState
	}

	return nil
}

func (s *GitHubOAuthService) resolveUser(identity *GitHubIdentity) (*models.User, bool, error) {
	if identity == nil {
		return nil, false, ErrOAuthProviderFailure
	}

	normalizedEmail := strings.ToLower(strings.TrimSpace(identity.Email))
	if strings.TrimSpace(identity.Subject) == "" || !isValidEmail(normalizedEmail) {
		return nil, false, ErrOAuthProviderFailure
	}

	existingIdentity, err := s.identities.GetByProviderSubject(GitHubOAuthProviderName, identity.Subject)
	if err != nil {
		return nil, false, err
	}
	if existingIdentity != nil {
		if existingIdentity.RevokedAt != nil {
			return nil, false, ErrRevokedOAuthIdentity
		}

		user, err := s.users.GetByID(existingIdentity.UserID)
		if err != nil {
			return nil, false, err
		}
		if user == nil {
			return nil, false, ErrOAuthProviderFailure
		}
		if user.Status != "active" {
			return nil, false, ErrAccountDisabled
		}

		if err := s.identities.UpdateProfile(existingIdentity.ID, normalizedEmail, strings.TrimSpace(identity.AvatarURL), time.Now().UTC()); err != nil {
			return nil, false, err
		}

		return user, true, nil
	}

	user, err := s.users.GetByEmail(normalizedEmail)
	if err != nil {
		return nil, false, err
	}
	if user != nil && user.Status != "active" {
		return nil, false, ErrAccountDisabled
	}

	linkedExistingUser := user != nil
	if user == nil {
		displayName := utils.NormalizeSpace(identity.Name)
		if displayName == "" {
			displayName = strings.TrimSpace(identity.Login)
		}
		if displayName == "" {
			displayName = strings.Split(normalizedEmail, "@")[0]
		}

		user = &models.User{
			ID:          utils.NewPrefixedID("usr"),
			DisplayName: displayName,
			Email:       normalizedEmail,
			Role:        RoleViewer,
			Status:      "active",
		}
		if err := s.users.Create(user); err != nil {
			return nil, false, err
		}
	}

	if linkedExistingUser {
		userIdentity, err := s.identities.GetByUserProvider(user.ID, GitHubOAuthProviderName)
		if err != nil {
			return nil, false, err
		}
		if userIdentity != nil {
			if userIdentity.RevokedAt != nil {
				return nil, false, ErrRevokedOAuthIdentity
			}
			if userIdentity.ProviderSubject != strings.TrimSpace(identity.Subject) {
				return nil, false, ErrOAuthIdentityOwnershipMismatch
			}
		}
	}

	linkedIdentity := &models.OAuthIdentity{
		ID:              utils.NewPrefixedID("oid"),
		UserID:          user.ID,
		Provider:        GitHubOAuthProviderName,
		ProviderSubject: strings.TrimSpace(identity.Subject),
		Email:           normalizedEmail,
		AvatarURL:       strings.TrimSpace(identity.AvatarURL),
	}
	if err := s.identities.Create(linkedIdentity); err != nil {
		return nil, false, err
	}

	return user, linkedExistingUser, nil
}
