package api

import (
	"github.com/gin-gonic/gin"

	"lazyops-server/internal/api/middleware"
	v1 "lazyops-server/internal/api/v1"
	"lazyops-server/internal/bootstrap"
)

func NewRouter(app *bootstrap.Application) *gin.Engine {
	gin.SetMode(app.Config.Server.GinMode)

	router := gin.New()
	router.Use(middleware.RequestID())
	router.Use(middleware.Recovery())
	router.Use(middleware.RequestLogger())
	router.Use(middleware.SecurityHeaders())
	router.Use(middleware.CORS(app.Config.Security.AllowedOrigins))
	router.Use(middleware.Timeout(app.Config.Server.RequestTimeout))
	router.Use(middleware.RateLimit(
		app.Config.Security.RateLimitRPS,
		app.Config.Security.RateLimitBurst,
	))

	v1.RegisterRoutes(router, app)

	return router
}
