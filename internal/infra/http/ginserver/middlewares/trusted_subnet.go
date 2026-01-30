package middlewares

import (
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// TrustedSubnet rejects requests with X-Real-IP outside the configured subnet.
func TrustedSubnet(subnet *net.IPNet) gin.HandlerFunc {
	if subnet == nil {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	return func(c *gin.Context) {
		raw := strings.TrimSpace(c.GetHeader("X-Real-IP"))
		if raw == "" {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		ip := net.ParseIP(raw)
		if ip == nil {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		if !subnet.Contains(ip) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	}
}
