package middlewares

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func ZapLogger(l *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		method := c.Request.Method
		uri := c.Request.RequestURI

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		size := max(c.Writer.Size(), 0)

		l.Info("http_request",
			zap.String("method", method),
			zap.String("uri", uri),
			zap.Int("status", status),
			zap.Int("size", size),
			zap.Duration("duration", latency),
		)
	}
}
