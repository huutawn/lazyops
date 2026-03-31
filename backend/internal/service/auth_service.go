package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"lazyops-server/internal/config"
	"lazyops-server/internal/models"
	"lazyops-server/pkg/utils"
)

const (
	RoleAdmin            = "admin"
	RoleOperator         = "operator"
	RoleViewer           = "viewer"
	WebSessionCookieName = "lazyops_session"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailExists        = errors.New("email already exists")
	ErrInvalidInput       = errors.New("invalid input")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrWeakPassword       = errors.New("weak password")
	ErrAccountDisabled    = errors.New("account disabled")
	ErrAccessDenied       = errors.New("access denied")
	ErrTokenNotFound      = errors.New("token not found")
	ErrTokenAccessDenied  = errors.New("token access denied")
	ErrTokenRevoked       = errors.New("token revoked")
	ErrTokenExpired       = errors.New("token expired")
)

type AuthService struct {
	users  UserStore
	pats   PATStore
	jwtCfg config.JWTConfig
	patCfg config.PATConfig
}

type Claims struct {
	AuthKind string `json:"auth_kind"`
	UserID   string `json:"user_id"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	TokenID  string `json:"token_id,omitempty"`
	jwt.RegisteredClaims
}

func NewAuthService(users UserStore, pats PATStore, jwtCfg config.JWTConfig, patCfg config.PATConfig) *AuthService {
	return &AuthService{
		users:  users,
		pats:   pats,
		jwtCfg: jwtCfg,
		patCfg: patCfg,
	}
}

func (s *AuthService) Register(cmd RegisterCommand) (*AuthResult, error) {
	email := strings.ToLower(strings.TrimSpace(cmd.Email))
	displayName := utils.NormalizeSpace(cmd.Name)
	if !isValidEmail(email) || strings.TrimSpace(cmd.Password) == "" || displayName == "" {
		return nil, ErrInvalidInput
	}
	if err := validatePassword(cmd.Password); err != nil {
		return nil, err
	}

	existing, err := s.users.GetByEmail(email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrEmailExists
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(cmd.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &models.User{
		ID:           utils.NewPrefixedID("usr"),
		DisplayName:  displayName,
		Email:        email,
		PasswordHash: string(passwordHash),
		Role:         RoleViewer,
		Status:       "active",
	}
	if err := s.users.Create(user); err != nil {
		return nil, err
	}

	return s.issueWebSession(user)
}

func (s *AuthService) Login(cmd LoginCommand) (*AuthResult, error) {
	user, err := s.authenticatePasswordUser(cmd.Email, cmd.Password)
	if err != nil {
		return nil, err
	}

	return s.issueWebSession(user)
}

func (s *AuthService) CLILogin(cmd CLILoginCommand) (*CLIAuthResult, error) {
	if strings.ToLower(strings.TrimSpace(cmd.AuthFlow)) != "password" {
		return nil, ErrInvalidInput
	}

	deviceName := utils.NormalizeSpace(cmd.DeviceName)
	if deviceName == "" {
		return nil, ErrInvalidInput
	}

	user, err := s.authenticatePasswordUser(cmd.Email, cmd.Password)
	if err != nil {
		return nil, err
	}

	return s.issuePAT(user, deviceName)
}

func (s *AuthService) RevokePAT(cmd RevokePATCommand) (*PATRevokeResult, error) {
	tokenID := strings.TrimSpace(cmd.TokenID)
	if strings.TrimSpace(cmd.UserID) == "" || tokenID == "" {
		return nil, ErrInvalidInput
	}

	token, err := s.pats.GetByID(tokenID)
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, ErrTokenNotFound
	}
	if token.UserID != cmd.UserID {
		return nil, ErrTokenAccessDenied
	}

	if token.RevokedAt == nil {
		now := time.Now().UTC()
		if err := s.pats.RevokeByIDForUser(cmd.UserID, token.ID, now); err != nil {
			return nil, err
		}
	}

	return &PATRevokeResult{
		TokenID: token.ID,
		Revoked: true,
	}, nil
}

func (s *AuthService) ParseToken(token string) (*Claims, error) {
	if looksLikeJWT(token) {
		return s.parseWebSessionToken(token)
	}

	return s.parsePATToken(token)
}

func (s *AuthService) parseWebSessionToken(token string) (*Claims, error) {
	claims := &Claims{}
	parsed, err := jwt.ParseWithClaims(
		token,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, ErrUnauthorized
			}
			return []byte(s.jwtCfg.Secret), nil
		},
		jwt.WithIssuer(s.jwtCfg.Issuer),
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) || errors.Is(err, jwt.ErrTokenNotValidYet) {
			return nil, ErrTokenExpired
		}
		return nil, ErrUnauthorized
	}
	if !parsed.Valid {
		return nil, ErrUnauthorized
	}
	if claims.AuthKind != "web_session" || claims.UserID == "" {
		return nil, ErrUnauthorized
	}

	user, err := s.validateActiveUser(claims.UserID)
	if err != nil {
		return nil, err
	}
	claims.Email = user.Email
	claims.Role = user.Role
	claims.Subject = user.ID

	return claims, nil
}

func (s *AuthService) parsePATToken(token string) (*Claims, error) {
	tokenHash := hashOpaqueToken(token)
	record, err := s.pats.GetByHash(tokenHash)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, ErrUnauthorized
	}
	if record.RevokedAt != nil {
		return nil, ErrTokenRevoked
	}
	if record.ExpiresAt != nil && time.Now().UTC().After(*record.ExpiresAt) {
		return nil, ErrTokenExpired
	}

	user, err := s.validateActiveUser(record.UserID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	if err := s.pats.TouchLastUsed(record.ID, now); err != nil {
		return nil, err
	}

	claims := &Claims{
		AuthKind: "cli_pat",
		UserID:   user.ID,
		Email:    user.Email,
		Role:     user.Role,
		TokenID:  record.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:  s.jwtCfg.Issuer,
			Subject: user.ID,
			ID:      record.ID,
		},
	}
	if !record.CreatedAt.IsZero() {
		claims.IssuedAt = jwt.NewNumericDate(record.CreatedAt)
	}
	if record.ExpiresAt != nil {
		claims.ExpiresAt = jwt.NewNumericDate(*record.ExpiresAt)
	}

	return claims, nil
}

func (s *AuthService) IssueWebSessionForUser(user *models.User) (*AuthResult, error) {
	return s.issueWebSession(user)
}

func (s *AuthService) issueWebSession(user *models.User) (*AuthResult, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(s.jwtCfg.ExpiresIn)
	claims := Claims{
		AuthKind: "web_session",
		UserID:   user.ID,
		Email:    user.Email,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.jwtCfg.Issuer,
			Subject:   user.ID,
			ID:        utils.NewPrefixedID("sess"),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.jwtCfg.Secret))
	if err != nil {
		return nil, err
	}

	return &AuthResult{
		AccessToken: signed,
		TokenType:   "Bearer",
		ExpiresIn:   s.jwtCfg.ExpiresIn,
		User:        ToUserProfile(user),
	}, nil
}

func (s *AuthService) issuePAT(user *models.User, deviceName string) (*CLIAuthResult, error) {
	rawToken, tokenPrefix, err := newOpaquePAT()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	expiresAt := now.Add(s.patCfg.ExpiresIn)
	record := &models.PersonalAccessToken{
		ID:          utils.NewPrefixedID("pat"),
		UserID:      user.ID,
		Name:        deviceName,
		TokenHash:   hashOpaqueToken(rawToken),
		TokenPrefix: tokenPrefix,
		ExpiresAt:   &expiresAt,
	}
	if err := s.pats.Create(record); err != nil {
		return nil, err
	}

	return &CLIAuthResult{
		Token:     rawToken,
		TokenType: "Bearer",
		TokenID:   record.ID,
		ExpiresAt: &expiresAt,
		User:      ToUserProfile(user),
	}, nil
}

func (s *AuthService) authenticatePasswordUser(email, password string) (*models.User, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if !isValidEmail(normalizedEmail) || strings.TrimSpace(password) == "" {
		return nil, ErrInvalidInput
	}

	user, err := s.users.GetByEmail(normalizedEmail)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrInvalidCredentials
	}
	if user.Status != "active" {
		return nil, ErrAccountDisabled
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	now := time.Now().UTC()
	if err := s.users.TouchLastLogin(user.ID, now); err != nil {
		return nil, err
	}
	user.LastLoginAt = &now

	return user, nil
}

func (s *AuthService) validateActiveUser(userID string) (*models.User, error) {
	user, err := s.users.GetByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUnauthorized
	}
	if user.Status != "active" {
		return nil, ErrAccountDisabled
	}
	return user, nil
}

func ToUserProfile(user *models.User) UserProfile {
	return UserProfile{
		ID:          user.ID,
		DisplayName: user.DisplayName,
		Email:       user.Email,
		Role:        user.Role,
		Status:      user.Status,
		LastLoginAt: user.LastLoginAt,
	}
}

func isValidEmail(email string) bool {
	if email == "" {
		return false
	}
	addr, err := mail.ParseAddress(email)
	return err == nil && strings.EqualFold(addr.Address, email)
}

func validatePassword(password string) error {
	if len(password) < 8 || len(password) > 72 {
		return ErrWeakPassword
	}

	var hasUpper, hasLower, hasDigit bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}

	if !hasUpper || !hasLower || !hasDigit {
		return ErrWeakPassword
	}

	return nil
}

func looksLikeJWT(token string) bool {
	return strings.Count(token, ".") == 2
}

func newOpaquePAT() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}

	token := "lop_pat_" + base64.RawURLEncoding.EncodeToString(raw)
	prefix := token
	if len(prefix) > 16 {
		prefix = prefix[:16]
	}

	return token, prefix, nil
}

func hashOpaqueToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
