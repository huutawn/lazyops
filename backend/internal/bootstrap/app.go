package bootstrap

import (
	"lazyops-server/internal/ai"
	"lazyops-server/internal/config"
	gh "lazyops-server/internal/github"
	"lazyops-server/internal/hub"
	"lazyops-server/internal/oauth"
	"lazyops-server/internal/repository"
	"lazyops-server/internal/service"

	"gorm.io/gorm"
)

type Application struct {
	Config              config.Config
	DB                  *gorm.DB
	Hub                 *hub.Hub
	AI                  *ai.GeminiClient
	UserRepo            *repository.UserRepository
	OAuthIdentityRepo   *repository.OAuthIdentityRepository
	GitHubInstallRepo   *repository.GitHubInstallationRepository
	ProjectRepo         *repository.ProjectRepository
	ProjectRepoLinkRepo *repository.ProjectRepoLinkRepository
	PATRepo             *repository.PersonalAccessTokenRepository
	AgentRepo           *repository.AgentRepository
	AuthService         *service.AuthService
	GoogleOAuthService  *service.GoogleOAuthService
	GitHubOAuthService  *service.GitHubOAuthService
	GitHubInstallSvc    *service.GitHubInstallationService
	GitHubWebhookSvc    *service.GitHubWebhookService
	ProjectService      *service.ProjectService
	ProjectRepoLinkSvc  *service.ProjectRepoLinkService
	UserService         *service.UserService
	AgentService        *service.AgentService
}

func NewApplication(cfg config.Config) (*Application, error) {
	db, err := NewDatabase(cfg)
	if err != nil {
		return nil, err
	}

	if err := Migrate(db); err != nil {
		return nil, err
	}
	if err := SeedAdmin(db, cfg); err != nil {
		return nil, err
	}

	userRepo := repository.NewUserRepository(db)
	oauthIdentityRepo := repository.NewOAuthIdentityRepository(db)
	githubInstallRepo := repository.NewGitHubInstallationRepository(db)
	projectRepo := repository.NewProjectRepository(db)
	projectRepoLinkRepo := repository.NewProjectRepoLinkRepository(db)
	patRepo := repository.NewPersonalAccessTokenRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	authService := service.NewAuthService(userRepo, patRepo, cfg.JWT, cfg.PAT)
	googleProvider := oauth.NewGoogleProvider(cfg.GoogleOAuth, nil)
	googleOAuthService := service.NewGoogleOAuthService(
		userRepo,
		oauthIdentityRepo,
		authService,
		googleProvider,
		cfg.JWT.Secret,
		cfg.GoogleOAuth,
	)
	githubProvider := oauth.NewGitHubProvider(cfg.GitHubOAuth, nil)
	githubOAuthService := service.NewGitHubOAuthService(
		userRepo,
		oauthIdentityRepo,
		authService,
		githubProvider,
		cfg.JWT.Secret,
		cfg.GitHubOAuth,
	)
	githubInstallProvider := gh.NewAppInstallationsProvider(nil)
	githubInstallSvc := service.NewGitHubInstallationService(
		oauthIdentityRepo,
		githubInstallRepo,
		githubInstallProvider,
	)
	projectService := service.NewProjectService(projectRepo)
	projectRepoLinkSvc := service.NewProjectRepoLinkService(projectRepo, githubInstallRepo, projectRepoLinkRepo)
	githubWebhookSvc := service.NewGitHubWebhookService(cfg.GitHubApp.WebhookSecret, projectRepoLinkSvc)
	userService := service.NewUserService(userRepo)
	agentService := service.NewAgentService(agentRepo)
	wsHub := hub.New()
	wsHub.Start()

	return &Application{
		Config:              cfg,
		DB:                  db,
		Hub:                 wsHub,
		AI:                  ai.NewGeminiClient(""),
		UserRepo:            userRepo,
		OAuthIdentityRepo:   oauthIdentityRepo,
		GitHubInstallRepo:   githubInstallRepo,
		ProjectRepo:         projectRepo,
		ProjectRepoLinkRepo: projectRepoLinkRepo,
		PATRepo:             patRepo,
		AgentRepo:           agentRepo,
		AuthService:         authService,
		GoogleOAuthService:  googleOAuthService,
		GitHubOAuthService:  githubOAuthService,
		GitHubInstallSvc:    githubInstallSvc,
		GitHubWebhookSvc:    githubWebhookSvc,
		ProjectService:      projectService,
		ProjectRepoLinkSvc:  projectRepoLinkSvc,
		UserService:         userService,
		AgentService:        agentService,
	}, nil
}
