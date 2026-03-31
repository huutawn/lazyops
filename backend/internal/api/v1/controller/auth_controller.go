package controller

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"lazyops-server/internal/api/middleware"
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

func (ctl *AuthController) CLILogin(c *gin.Context) {
	var req requestdto.CLILoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}

	result, err := ctl.auth.CLILogin(mapper.ToCLILoginCommand(req))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			response.Error(c, http.StatusUnauthorized, "cli login failed", "invalid_credentials", err.Error())
		case errors.Is(err, service.ErrAccountDisabled):
			response.Error(c, http.StatusUnauthorized, "cli login failed", "account_disabled", err.Error())
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "cli login failed", "invalid_input", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "cli login failed", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusOK, "cli login successful", mapper.ToCLILoginResponse(*result))
}

func (ctl *AuthController) RevokePAT(c *gin.Context) {
	var req requestdto.PATRevokeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid request payload", "invalid_payload", err.Error())
		return
	}

	claims := middleware.MustClaims(c)
	result, err := ctl.auth.RevokePAT(mapper.ToPATRevokeCommand(claims.UserID, req))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "pat revoke failed", "invalid_input", err.Error())
		case errors.Is(err, service.ErrTokenNotFound):
			response.Error(c, http.StatusNotFound, "pat revoke failed", "token_not_found", err.Error())
		case errors.Is(err, service.ErrTokenAccessDenied):
			response.Error(c, http.StatusForbidden, "pat revoke failed", "token_access_denied", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "pat revoke failed", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusOK, "pat revoked", mapper.ToPATRevokeResponse(*result))
}
