package service

import (
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"lazyops-server/internal/config"
	"lazyops-server/internal/models"
	"lazyops-server/internal/repository"
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
)

type AuthService struct {
	users *repository.UserRepository
	cfg   config.JWTConfig
}

type Claims struct {
	UserID uint   `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func NewAuthService(users *repository.UserRepository, cfg config.JWTConfig) *AuthService {
	return &AuthService{users: users, cfg: cfg}
}

func (s *AuthService) Register(cmd RegisterCommand) (*AuthResult, error) {
	email := strings.ToLower(strings.TrimSpace(cmd.Email))
	if email == "" || strings.TrimSpace(cmd.Password) == "" || strings.TrimSpace(cmd.Name) == "" {
		return nil, ErrInvalidInput
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
		Name:         strings.TrimSpace(cmd.Name),
		Email:        email,
		PasswordHash: string(passwordHash),
		Role:         normalizeRole(cmd.Role),
	}
	if err := s.users.Create(user); err != nil {
		return nil, err
	}

	return s.issueToken(user)
}

func (s *AuthService) Login(cmd LoginCommand) (*AuthResult, error) {
	email := strings.ToLower(strings.TrimSpace(cmd.Email))
	if email == "" || strings.TrimSpace(cmd.Password) == "" {
		return nil, ErrInvalidInput
	}

	user, err := s.users.GetByEmail(email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(cmd.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	return s.issueToken(user)
}

func (s *AuthService) ParseToken(token string) (*Claims, error) {
	claims := &Claims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (any, error) {
		return []byte(s.cfg.Secret), nil
	})
	if err != nil || !parsed.Valid {
		return nil, ErrUnauthorized
	}

	return claims, nil
}

func (s *AuthService) issueToken(user *models.User) (*AuthResult, error) {
	now := time.Now()
	expiresAt := now.Add(s.cfg.ExpiresIn)
	claims := Claims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.cfg.Issuer,
			Subject:   user.Email,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
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
		ID:    user.ID,
		Name:  user.Name,
		Email: user.Email,
		Role:  user.Role,
	}
}

func normalizeRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case RoleAdmin:
		return RoleAdmin
	case RoleOperator:
		return RoleOperator
	default:
		return RoleViewer
	}
}
