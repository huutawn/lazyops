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
	projectController := controller.NewProjectController(app.ProjectService, app.ProjectRepoLinkSvc)
	deploymentBindingController := controller.NewDeploymentBindingController(app.DeploymentBindingSvc)
	instanceController := controller.NewInstanceController(app.InstanceService)
	targetController := controller.NewTargetController(app.MeshNetworkService, app.ClusterService)
	agentRuntimeController := controller.NewAgentRuntimeController(app.AgentEnrollmentSvc)
	userController := controller.NewUserController(app.UserService)
	agentController := controller.NewAgentController(app.AgentService, app.Hub)
	wsController := controller.NewWebSocketController(app.Hub, app.AgentService, app.Config)

	v1 := router.Group("/api/v1")
	{
		v1.GET("/health", healthController.Health)
		v1.POST("/integrations/github/webhook", integrationController.GitHubWebhook)
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
			userProtected.POST("/projects/:id/deployment-bindings",
				middleware.RequireRoles(service.RoleAdmin, service.RoleOperator),
				deploymentBindingController.Create,
			)
			userProtected.POST("/instances", instanceController.Create)
			userProtected.GET("/instances", instanceController.List)
			userProtected.POST("/mesh-networks", targetController.CreateMeshNetwork)
			userProtected.GET("/mesh-networks", targetController.ListMeshNetworks)
			userProtected.POST("/clusters", targetController.CreateCluster)
			userProtected.GET("/clusters", targetController.ListClusters)
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
		}
	}
}
