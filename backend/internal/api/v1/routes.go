package v1

import (
	"github.com/gin-gonic/gin"

	"lazyops-server/internal/api/middleware"
	"lazyops-server/internal/api/v1/controller"
	"lazyops-server/internal/bootstrap"
	"lazyops-server/internal/service"
)

func RegisterRoutes(router *gin.Engine, app *bootstrap.Application) {
	healthController := controller.NewHealthController(app.Config)
	authController := controller.NewAuthController(app.AuthService, app.GoogleOAuthService, app.GitHubOAuthService, app.Config)
	githubController := controller.NewGitHubController(app.GitHubInstallSvc)
	integrationController := controller.NewIntegrationController(app.GitHubWebhookSvc)
	buildController := controller.NewBuildController(app.BuildCallbackSvc)
	projectController := controller.NewProjectController(app.ProjectService, app.ProjectRepoLinkSvc)
	deploymentBindingController := controller.NewDeploymentBindingController(app.DeploymentBindingSvc)
	initContractController := controller.NewInitContractController(app.InitContractSvc)
	blueprintController := controller.NewBlueprintController(app.BlueprintSvc)
	deploymentController := controller.NewDeploymentController(app.DeploymentSvc, app.RolloutExecutionSvc)
	instanceController := controller.NewInstanceController(app.InstanceService)
	targetController := controller.NewTargetController(app.MeshNetworkService, app.ClusterService)
	observabilityController := controller.NewObservabilityController(app.ProjectRepo, app.ObservabilitySvc)
	tunnelController := controller.NewTunnelController(app.ProjectRepo, app.DeploymentBindingRepo, app.TunnelSessionRepo, app.MeshPlanningSvc)
	agentRuntimeController := controller.NewAgentRuntimeController(app.AgentEnrollmentSvc)
	userController := controller.NewUserController(app.UserService)
	agentController := controller.NewAgentController(app.AgentService, app.Hub)
	wsController := controller.NewWebSocketController(app.Hub, app.AgentService, app.Config)
	agentControlController := controller.NewAgentControlController(app.ControlHub, app.CommandTracker, app.ObservabilitySvc, app.Config)
	operatorStreamController := controller.NewOperatorStreamController(app.OperatorStreamHub, app.Config)

	rootAgentControl := router.Group("/ws")
	rootAgentControl.Use(middleware.Authenticate(app.AuthService))
	rootAgentControl.Use(middleware.RequireAuthKinds(service.AuthKindAgentToken))
	{
		rootAgentControl.GET("/agents/control", agentControlController.ControlStream)
	}

	rootOperatorStream := router.Group("/ws")
	rootOperatorStream.Use(middleware.Authenticate(app.AuthService))
	rootOperatorStream.Use(middleware.RequireAuthKinds(service.AuthKindWebSession, service.AuthKindCLIPAT))
	rootOperatorStream.Use(middleware.RequireRoles(service.RoleAdmin, service.RoleOperator))
	{
		rootOperatorStream.GET("/operators/stream", operatorStreamController.OperatorStream)
	}

	rootLogsStream := router.Group("/ws")
	rootLogsStream.Use(middleware.Authenticate(app.AuthService))
	rootLogsStream.Use(middleware.RequireAuthKinds(service.AuthKindWebSession, service.AuthKindCLIPAT))
	{
		rootLogsStream.GET("/logs/stream", observabilityController.StreamLogs)
	}

	v1 := router.Group("/api/v1")
	{
		v1.GET("/health", healthController.Health)
		v1.POST("/integrations/github/webhook", integrationController.GitHubWebhook)
		v1.POST("/builds/callback", buildController.Callback)
		v1.POST("/agents/enroll", agentRuntimeController.Enroll)

		authGroup := v1.Group("/auth")
		{
			authGroup.POST("/login", authController.Login)
			authGroup.POST("/register", authController.Register)
			authGroup.GET("/oauth/google/start", authController.GoogleOAuthStart)
			authGroup.GET("/oauth/google/callback", authController.GoogleOAuthCallback)
			authGroup.GET("/oauth/github/start", authController.GitHubOAuthStart)
			authGroup.GET("/oauth/github/callback", authController.GitHubOAuthCallback)
			authGroup.POST(
				"/cli-login",
				middleware.ScopedRateLimit(
					"auth:cli-login",
					app.Config.Security.CLILoginRateLimitRPS,
					app.Config.Security.CLILoginRateLimitBurst,
				),
				authController.CLILogin,
			)
		}

		userProtected := v1.Group("/")
		userProtected.Use(middleware.Authenticate(app.AuthService))
		userProtected.Use(middleware.RequireAuthKinds(service.AuthKindWebSession, service.AuthKindCLIPAT))
		{
			userProtected.POST("/auth/pat/revoke", authController.RevokePAT)
			userProtected.POST("/github/app/installations/sync", githubController.SyncInstallations)
			userProtected.GET("/github/repos", githubController.ListRepos)
			userProtected.POST("/projects", projectController.Create)
			userProtected.GET("/projects", projectController.List)
			userProtected.POST("/projects/:id/repo-link", projectController.LinkRepo)
			userProtected.GET("/projects/:id/deployment-bindings", deploymentBindingController.List)
			userProtected.POST("/projects/:id/deployment-bindings",
				middleware.RequireRoles(service.RoleAdmin, service.RoleOperator),
				deploymentBindingController.Create,
			)
			userProtected.POST("/projects/:id/init/validate-lazyops-yaml",
				middleware.RequireRoles(service.RoleAdmin, service.RoleOperator),
				initContractController.ValidateLazyopsYAML,
			)
			userProtected.PUT("/projects/:id/blueprint",
				middleware.RequireRoles(service.RoleAdmin, service.RoleOperator),
				blueprintController.Compile,
			)
			userProtected.POST("/projects/:id/deployments",
				middleware.RequireRoles(service.RoleAdmin, service.RoleOperator),
				deploymentController.Create,
			)
			userProtected.GET("/projects/:id/topology", observabilityController.GetTopology)
			userProtected.GET("/traces/:correlation_id", observabilityController.GetTrace)
			userProtected.GET("/ws/logs/stream", observabilityController.StreamLogs)
			userProtected.GET("/observability/correlate", observabilityController.GetCorrelatedObservability)
			userProtected.GET("/traces/:correlation_id/logs", observabilityController.GetTraceLogs)
			userProtected.GET("/topology/:node_ref/logs", observabilityController.GetTopologyNodeLogs)
			userProtected.POST("/observability/query", observabilityController.QueryObservability)
			userProtected.POST("/instances", instanceController.Create)
			userProtected.GET("/instances", instanceController.List)
			userProtected.POST("/mesh-networks", targetController.CreateMeshNetwork)
			userProtected.GET("/mesh-networks", targetController.ListMeshNetworks)
			userProtected.POST("/clusters", targetController.CreateCluster)
			userProtected.GET("/clusters", targetController.ListClusters)
			userProtected.POST("/tunnels/db/sessions",
				middleware.RequireRoles(service.RoleAdmin, service.RoleOperator),
				tunnelController.CreateDBSession,
			)
			userProtected.POST("/tunnels/tcp/sessions",
				middleware.RequireRoles(service.RoleAdmin, service.RoleOperator),
				tunnelController.CreateTCPSession,
			)
			userProtected.DELETE("/tunnels/sessions/:id",
				middleware.RequireRoles(service.RoleAdmin, service.RoleOperator),
				tunnelController.CloseSession,
			)
			userProtected.GET("/users/me", userController.Me)
			userProtected.GET("/agents", agentController.List)
			userProtected.POST("/agents",
				middleware.RequireRoles(service.RoleAdmin, service.RoleOperator),
				agentController.Create,
			)
			userProtected.PUT("/agents/:agentID/status",
				middleware.RequireRoles(service.RoleAdmin, service.RoleOperator),
				agentController.UpdateStatus,
			)
			userProtected.GET("/ws/agents", wsController.AgentStream)
		}

		agentProtected := v1.Group("/")
		agentProtected.Use(middleware.Authenticate(app.AuthService))
		agentProtected.Use(middleware.RequireAuthKinds(service.AuthKindAgentToken))
		{
			agentProtected.POST("/agents/heartbeat", agentRuntimeController.Heartbeat)
			agentProtected.GET("/ws/agents/control", agentControlController.ControlStream)
		}

		operatorProtected := v1.Group("/")
		operatorProtected.Use(middleware.Authenticate(app.AuthService))
		operatorProtected.Use(middleware.RequireAuthKinds(service.AuthKindWebSession, service.AuthKindCLIPAT))
		operatorProtected.Use(middleware.RequireRoles(service.RoleAdmin, service.RoleOperator))
		{
			operatorProtected.GET("/ws/operators/stream", operatorStreamController.OperatorStream)
		}

		operatorProtected.POST("/agents/:agent_id/dispatch",
			middleware.RequireRoles(service.RoleAdmin, service.RoleOperator),
			agentControlController.DispatchCommand,
		)
	}
}
