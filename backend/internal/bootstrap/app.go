package bootstrap

import (
	"lazyops-server/internal/ai"
	"lazyops-server/internal/config"
	gh "lazyops-server/internal/github"
	"lazyops-server/internal/hub"
	"lazyops-server/internal/oauth"
	"lazyops-server/internal/repository"
	"lazyops-server/internal/runtime"
	"lazyops-server/internal/service"

	"gorm.io/gorm"
)

type Application struct {
	Config                 config.Config
	DB                     *gorm.DB
	Hub                    *hub.Hub
	AI                     *ai.GeminiClient
	UserRepo               *repository.UserRepository
	OAuthIdentityRepo      *repository.OAuthIdentityRepository
	GitHubInstallRepo      *repository.GitHubInstallationRepository
	ProjectRepo            *repository.ProjectRepository
	ProjectRepoLinkRepo    *repository.ProjectRepoLinkRepository
	ProjectInternalSvcRepo *repository.ProjectInternalServiceRepository
	BuildJobRepo           *repository.BuildJobRepository
	DeploymentBindingRepo  *repository.DeploymentBindingRepository
	ServiceRepo            *repository.ServiceRepository
	BlueprintRepo          *repository.BlueprintRepository
	RevisionRepo           *repository.DesiredStateRevisionRepository
	DeploymentRepo         *repository.DeploymentRepository
	InstanceRepo           *repository.InstanceRepository
	MeshNetworkRepo        *repository.MeshNetworkRepository
	ClusterRepo            *repository.ClusterRepository
	TunnelSessionRepo      *repository.TunnelSessionRepository
	TraceSummaryRepo       *repository.TraceSummaryRepository
	MetricRollupRepo       *repository.MetricRollupRepository
	LogStreamRepo          *repository.LogStreamRepository
	TopologyStateRepo      *repository.TopologyStateRepository
	TopologyNodeRepo       *repository.TopologyNodeRepository
	TopologyEdgeRepo       *repository.TopologyEdgeRepository
	BootstrapTokenRepo     *repository.BootstrapTokenRepository
	AgentTokenRepo         *repository.AgentTokenRepository
	PATRepo                *repository.PersonalAccessTokenRepository
	AgentRepo              *repository.AgentRepository
	AuthService            *service.AuthService
	GoogleOAuthService     *service.GoogleOAuthService
	GitHubOAuthService     *service.GitHubOAuthService
	GitHubInstallSvc       *service.GitHubInstallationService
	GitHubWebhookSvc       *service.GitHubWebhookService
	BuildCallbackSvc       *service.BuildCallbackService
	ProjectService         *service.ProjectService
	ProjectInternalSvc     *service.ProjectInternalServiceService
	ProjectRepoLinkSvc     *service.ProjectRepoLinkService
	BootstrapOrchestrator  *service.BootstrapOrchestrator
	BuildJobSvc            *service.BuildJobService
	DeploymentBindingSvc   *service.DeploymentBindingService
	InitContractSvc        *service.InitContractService
	BlueprintSvc           *service.BlueprintService
	DeploymentSvc          *service.DeploymentService
	InstanceService        *service.InstanceService
	InstanceSSHInstallSvc  *service.InstanceSSHInstallService
	MeshNetworkService     *service.MeshNetworkService
	ClusterService         *service.ClusterService
	MeshPlanningSvc        *service.MeshPlanningService
	ObservabilitySvc       *service.ObservabilityService
	AgentEnrollmentSvc     *service.AgentEnrollmentService
	UserService            *service.UserService
	AgentService           *service.AgentService
	ControlService         *service.ControlService
	ControlHub             *service.ControlHub
	CommandTracker         *service.CommandTracker
	OperatorStreamHub      *service.OperatorStreamHub
	RuntimeRegistry        *runtime.Registry
	RolloutPlanner         *service.RolloutPlanner
	RolloutExecutionSvc    *service.RolloutExecutionService
	IncidentRepo           *repository.RuntimeIncidentRepository
	PreviewRepo            *repository.PreviewEnvironmentRepository
	PreviewService         *service.PreviewEnvironmentService
	RoutingSvc             *service.RoutingService
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
	projectInternalSvcRepo := repository.NewProjectInternalServiceRepository(db)
	buildJobRepo := repository.NewBuildJobRepository(db)
	deploymentBindingRepo := repository.NewDeploymentBindingRepository(db)
	serviceRepo := repository.NewServiceRepository(db)
	blueprintRepo := repository.NewBlueprintRepository(db)
	revisionRepo := repository.NewDesiredStateRevisionRepository(db)
	deploymentRepo := repository.NewDeploymentRepository(db)
	instanceRepo := repository.NewInstanceRepository(db)
	meshNetworkRepo := repository.NewMeshNetworkRepository(db)
	clusterRepo := repository.NewClusterRepository(db)
	tunnelSessionRepo := repository.NewTunnelSessionRepository(db)
	traceSummaryRepo := repository.NewTraceSummaryRepository(db)
	metricRollupRepo := repository.NewMetricRollupRepository(db)
	logStreamRepo := repository.NewLogStreamRepository(db)
	topologyStateRepo := repository.NewTopologyStateRepository(db)
	topologyNodeRepo := repository.NewTopologyNodeRepository(db)
	topologyEdgeRepo := repository.NewTopologyEdgeRepository(db)
	bootstrapTokenRepo := repository.NewBootstrapTokenRepository(db)
	agentTokenRepo := repository.NewAgentTokenRepository(db)
	patRepo := repository.NewPersonalAccessTokenRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	incidentRepo := repository.NewRuntimeIncidentRepository(db)
	previewRepo := repository.NewPreviewEnvironmentRepository(db)
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
	githubOAuthService.WithInstallationSync(githubInstallSvc)
	projectService := service.NewProjectService(projectRepo, projectInternalSvcRepo)
	projectInternalSvc := service.NewProjectInternalServiceService(projectRepo, projectInternalSvcRepo)
	projectRepoLinkSvc := service.NewProjectRepoLinkService(projectRepo, githubInstallRepo, projectRepoLinkRepo)
	buildJobSvc := service.NewBuildJobService(projectRepoLinkRepo, buildJobRepo)
	deploymentBindingSvc := service.NewDeploymentBindingService(projectRepo, deploymentBindingRepo, instanceRepo, meshNetworkRepo, clusterRepo)
	bootstrapOrchestrator := service.NewBootstrapOrchestrator(
		projectRepo,
		projectService,
		projectRepoLinkSvc,
		projectRepoLinkRepo,
		deploymentBindingSvc,
		deploymentBindingRepo,
		deploymentRepo,
		instanceRepo,
		meshNetworkRepo,
		clusterRepo,
		githubInstallRepo,
	)
	bootstrapOrchestrator.WithInternalServiceStore(projectInternalSvcRepo)
	initContractSvc := service.NewInitContractService(projectRepo, deploymentBindingRepo, instanceRepo, meshNetworkRepo, clusterRepo)
	blueprintSvc := service.NewBlueprintService(projectRepo, projectRepoLinkRepo, deploymentBindingRepo, serviceRepo, blueprintRepo)
	deploymentSvc := service.NewDeploymentService(projectRepo, blueprintRepo, revisionRepo, deploymentRepo).
		WithIncidentStore(incidentRepo)
	githubWebhookSvc := service.NewGitHubWebhookService(cfg.GitHubApp.WebhookSecret, projectRepoLinkSvc).WithBuildDispatcher(buildJobSvc)
	instanceService := service.NewInstanceService(instanceRepo, bootstrapTokenRepo, cfg.Enrollment)
	instanceSSHInstallSvc := service.NewInstanceSSHInstallService(instanceService, service.NewNativeSSHExecutor()).
		WithBootstrapOrchestrator(bootstrapOrchestrator)
	meshNetworkService := service.NewMeshNetworkService(meshNetworkRepo)
	clusterService := service.NewClusterService(clusterRepo)
	meshPlanningSvc := service.NewMeshPlanningService(instanceRepo, deploymentBindingRepo, revisionRepo, tunnelSessionRepo, topologyStateRepo)
	observabilitySvc := service.
		NewObservabilityService(traceSummaryRepo, incidentRepo, logStreamRepo, topologyNodeRepo, topologyEdgeRepo, instanceRepo, meshNetworkRepo, clusterRepo).
		WithBindingStore(deploymentBindingRepo).
		WithMetricRollupStore(metricRollupRepo)
	agentEnrollmentSvc := service.NewAgentEnrollmentService(agentRepo, instanceRepo, bootstrapTokenRepo, agentTokenRepo, cfg.Enrollment)
	userService := service.NewUserService(userRepo)
	agentService := service.NewAgentService(agentRepo)
	wsHub := hub.New()
	wsHub.Start()
	buildCallbackSvc := service.NewBuildCallbackService(projectRepo, blueprintRepo, revisionRepo, deploymentRepo, buildJobRepo, wsHub)
	controlHub := service.NewControlHub()
	controlHub.Start()
	operatorStreamHub := service.NewOperatorStreamHub()
	operatorStreamHub.Start()

	rtRegistry := runtime.NewRegistry()
	rtRegistry.Register(runtime.NewStandaloneDriver())
	rtRegistry.Register(runtime.NewDistributedMeshDriver())
	rtRegistry.Register(runtime.NewDistributedK3sDriver())
	commandTracker := service.NewCommandTracker()
	controlService := service.NewControlService(controlHub, commandTracker, rtRegistry, instanceRepo, agentRepo)
	projectInternalSvc.WithRuntimeProvisioner(deploymentBindingRepo, instanceRepo, controlService)

	rolloutPlanner := service.NewRolloutPlanner(
		rtRegistry,
		revisionRepo,
		deploymentRepo,
		incidentRepo,
		deploymentBindingRepo,
		operatorStreamHub,
	)
	rolloutExecutionSvc := service.NewRolloutExecutionService(
		deploymentSvc,
		rolloutPlanner,
		instanceRepo,
		controlService,
		operatorStreamHub,
	)
	buildCallbackSvc.WithRolloutStarter(rolloutExecutionSvc)
	bootstrapOrchestrator.WithOneClickPipeline(
		serviceRepo,
		initContractSvc,
		blueprintSvc,
		deploymentSvc,
		rolloutExecutionSvc,
	)

	previewService := service.NewPreviewEnvironmentService(
		projectRepo,
		projectRepoLinkRepo,
		revisionRepo,
		deploymentRepo,
		blueprintRepo,
		previewRepo,
		nil,
		operatorStreamHub,
	)

	routingPolicyRepo := repository.NewRoutingPolicyRepository(db)
	routingSvc := service.NewRoutingService(routingPolicyRepo, serviceRepo)

	return &Application{
		Config:                 cfg,
		DB:                     db,
		Hub:                    wsHub,
		AI:                     ai.NewGeminiClient(""),
		UserRepo:               userRepo,
		OAuthIdentityRepo:      oauthIdentityRepo,
		GitHubInstallRepo:      githubInstallRepo,
		ProjectRepo:            projectRepo,
		ProjectRepoLinkRepo:    projectRepoLinkRepo,
		ProjectInternalSvcRepo: projectInternalSvcRepo,
		BuildJobRepo:           buildJobRepo,
		DeploymentBindingRepo:  deploymentBindingRepo,
		ServiceRepo:            serviceRepo,
		BlueprintRepo:          blueprintRepo,
		RevisionRepo:           revisionRepo,
		DeploymentRepo:         deploymentRepo,
		InstanceRepo:           instanceRepo,
		MeshNetworkRepo:        meshNetworkRepo,
		ClusterRepo:            clusterRepo,
		TunnelSessionRepo:      tunnelSessionRepo,
		TraceSummaryRepo:       traceSummaryRepo,
		MetricRollupRepo:       metricRollupRepo,
		LogStreamRepo:          logStreamRepo,
		TopologyStateRepo:      topologyStateRepo,
		TopologyNodeRepo:       topologyNodeRepo,
		TopologyEdgeRepo:       topologyEdgeRepo,
		BootstrapTokenRepo:     bootstrapTokenRepo,
		AgentTokenRepo:         agentTokenRepo,
		PATRepo:                patRepo,
		AgentRepo:              agentRepo,
		AuthService:            authService,
		GoogleOAuthService:     googleOAuthService,
		GitHubOAuthService:     githubOAuthService,
		GitHubInstallSvc:       githubInstallSvc,
		GitHubWebhookSvc:       githubWebhookSvc,
		BuildCallbackSvc:       buildCallbackSvc,
		ProjectService:         projectService,
		ProjectInternalSvc:     projectInternalSvc,
		ProjectRepoLinkSvc:     projectRepoLinkSvc,
		BootstrapOrchestrator:  bootstrapOrchestrator,
		BuildJobSvc:            buildJobSvc,
		DeploymentBindingSvc:   deploymentBindingSvc,
		InitContractSvc:        initContractSvc,
		BlueprintSvc:           blueprintSvc,
		DeploymentSvc:          deploymentSvc,
		InstanceService:        instanceService,
		InstanceSSHInstallSvc:  instanceSSHInstallSvc,
		MeshNetworkService:     meshNetworkService,
		ClusterService:         clusterService,
		MeshPlanningSvc:        meshPlanningSvc,
		ObservabilitySvc:       observabilitySvc,
		AgentEnrollmentSvc:     agentEnrollmentSvc,
		UserService:            userService,
		AgentService:           agentService,
		ControlService:         controlService,
		ControlHub:             controlHub,
		CommandTracker:         commandTracker,
		OperatorStreamHub:      operatorStreamHub,
		RuntimeRegistry:        rtRegistry,
		RolloutPlanner:         rolloutPlanner,
		RolloutExecutionSvc:    rolloutExecutionSvc,
		IncidentRepo:           incidentRepo,
		PreviewRepo:            previewRepo,
		PreviewService:         previewService,
		RoutingSvc:             routingSvc,
	}, nil
}
