package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"lazyops-server/internal/api/response"
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var (
	visitors    = map[string]*visitor{}
	visitorsMu  sync.Mutex
	cleanupOnce sync.Once
)

func RateLimit(rps float64, burst int) gin.HandlerFunc {
	cleanupOnce.Do(func() {
		go cleanupVisitors()
	})

	return func(c *gin.Context) {
		limiter := getVisitor(c.ClientIP(), rps, burst)
		if !limiter.Allow() {
			response.Error(c, http.StatusTooManyRequests, "rate limit exceeded", nil)
			c.Abort()
			return
		}

		c.Next()
	}
}

func getVisitor(ip string, rps float64, burst int) *rate.Limiter {
	visitorsMu.Lock()
	defer visitorsMu.Unlock()

	entry, exists := visitors[ip]
	if !exists {
		limiter := rate.NewLimiter(rate.Limit(rps), burst)
		visitors[ip] = &visitor{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}

	entry.lastSeen = time.Now()
	return entry.limiter
}

func cleanupVisitors() {
	for {
		time.Sleep(time.Minute)

		visitorsMu.Lock()
		for ip, entry := range visitors {
			if time.Since(entry.lastSeen) > 3*time.Minute {
				delete(visitors, ip)
			}
		}
		visitorsMu.Unlock()
	}
}
