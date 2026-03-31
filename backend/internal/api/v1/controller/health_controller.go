package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"lazyops-server/internal/api/response"
	"lazyops-server/internal/config"
)

type HealthController struct {
	cfg config.Config
}

func NewHealthController(cfg config.Config) *HealthController {
	return &HealthController{cfg: cfg}
}

func (ctl *HealthController) Health(c *gin.Context) {
	response.JSON(c, http.StatusOK, "service healthy", gin.H{
		"service": ctl.cfg.App.Name,
		"env":     ctl.cfg.App.Environment,
		"version": "v1",
	})
}
