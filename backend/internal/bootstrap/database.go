package bootstrap

import (
	"database/sql"
	"sort"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"lazyops-server/internal/config"
	"lazyops-server/internal/models"
	"lazyops-server/internal/repository"
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
	if err := migrateBuildJobLegacyColumns(db); err != nil {
		return err
	}
	if err := migrateBuildJobGitHubDeliveryColumn(db); err != nil {
		return err
	}

	if err := db.AutoMigrate(
		&models.User{},
		&models.OAuthIdentity{},
		&models.PersonalAccessToken{},
		&models.GitHubInstallation{},
		&models.Project{},
		&models.ProjectDomain{},
		&models.ProjectRepoLink{},
		&models.ProjectInternalService{},
		&models.ProjectEnvBundle{},
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
		&models.MetricRollup{},
		&models.LogStreamEntry{},
		&models.RoutingPolicy{},
	); err != nil {
		return err
	}
	if err := migrateProjectPivotMetadata(db); err != nil {
		return err
	}
	return migrateLegacyInternalServicesToManagedServices(db)
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

func migrateBuildJobGitHubDeliveryColumn(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if !db.Migrator().HasTable("build_jobs") {
		return nil
	}
	if !db.Migrator().HasColumn("build_jobs", "github_delivery_id") {
		if err := db.Exec(`ALTER TABLE build_jobs ADD COLUMN github_delivery_id VARCHAR(255) NOT NULL DEFAULT ''`).Error; err != nil {
			return err
		}
	}
	if !db.Migrator().HasColumn("build_jobs", "github_installation_id") {
		if err := db.Exec(`ALTER TABLE build_jobs ADD COLUMN github_installation_id BIGINT NOT NULL DEFAULT 0`).Error; err != nil {
			return err
		}
	}
	if !db.Migrator().HasColumn("build_jobs", "github_repo_id") {
		if err := db.Exec(`ALTER TABLE build_jobs ADD COLUMN github_repo_id BIGINT NOT NULL DEFAULT 0`).Error; err != nil {
			return err
		}
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_build_jobs_github_delivery_id ON build_jobs(github_delivery_id)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_build_jobs_github_installation_id ON build_jobs(github_installation_id)`).Error; err != nil {
		return err
	}
	return db.Exec(`CREATE INDEX IF NOT EXISTS idx_build_jobs_github_repo_id ON build_jobs(github_repo_id)`).Error
}

func migrateBuildJobLegacyColumns(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if !db.Migrator().HasTable("build_jobs") {
		return nil
	}

	if db.Migrator().HasColumn("build_jobs", "git_hub_delivery_id") &&
		!db.Migrator().HasColumn("build_jobs", "github_delivery_id") {
		if err := db.Exec(`ALTER TABLE build_jobs RENAME COLUMN git_hub_delivery_id TO github_delivery_id`).Error; err != nil {
			return err
		}
	}
	if db.Migrator().HasColumn("build_jobs", "git_hub_installation_id") &&
		!db.Migrator().HasColumn("build_jobs", "github_installation_id") {
		if err := db.Exec(`ALTER TABLE build_jobs RENAME COLUMN git_hub_installation_id TO github_installation_id`).Error; err != nil {
			return err
		}
	}
	if db.Migrator().HasColumn("build_jobs", "git_hub_repo_id") &&
		!db.Migrator().HasColumn("build_jobs", "github_repo_id") {
		if err := db.Exec(`ALTER TABLE build_jobs RENAME COLUMN git_hub_repo_id TO github_repo_id`).Error; err != nil {
			return err
		}
	}

	return nil
}

func migrateProjectPivotMetadata(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.Project{}) {
		return nil
	}

	var projects []models.Project
	if err := db.Find(&projects).Error; err != nil {
		return err
	}
	if len(projects) == 0 {
		return nil
	}

	bindingsByProject := map[string][]models.DeploymentBinding{}
	if db.Migrator().HasTable(&models.DeploymentBinding{}) {
		var bindings []models.DeploymentBinding
		if err := db.Order("created_at ASC").Find(&bindings).Error; err != nil {
			return err
		}
		for _, binding := range bindings {
			bindingsByProject[binding.ProjectID] = append(bindingsByProject[binding.ProjectID], binding)
		}
	}

	for _, project := range projects {
		updates := map[string]any{}
		namespace := normalizeProjectNamespaceSlug(project.NamespaceSlug, project.Slug, project.Name)
		if namespace != "" && strings.TrimSpace(project.NamespaceSlug) != namespace {
			updates["namespace_slug"] = namespace
		}

		inferredRuntimeMode, inferredClusterID := inferProjectPivotFromBindings(bindingsByProject[project.ID])
		if inferredRuntimeMode != "" && strings.TrimSpace(project.RuntimeMode) != inferredRuntimeMode {
			updates["runtime_mode"] = inferredRuntimeMode
		}

		currentClusterID := ""
		if project.ClusterID != nil {
			currentClusterID = strings.TrimSpace(*project.ClusterID)
		}
		if inferredClusterID != "" && currentClusterID != inferredClusterID {
			updates["cluster_id"] = inferredClusterID
		}

		if len(updates) == 0 {
			continue
		}
		if err := db.Model(&models.Project{}).Where("id = ?", project.ID).Updates(updates).Error; err != nil {
			return err
		}
	}

	return nil
}

func inferProjectPivotFromBindings(items []models.DeploymentBinding) (string, string) {
	if len(items) == 0 {
		return "", ""
	}

	sorted := append([]models.DeploymentBinding(nil), items...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].CreatedAt.Before(sorted[j].CreatedAt)
	})

	for _, item := range sorted {
		if strings.TrimSpace(item.RuntimeMode) == "distributed-k3s" || strings.TrimSpace(item.TargetKind) == "cluster" {
			return "distributed-k3s", strings.TrimSpace(item.TargetID)
		}
	}
	for _, item := range sorted {
		if strings.TrimSpace(item.RuntimeMode) == "distributed-mesh" || strings.TrimSpace(item.TargetKind) == "mesh" {
			return "distributed-mesh", ""
		}
	}
	for _, item := range sorted {
		if strings.TrimSpace(item.RuntimeMode) == "standalone" || strings.TrimSpace(item.TargetKind) == "instance" {
			return "standalone", ""
		}
	}

	return strings.TrimSpace(sorted[0].RuntimeMode), ""
}

func normalizeProjectNamespaceSlug(values ...string) string {
	var source string
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			source = strings.TrimSpace(value)
			break
		}
	}
	if source == "" {
		return ""
	}

	source = strings.ToLower(strings.TrimSpace(source))
	var b strings.Builder
	lastDash := false
	for _, r := range source {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if b.Len() > 0 && !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}

	slug := strings.Trim(b.String(), "-")
	if len(slug) > 63 {
		slug = strings.Trim(slug[:63], "-")
	}
	return slug
}

func migrateLegacyInternalServicesToManagedServices(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.ProjectInternalService{}) || !db.Migrator().HasTable(&models.Service{}) {
		return nil
	}

	var items []models.ProjectInternalService
	if err := db.Order("project_id ASC, kind ASC").Find(&items).Error; err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}

	grouped := make(map[string][]models.ProjectInternalService)
	projectIDs := make([]string, 0)
	seenProjects := make(map[string]struct{})
	for _, item := range items {
		projectID := strings.TrimSpace(item.ProjectID)
		grouped[projectID] = append(grouped[projectID], item)
		if _, exists := seenProjects[projectID]; exists {
			continue
		}
		seenProjects[projectID] = struct{}{}
		projectIDs = append(projectIDs, projectID)
	}
	sort.Strings(projectIDs)

	store := repository.NewManagedInternalServiceRepository(db, nil)
	for _, projectID := range projectIDs {
		if err := store.ReplaceForProject(projectID, grouped[projectID]); err != nil {
			return err
		}
	}

	return nil
}
