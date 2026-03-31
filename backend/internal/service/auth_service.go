package service

import (
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
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleViewer   = "viewer"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailExists        = errors.New("email already exists")
	ErrInvalidInput       = errors.New("invalid input")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrWeakPassword       = errors.New("weak password")
	ErrAccountDisabled    = errors.New("account disabled")
	ErrAccessDenied       = errors.New("access denied")
)

type AuthService struct {
	users UserStore
	cfg   config.JWTConfig
}

type Claims struct {
	AuthKind string `json:"auth_kind"`
	UserID   string `json:"user_id"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func NewAuthService(users UserStore, cfg config.JWTConfig) *AuthService {
	return &AuthService{users: users, cfg: cfg}
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

	return s.issueToken(user)
}

func (s *AuthService) Login(cmd LoginCommand) (*AuthResult, error) {
	email := strings.ToLower(strings.TrimSpace(cmd.Email))
	if !isValidEmail(email) || strings.TrimSpace(cmd.Password) == "" {
		return nil, ErrInvalidInput
	}

	user, err := s.users.GetByEmail(email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrInvalidCredentials
	}
	if user.Status != "active" {
		return nil, ErrAccountDisabled
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(cmd.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	now := time.Now().UTC()
	if err := s.users.TouchLastLogin(user.ID, now); err != nil {
		return nil, err
	}
	user.LastLoginAt = &now

	return s.issueToken(user)
}

func (s *AuthService) ParseToken(token string) (*Claims, error) {
	claims := &Claims{}
	parsed, err := jwt.ParseWithClaims(
		token,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, ErrUnauthorized
			}
			return []byte(s.cfg.Secret), nil
		},
		jwt.WithIssuer(s.cfg.Issuer),
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil || !parsed.Valid {
		return nil, ErrUnauthorized
	}
	if claims.AuthKind != "web_session" || claims.UserID == "" {
		return nil, ErrUnauthorized
	}

	return claims, nil
}

func (s *AuthService) issueToken(user *models.User) (*AuthResult, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(s.cfg.ExpiresIn)
	claims := Claims{
		AuthKind: "web_session",
		UserID:   user.ID,
		Email:    user.Email,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.cfg.Issuer,
			Subject:   user.ID,
			ID:        utils.NewPrefixedID("sess"),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.cfg.Secret))
	if err != nil {
		return nil, err
	}

	return &AuthResult{
		AccessToken: signed,
		TokenType:   "Bearer",
		ExpiresIn:   s.cfg.ExpiresIn,
		User:        ToUserProfile(user),
	}, nil
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
