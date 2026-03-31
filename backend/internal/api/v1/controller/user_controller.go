package controller

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"lazyops-server/internal/api/middleware"
	"lazyops-server/internal/api/response"
	"lazyops-server/internal/api/v1/mapper"
	"lazyops-server/internal/service"
)

type UserController struct {
	users *service.UserService
}

func NewUserController(users *service.UserService) *UserController {
	return &UserController{users: users}
}

func (ctl *UserController) Me(c *gin.Context) {
	claims := middleware.MustClaims(c)
	profile, err := ctl.users.GetProfile(claims.UserID)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			response.Error(c, http.StatusNotFound, "user not found", "user_not_found", err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "failed to load profile", "internal_error", err.Error())
		return
	}

	response.JSON(c, http.StatusOK, "profile loaded", mapper.ToUserResponse(*profile))
}
