package controller

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"lazyops-server/internal/api/response"
	requestdto "lazyops-server/internal/api/v1/dto/request"
	"lazyops-server/internal/api/v1/mapper"
	"lazyops-server/internal/service"
)

type AuthController struct {
	auth *service.AuthService
}

func NewAuthController(auth *service.AuthService) *AuthController {
	return &AuthController{auth: auth}
}

func (ctl *AuthController) Register(c *gin.Context) {
	var req requestdto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}

	result, err := ctl.auth.Register(mapper.ToRegisterCommand(req))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrEmailExists):
			response.Error(c, http.StatusConflict, "register failed", "email_already_exists", err.Error())
		case errors.Is(err, service.ErrWeakPassword):
			response.Error(c, http.StatusBadRequest, "register failed", "weak_password", err.Error())
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "register failed", "invalid_input", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "register failed", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusCreated, "register successful", mapper.ToAuthResponse(*result))
}

func (ctl *AuthController) Login(c *gin.Context) {
	var req requestdto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}

	result, err := ctl.auth.Login(mapper.ToLoginCommand(req))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			response.Error(c, http.StatusUnauthorized, "login failed", "invalid_credentials", err.Error())
		case errors.Is(err, service.ErrAccountDisabled):
			response.Error(c, http.StatusUnauthorized, "login failed", "account_disabled", err.Error())
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "login failed", "invalid_input", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "login failed", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusOK, "login successful", mapper.ToAuthResponse(*result))
}
