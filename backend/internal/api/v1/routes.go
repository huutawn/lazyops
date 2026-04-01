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
	userController := controller.NewUserController(app.UserService)
	agentController := controller.NewAgentController(app.AgentService, app.Hub)
	wsController := controller.NewWebSocketController(app.Hub, app.AgentService, app.Config)

	v1 := router.Group("/api/v1")
	{
		v1.GET("/health", healthController.Health)
		v1.POST("/integrations/github/webhook", integrationController.GitHubWebhook)

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

		protected := v1.Group("/")
		protected.Use(middleware.Authenticate(app.AuthService))
		{
			protected.POST("/auth/pat/revoke", authController.RevokePAT)
			protected.POST("/github/app/installations/sync", githubController.SyncInstallations)
			protected.GET("/github/repos", githubController.ListRepos)
			protected.POST("/projects", projectController.Create)
			protected.GET("/projects", projectController.List)
			protected.POST("/projects/:id/repo-link", projectController.LinkRepo)
			protected.GET("/users/me", userController.Me)
			protected.GET("/agents", agentController.List)
			protected.POST("/agents",
				middleware.RequireRoles(service.RoleAdmin, service.RoleOperator),
				agentController.Create,
			)
			protected.PUT("/agents/:agentID/status",
				middleware.RequireRoles(service.RoleAdmin, service.RoleOperator),
				agentController.UpdateStatus,
			)
			protected.GET("/ws/agents", wsController.AgentStream)
		}
	}
}
