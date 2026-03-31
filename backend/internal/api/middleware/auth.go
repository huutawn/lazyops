package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"lazyops-server/internal/api/response"
	"lazyops-server/internal/service"
)

const claimsContextKey = "auth.claims"

func Authenticate(auth *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			response.Error(c, http.StatusUnauthorized, "missing bearer token", "missing_bearer_token", nil)
			c.Abort()
			return
		}

		token := strings.TrimPrefix(header, "Bearer ")
		if strings.TrimSpace(token) == "" {
			response.Error(c, http.StatusUnauthorized, "invalid authorization header", "invalid_authorization_header", nil)
			c.Abort()
			return
		}

		claims, err := auth.ParseToken(token)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "invalid token", "invalid_token", err.Error())
			c.Abort()
			return
		}

		c.Set(claimsContextKey, claims)
		c.Next()
	}
}

func RequireRoles(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := MustClaims(c)
		for _, role := range roles {
			if claims.Role == role {
				c.Next()
				return
			}
		}

		response.Error(c, http.StatusForbidden, "insufficient permissions", "insufficient_permissions", nil)
		c.Abort()
	}
}

func MustClaims(c *gin.Context) *service.Claims {
	claims, exists := c.Get(claimsContextKey)
	if !exists {
		panic("auth claims missing from context")
	}

	return claims.(*service.Claims)
}
