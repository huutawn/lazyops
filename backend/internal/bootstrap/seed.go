package bootstrap

import (
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"lazyops-server/internal/config"
	"lazyops-server/internal/models"
	"lazyops-server/internal/service"
)

func SeedAdmin(db *gorm.DB, cfg config.Config) error {
	email := strings.ToLower(strings.TrimSpace(cfg.Seed.AdminEmail))
	if email == "" || strings.TrimSpace(cfg.Seed.AdminPassword) == "" {
		return nil
	}

	var existing models.User
	err := db.Where("email = ?", email).First(&existing).Error
	if err == nil {
		return nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(cfg.Seed.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	admin := models.User{
		Name:         cfg.Seed.AdminName,
		Email:        email,
		PasswordHash: string(passwordHash),
		Role:         service.RoleAdmin,
	}

	return db.Create(&admin).Error
}
