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
	if err := migrateProjectRepoLinkLegacyColumns(db); err != nil {
		return err
	}

	return db.AutoMigrate(
		&models.User{},
		&models.OAuthIdentity{},
		&models.PersonalAccessToken{},
		&models.GitHubInstallation{},
		&models.Project{},
		&models.ProjectRepoLink{},
		&models.ProjectInternalService{},
		&models.BuildJob{},
		&models.DeploymentBinding{},
		&models.Service{},
		&models.Blueprint{},
		&models.DesiredStateRevision{},
		&models.Deployment{},
		&models.Instance{},
		&models.MeshNetwork{},
		&models.Cluster{},
		&models.BootstrapToken{},
		&models.Agent{},
		&models.AgentToken{},
		&models.RuntimeIncident{},
		&models.PublicRoute{},
		&models.GatewayConfigIntent{},
		&models.ReleaseHistory{},
		&models.PreviewEnvironment{},
		&models.TunnelSession{},
		&models.TopologyState{},
		&models.TraceSummary{},
		&models.TopologyNode{},
		&models.TopologyEdge{},
		&models.LogStreamEntry{},
		&models.RoutingPolicy{},
	)
}

func migrateProjectRepoLinkLegacyColumns(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if !db.Migrator().HasTable(&models.ProjectRepoLink{}) {
		return nil
	}

	hasLegacyInstall := db.Migrator().HasColumn("project_repo_links", "git_hub_installation_id")
	hasCanonicalInstall := db.Migrator().HasColumn("project_repo_links", "github_installation_id")
	if hasLegacyInstall && !hasCanonicalInstall {
		if err := db.Exec(`ALTER TABLE project_repo_links RENAME COLUMN git_hub_installation_id TO github_installation_id`).Error; err != nil {
			return err
		}
	}

	hasLegacyRepo := db.Migrator().HasColumn("project_repo_links", "git_hub_repo_id")
	hasCanonicalRepo := db.Migrator().HasColumn("project_repo_links", "github_repo_id")
	if hasLegacyRepo && !hasCanonicalRepo {
		if err := db.Exec(`ALTER TABLE project_repo_links RENAME COLUMN git_hub_repo_id TO github_repo_id`).Error; err != nil {
			return err
		}
	}

	return nil
}
