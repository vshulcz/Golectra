package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/vshulcz/Golectra/cmd/server/middlewares"
	"go.uber.org/zap"
)

func NewRouter(h *Handler, logger *zap.Logger) *gin.Engine {
	r := gin.New()

	r.Use(gin.Recovery())
	r.Use(middlewares.ZapLogger(logger))

	r.Use(middlewares.GunzipRequest())
	r.Use(middlewares.GzipResponse())

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

	return r
}
