package bootstrap

import (
	"database/sql"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"lazyops-server/internal/config"
	"lazyops-server/internal/models"
)

func NewDatabase(cfg config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.PostgresDSN()), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	configureSQLDB(sqlDB, cfg)

	return db, nil
}

func configureSQLDB(sqlDB *sql.DB, cfg config.Config) {
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
}

func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.OAuthIdentity{},
		&models.PersonalAccessToken{},
		&models.GitHubInstallation{},
		&models.Agent{},
	)
}
