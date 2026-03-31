package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"lazyops-server/internal/config"
	"lazyops-server/internal/models"
	"lazyops-server/pkg/utils"
)

const (
	GoogleOAuthProviderName = "google"
	GoogleOAuthStateCookie  = "lazyops_oauth_google_state"
)

var (
	ErrOAuthNotConfigured   = errors.New("oauth not configured")
	ErrInvalidOAuthState    = errors.New("invalid oauth state")
	ErrOAuthProviderFailure = errors.New("oauth provider error")
	ErrRevokedOAuthIdentity = errors.New("revoked oauth identity")
)

type GoogleIdentity struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	AvatarURL     string
}

type GoogleOAuthProvider interface {
	AuthorizationURL(state string) string
	ExchangeCode(ctx context.Context, code string) (string, error)
	FetchIdentity(ctx context.Context, accessToken string) (*GoogleIdentity, error)
}

type GoogleOAuthStartResult struct {
	AuthorizationURL string
	StateNonce       string
}

type GoogleOAuthCallbackInput struct {
	State         string
	StateNonce    string
	Code          string
	ProviderError string
}

type GoogleOAuthCallbackResult struct {
	AuthResult *AuthResult
	User       UserProfile
	Linked     bool
}

type googleOAuthStateClaims struct {
	Provider string `json:"provider"`
	Nonce    string `json:"nonce"`
	jwt.RegisteredClaims
}

type GoogleOAuthService struct {
	users       UserStore
	identities  OAuthIdentityStore
	auth        *AuthService
	provider    GoogleOAuthProvider
	cfg         config.GoogleOAuthConfig
	stateSecret string
}

func NewGoogleOAuthService(
	users UserStore,
	identities OAuthIdentityStore,
	auth *AuthService,
	provider GoogleOAuthProvider,
	stateSecret string,
	cfg config.GoogleOAuthConfig,
) *GoogleOAuthService {
	return &GoogleOAuthService{
		users:       users,
		identities:  identities,
		auth:        auth,
		provider:    provider,
		cfg:         cfg,
		stateSecret: stateSecret,
	}
}

func (s *GoogleOAuthService) Start() (*GoogleOAuthStartResult, error) {
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

	return &GoogleOAuthStartResult{
		AuthorizationURL: s.provider.AuthorizationURL(state),
		StateNonce:       nonce,
	}, nil
}

func (s *GoogleOAuthService) HandleCallback(ctx context.Context, input GoogleOAuthCallbackInput) (*GoogleOAuthCallbackResult, error) {
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

	now := time.Now().UTC()
	if err := s.users.TouchLastLogin(user.ID, now); err != nil {
		return nil, err
	}
	user.LastLoginAt = &now

	authResult, err := s.auth.IssueWebSessionForUser(user)
	if err != nil {
		return nil, err
	}

	return &GoogleOAuthCallbackResult{
		AuthResult: authResult,
		User:       ToUserProfile(user),
		Linked:     linked,
	}, nil
}

func (s *GoogleOAuthService) SuccessRedirectURL() string {
	return strings.TrimSpace(s.cfg.SuccessRedirectURL)
}

func (s *GoogleOAuthService) FailureRedirectURL() string {
	return strings.TrimSpace(s.cfg.FailureRedirectURL)
}

func (s *GoogleOAuthService) StateTTL() time.Duration {
	if s.cfg.StateTTL <= 0 {
		return 10 * time.Minute
	}
	return s.cfg.StateTTL
}

func (s *GoogleOAuthService) isConfigured() bool {
	return s.cfg.Enabled &&
		strings.TrimSpace(s.cfg.ClientID) != "" &&
		strings.TrimSpace(s.cfg.ClientSecret) != "" &&
		strings.TrimSpace(s.cfg.CallbackURL) != ""
}

func (s *GoogleOAuthService) signState(nonce string) (string, error) {
	now := time.Now().UTC()
	claims := googleOAuthStateClaims{
		Provider: GoogleOAuthProviderName,
		Nonce:    nonce,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "lazyops-google-oauth-state",
			Subject:   GoogleOAuthProviderName,
			ExpiresAt: jwt.NewNumericDate(now.Add(s.StateTTL())),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.stateSecret))
}

func (s *GoogleOAuthService) validateState(state, nonce string) error {
	if strings.TrimSpace(state) == "" || strings.TrimSpace(nonce) == "" {
		return ErrInvalidOAuthState
	}

	claims := &googleOAuthStateClaims{}
	parsed, err := jwt.ParseWithClaims(
		state,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, ErrInvalidOAuthState
			}
			return []byte(s.stateSecret), nil
		},
		jwt.WithIssuer("lazyops-google-oauth-state"),
		jwt.WithSubject(GoogleOAuthProviderName),
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil || !parsed.Valid {
		return ErrInvalidOAuthState
	}
	if claims.Provider != GoogleOAuthProviderName || claims.Nonce != nonce {
		return ErrInvalidOAuthState
	}

	return nil
}

func (s *GoogleOAuthService) resolveUser(identity *GoogleIdentity) (*models.User, bool, error) {
	if identity == nil {
		return nil, false, ErrOAuthProviderFailure
	}

	normalizedEmail := strings.ToLower(strings.TrimSpace(identity.Email))
	if strings.TrimSpace(identity.Subject) == "" || !isValidEmail(normalizedEmail) {
		return nil, false, ErrOAuthProviderFailure
	}
	if !identity.EmailVerified {
		return nil, false, ErrOAuthProviderFailure
	}

	existingIdentity, err := s.identities.GetByProviderSubject(GoogleOAuthProviderName, identity.Subject)
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

	linkedIdentity := &models.OAuthIdentity{
		ID:              utils.NewPrefixedID("oid"),
		UserID:          user.ID,
		Provider:        GoogleOAuthProviderName,
		ProviderSubject: strings.TrimSpace(identity.Subject),
		Email:           normalizedEmail,
		AvatarURL:       strings.TrimSpace(identity.AvatarURL),
	}
	if err := s.identities.Create(linkedIdentity); err != nil {
		return nil, false, err
	}

	return user, linkedExistingUser, nil
}

func newOAuthNonce() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(raw), nil
}
