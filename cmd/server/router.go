package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func NewRouter(h *Handler) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.RedirectTrailingSlash = false
	r.RemoveExtraSlash = true

	r.HandleMethodNotAllowed = true
	r.NoMethod(func(c *gin.Context) {
		c.String(http.StatusMethodNotAllowed, "method not allowed")
	})

	r.POST("/update/:type/:name/:value", h.UpdateMetric)
	r.GET("/value/:type/:name", h.GetMetric)
	r.GET("/", h.Index)

	return r
}
