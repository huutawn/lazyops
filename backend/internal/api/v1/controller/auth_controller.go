package controller

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"lazyops-server/internal/api/middleware"
	"lazyops-server/internal/api/response"
	requestdto "lazyops-server/internal/api/v1/dto/request"
	"lazyops-server/internal/api/v1/mapper"
	"lazyops-server/internal/config"
	"lazyops-server/internal/service"
)

type AuthController struct {
	auth        *service.AuthService
	googleOAuth *service.GoogleOAuthService
	githubOAuth *service.GitHubOAuthService
	cfg         config.Config
}

func NewAuthController(
	auth *service.AuthService,
	googleOAuth *service.GoogleOAuthService,
	githubOAuth *service.GitHubOAuthService,
	cfg config.Config,
) *AuthController {
	return &AuthController{
		auth:        auth,
		googleOAuth: googleOAuth,
		githubOAuth: githubOAuth,
		cfg:         cfg,
	}
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

func (ctl *AuthController) GoogleOAuthStart(c *gin.Context) {
	result, err := ctl.googleOAuth.Start()
	if err != nil {
		if errors.Is(err, service.ErrOAuthNotConfigured) {
			response.Error(c, http.StatusServiceUnavailable, "google oauth not configured", "oauth_not_configured", nil)
			return
		}
		response.Error(c, http.StatusInternalServerError, "failed to start google oauth", "internal_error", err.Error())
		return
	}

	ctl.setGoogleStateCookie(c, result.StateNonce, int(ctl.googleOAuth.StateTTL().Seconds()))
	if strings.EqualFold(c.Query("mode"), "json") {
		response.JSON(c, http.StatusOK, "redirect ready", gin.H{
			"provider":          service.GoogleOAuthProviderName,
			"authorization_url": result.AuthorizationURL,
		})
		return
	}

	c.Redirect(http.StatusFound, result.AuthorizationURL)
}

func (ctl *AuthController) GoogleOAuthCallback(c *gin.Context) {
	stateNonce := strings.TrimSpace(c.GetHeader("X-LazyOps-OAuth-State-Nonce"))
	if cookieStateNonce, err := c.Cookie(service.GoogleOAuthStateCookie); err == nil && strings.TrimSpace(cookieStateNonce) != "" {
		stateNonce = cookieStateNonce
	}
	forceJSON := strings.EqualFold(c.Query("mode"), "json")
	result, err := ctl.googleOAuth.HandleCallback(c.Request.Context(), service.GoogleOAuthCallbackInput{
		State:         c.Query("state"),
		StateNonce:    stateNonce,
		Code:          c.Query("code"),
		ProviderError: c.Query("error"),
	})
	ctl.clearGoogleStateCookie(c)

	if err != nil {
		status, code := mapGoogleOAuthError(err)
		if !forceJSON {
			failureURL := strings.TrimSpace(ctl.googleOAuth.FailureRedirectURL())
			if failureURL == "" {
				failureURL = ctl.defaultOAuthFailureRedirect(c)
			}
			c.Redirect(http.StatusFound, appendQuery(failureURL, map[string]string{"error_code": code}))
			return
		}

		message := "google oauth failed"
		if code == "invalid_oauth_state" {
			message = "invalid oauth state"
		}
		if code == "account_disabled" {
			message = "account disabled"
		}
		response.Error(c, status, message, code, nil)
		return
	}

	ctl.setWebSessionCookie(c, result.AuthResult.AccessToken, int(result.AuthResult.ExpiresIn.Seconds()))
	if !forceJSON {
		successURL := strings.TrimSpace(ctl.googleOAuth.SuccessRedirectURL())
		if successURL == "" {
			successURL = ctl.defaultOAuthSuccessRedirect(c)
		}
		c.Redirect(http.StatusFound, appendQuery(successURL, map[string]string{"status": "success"}))
		return
	}

	response.JSON(c, http.StatusOK, "google oauth successful", mapper.ToAuthResponse(*result.AuthResult))
}

func (ctl *AuthController) GitHubOAuthStart(c *gin.Context) {
	result, err := ctl.githubOAuth.Start()
	if err != nil {
		if errors.Is(err, service.ErrOAuthNotConfigured) {
			response.Error(c, http.StatusServiceUnavailable, "github oauth not configured", "oauth_not_configured", nil)
			return
		}
		response.Error(c, http.StatusInternalServerError, "failed to start github oauth", "internal_error", err.Error())
		return
	}

	ctl.setStateCookie(c, service.GitHubOAuthStateCookie, result.StateNonce, int(ctl.githubOAuth.StateTTL().Seconds()))
	if strings.EqualFold(c.Query("mode"), "json") {
		response.JSON(c, http.StatusOK, "redirect ready", gin.H{
			"provider":          service.GitHubOAuthProviderName,
			"authorization_url": result.AuthorizationURL,
		})
		return
	}

	c.Redirect(http.StatusFound, result.AuthorizationURL)
}

func (ctl *AuthController) GitHubOAuthCallback(c *gin.Context) {
	stateNonce := strings.TrimSpace(c.GetHeader("X-LazyOps-OAuth-State-Nonce"))
	if cookieStateNonce, err := c.Cookie(service.GitHubOAuthStateCookie); err == nil && strings.TrimSpace(cookieStateNonce) != "" {
		stateNonce = cookieStateNonce
	}
	forceJSON := strings.EqualFold(c.Query("mode"), "json")
	result, err := ctl.githubOAuth.HandleCallback(c.Request.Context(), service.GitHubOAuthCallbackInput{
		State:         c.Query("state"),
		StateNonce:    stateNonce,
		Code:          c.Query("code"),
		ProviderError: c.Query("error"),
	})
	ctl.clearStateCookie(c, service.GitHubOAuthStateCookie)

	if err != nil {
		status, code := mapGitHubOAuthError(err)
		if !forceJSON {
			failureURL := strings.TrimSpace(ctl.githubOAuth.FailureRedirectURL())
			if failureURL == "" {
				failureURL = ctl.defaultOAuthFailureRedirect(c)
			}
			c.Redirect(http.StatusFound, appendQuery(failureURL, map[string]string{"error_code": code}))
			return
		}

		message := "github oauth failed"
		if code == "invalid_oauth_state" {
			message = "invalid oauth state"
		}
		if code == "account_disabled" {
			message = "account disabled"
		}
		response.Error(c, status, message, code, nil)
		return
	}

	ctl.setWebSessionCookie(c, result.AuthResult.AccessToken, int(result.AuthResult.ExpiresIn.Seconds()))
	if !forceJSON {
		successURL := strings.TrimSpace(ctl.githubOAuth.SuccessRedirectURL())
		if successURL == "" {
			successURL = ctl.defaultOAuthSuccessRedirect(c)
		}
		c.Redirect(http.StatusFound, appendQuery(successURL, map[string]string{"status": "success"}))
		return
	}

	response.JSON(c, http.StatusOK, "github oauth successful", mapper.ToAuthResponse(*result.AuthResult))
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

func (ctl *AuthController) setGoogleStateCookie(c *gin.Context, nonce string, maxAge int) {
	ctl.setStateCookie(c, service.GoogleOAuthStateCookie, nonce, maxAge)
}

func (ctl *AuthController) clearGoogleStateCookie(c *gin.Context) {
	ctl.clearStateCookie(c, service.GoogleOAuthStateCookie)
}

func (ctl *AuthController) setWebSessionCookie(c *gin.Context, token string, maxAge int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(service.WebSessionCookieName, token, maxAge, "/", "", ctl.shouldUseSecureCookies(), true)
}

func (ctl *AuthController) setStateCookie(c *gin.Context, name, value string, maxAge int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(name, value, maxAge, "/", "", ctl.shouldUseSecureCookies(), true)
}

func (ctl *AuthController) clearStateCookie(c *gin.Context, name string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(name, "", -1, "/", "", ctl.shouldUseSecureCookies(), true)
}

func (ctl *AuthController) shouldUseSecureCookies() bool {
	return strings.EqualFold(ctl.cfg.App.Environment, "production") ||
		strings.HasPrefix(strings.ToLower(ctl.cfg.GoogleOAuth.CallbackURL), "https://") ||
		strings.HasPrefix(strings.ToLower(ctl.cfg.GitHubOAuth.CallbackURL), "https://")
}

func (ctl *AuthController) defaultOAuthSuccessRedirect(c *gin.Context) string {
	return ctl.requestOrigin(c) + "/dashboard"
}

func (ctl *AuthController) defaultOAuthFailureRedirect(c *gin.Context) string {
	return ctl.requestOrigin(c) + "/login"
}

func (ctl *AuthController) requestOrigin(c *gin.Context) string {
	proto := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto"))
	if proto != "" {
		proto = strings.TrimSpace(strings.Split(proto, ",")[0])
	}
	if proto == "" {
		if c.Request.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}

	host := strings.TrimSpace(c.GetHeader("X-Forwarded-Host"))
	if host != "" {
		host = strings.TrimSpace(strings.Split(host, ",")[0])
	}
	if host == "" {
		host = strings.TrimSpace(c.Request.Host)
	}
	if host == "" {
		host = "localhost"
	}

	return strings.ToLower(proto) + "://" + host
}

func mapGoogleOAuthError(err error) (int, string) {
	switch {
	case errors.Is(err, service.ErrInvalidOAuthState):
		return http.StatusBadRequest, "invalid_oauth_state"
	case errors.Is(err, service.ErrOAuthNotConfigured):
		return http.StatusServiceUnavailable, "oauth_not_configured"
	case errors.Is(err, service.ErrAccountDisabled), errors.Is(err, service.ErrRevokedOAuthIdentity):
		return http.StatusUnauthorized, "account_disabled"
	default:
		return http.StatusBadGateway, "oauth_provider_error"
	}
}

func mapGitHubOAuthError(err error) (int, string) {
	switch {
	case errors.Is(err, service.ErrInvalidOAuthState):
		return http.StatusBadRequest, "invalid_oauth_state"
	case errors.Is(err, service.ErrOAuthNotConfigured):
		return http.StatusServiceUnavailable, "oauth_not_configured"
	case errors.Is(err, service.ErrGitHubInstallationsSyncFailed):
		return http.StatusBadGateway, "github_installations_sync_failed"
	case errors.Is(err, service.ErrAccountDisabled), errors.Is(err, service.ErrRevokedOAuthIdentity):
		return http.StatusUnauthorized, "account_disabled"
	default:
		return http.StatusBadGateway, "oauth_provider_error"
	}
}

func appendQuery(base string, params map[string]string) string {
	parsed, err := url.Parse(base)
	if err != nil {
		return base
	}

	query := parsed.Query()
	for key, value := range params {
		query.Set(key, value)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
