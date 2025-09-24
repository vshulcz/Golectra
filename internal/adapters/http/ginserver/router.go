package ginserver

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func NewRouter(h *Handler, _ *zap.Logger, middlewares ...gin.HandlerFunc) *gin.Engine {
	r := gin.New()

	r.Use(gin.Recovery())
	for _, mw := range middlewares {
		r.Use(mw)
	}

	r.RedirectTrailingSlash = false
	r.RemoveExtraSlash = true

	r.HandleMethodNotAllowed = true
	r.NoMethod(func(c *gin.Context) {
		c.String(http.StatusMethodNotAllowed, "method not allowed")
	})

	r.GET("/ping", h.Ping)

	r.POST("/update/:type/:name/:value", h.UpdateMetric)
	r.GET("/value/:type/:name", h.GetMetric)
	r.GET("/", h.Index)

	// JSON endpoints
	r.POST("/update", h.UpdateMetricJSON)
	r.POST("/update/", h.UpdateMetricJSON)
	r.POST("/value", h.GetMetricJSON)
	r.POST("/value/", h.GetMetricJSON)

	r.POST("/updates", h.UpdateMetricsBatchJSON)
	r.POST("/updates/", h.UpdateMetricsBatchJSON)

	return r
}
