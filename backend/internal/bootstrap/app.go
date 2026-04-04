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
	Config                config.Config
	DB                    *gorm.DB
	Hub                   *hub.Hub
	AI                    *ai.GeminiClient
	UserRepo              *repository.UserRepository
	OAuthIdentityRepo     *repository.OAuthIdentityRepository
	GitHubInstallRepo     *repository.GitHubInstallationRepository
	ProjectRepo           *repository.ProjectRepository
	ProjectRepoLinkRepo   *repository.ProjectRepoLinkRepository
	BuildJobRepo          *repository.BuildJobRepository
	DeploymentBindingRepo *repository.DeploymentBindingRepository
	ServiceRepo           *repository.ServiceRepository
	BlueprintRepo         *repository.BlueprintRepository
	RevisionRepo          *repository.DesiredStateRevisionRepository
	DeploymentRepo        *repository.DeploymentRepository
	InstanceRepo          *repository.InstanceRepository
	MeshNetworkRepo       *repository.MeshNetworkRepository
	ClusterRepo           *repository.ClusterRepository
	BootstrapTokenRepo    *repository.BootstrapTokenRepository
	AgentTokenRepo        *repository.AgentTokenRepository
	PATRepo               *repository.PersonalAccessTokenRepository
	AgentRepo             *repository.AgentRepository
	AuthService           *service.AuthService
	GoogleOAuthService    *service.GoogleOAuthService
	GitHubOAuthService    *service.GitHubOAuthService
	GitHubInstallSvc      *service.GitHubInstallationService
	GitHubWebhookSvc      *service.GitHubWebhookService
	BuildCallbackSvc      *service.BuildCallbackService
	ProjectService        *service.ProjectService
	ProjectRepoLinkSvc    *service.ProjectRepoLinkService
	BuildJobSvc           *service.BuildJobService
	DeploymentBindingSvc  *service.DeploymentBindingService
	InitContractSvc       *service.InitContractService
	BlueprintSvc          *service.BlueprintService
	DeploymentSvc         *service.DeploymentService
	InstanceService       *service.InstanceService
	MeshNetworkService    *service.MeshNetworkService
	ClusterService        *service.ClusterService
	AgentEnrollmentSvc    *service.AgentEnrollmentService
	UserService           *service.UserService
	AgentService          *service.AgentService
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
	buildJobRepo := repository.NewBuildJobRepository(db)
	deploymentBindingRepo := repository.NewDeploymentBindingRepository(db)
	serviceRepo := repository.NewServiceRepository(db)
	blueprintRepo := repository.NewBlueprintRepository(db)
	revisionRepo := repository.NewDesiredStateRevisionRepository(db)
	deploymentRepo := repository.NewDeploymentRepository(db)
	instanceRepo := repository.NewInstanceRepository(db)
	meshNetworkRepo := repository.NewMeshNetworkRepository(db)
	clusterRepo := repository.NewClusterRepository(db)
	bootstrapTokenRepo := repository.NewBootstrapTokenRepository(db)
	agentTokenRepo := repository.NewAgentTokenRepository(db)
	patRepo := repository.NewPersonalAccessTokenRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	authService := service.NewAuthService(userRepo, patRepo, cfg.JWT, cfg.PAT).WithAgentTokens(agentTokenRepo)
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
	buildJobSvc := service.NewBuildJobService(projectRepoLinkRepo, buildJobRepo)
	deploymentBindingSvc := service.NewDeploymentBindingService(projectRepo, deploymentBindingRepo, instanceRepo, meshNetworkRepo, clusterRepo)
	initContractSvc := service.NewInitContractService(projectRepo, deploymentBindingRepo, instanceRepo, meshNetworkRepo, clusterRepo)
	blueprintSvc := service.NewBlueprintService(projectRepo, projectRepoLinkRepo, deploymentBindingRepo, serviceRepo, blueprintRepo)
	deploymentSvc := service.NewDeploymentService(projectRepo, blueprintRepo, revisionRepo, deploymentRepo)
	githubWebhookSvc := service.NewGitHubWebhookService(cfg.GitHubApp.WebhookSecret, projectRepoLinkSvc).WithBuildDispatcher(buildJobSvc)
	instanceService := service.NewInstanceService(instanceRepo, bootstrapTokenRepo, cfg.Enrollment)
	meshNetworkService := service.NewMeshNetworkService(meshNetworkRepo)
	clusterService := service.NewClusterService(clusterRepo)
	agentEnrollmentSvc := service.NewAgentEnrollmentService(agentRepo, instanceRepo, bootstrapTokenRepo, agentTokenRepo, cfg.Enrollment)
	userService := service.NewUserService(userRepo)
	agentService := service.NewAgentService(agentRepo)
	wsHub := hub.New()
	wsHub.Start()
	buildCallbackSvc := service.NewBuildCallbackService(projectRepo, blueprintRepo, revisionRepo, buildJobRepo, wsHub)

	return &Application{
		Config:                cfg,
		DB:                    db,
		Hub:                   wsHub,
		AI:                    ai.NewGeminiClient(""),
		UserRepo:              userRepo,
		OAuthIdentityRepo:     oauthIdentityRepo,
		GitHubInstallRepo:     githubInstallRepo,
		ProjectRepo:           projectRepo,
		ProjectRepoLinkRepo:   projectRepoLinkRepo,
		BuildJobRepo:          buildJobRepo,
		DeploymentBindingRepo: deploymentBindingRepo,
		ServiceRepo:           serviceRepo,
		BlueprintRepo:         blueprintRepo,
		RevisionRepo:          revisionRepo,
		DeploymentRepo:        deploymentRepo,
		InstanceRepo:          instanceRepo,
		MeshNetworkRepo:       meshNetworkRepo,
		ClusterRepo:           clusterRepo,
		BootstrapTokenRepo:    bootstrapTokenRepo,
		AgentTokenRepo:        agentTokenRepo,
		PATRepo:               patRepo,
		AgentRepo:             agentRepo,
		AuthService:           authService,
		GoogleOAuthService:    googleOAuthService,
		GitHubOAuthService:    githubOAuthService,
		GitHubInstallSvc:      githubInstallSvc,
		GitHubWebhookSvc:      githubWebhookSvc,
		BuildCallbackSvc:      buildCallbackSvc,
		ProjectService:        projectService,
		ProjectRepoLinkSvc:    projectRepoLinkSvc,
		BuildJobSvc:           buildJobSvc,
		DeploymentBindingSvc:  deploymentBindingSvc,
		InitContractSvc:       initContractSvc,
		BlueprintSvc:          blueprintSvc,
		DeploymentSvc:         deploymentSvc,
		InstanceService:       instanceService,
		MeshNetworkService:    meshNetworkService,
		ClusterService:        clusterService,
		AgentEnrollmentSvc:    agentEnrollmentSvc,
		UserService:           userService,
		AgentService:          agentService,
	}, nil
}
