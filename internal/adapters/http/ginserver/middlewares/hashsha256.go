package middlewares

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/vshulcz/Golectra/internal/misc"
)

type bodyBufferWriter struct {
	gin.ResponseWriter
	status int
	body   bytes.Buffer
}

func (w *bodyBufferWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(p)
}

func (w *bodyBufferWriter) WriteHeader(code int) {
	w.status = code
}

func HashSHA256(key string) gin.HandlerFunc {
	key = strings.TrimSpace(key)
	if key == "" {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	return func(c *gin.Context) {
		bw := &bodyBufferWriter{ResponseWriter: c.Writer}
		c.Writer = bw

		if got := strings.TrimSpace(c.GetHeader("HashSHA256")); got != "" {
			reqBody, err := io.ReadAll(c.Request.Body)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "read body failed"})
			} else {
				if err := c.Request.Body.Close(); err != nil {
					c.Error(err)
				}
				c.Request.Body = io.NopCloser(bytes.NewReader(reqBody))
				if len(reqBody) > 0 {
					want := misc.SumSHA256(reqBody, key)
					if !strings.EqualFold(got, want) {
						c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid hash"})
					}
				}
			}
		}

		if !c.IsAborted() {
			c.Next()
		}

		if bw.body.Len() > 0 {
			sum := misc.SumSHA256(bw.body.Bytes(), key)
			c.Header("HashSHA256", sum)
		}

		status := bw.status
		if status == 0 {
			status = http.StatusOK
		}

		c.Writer = bw.ResponseWriter
		c.Writer.WriteHeader(status)
		if _, err := c.Writer.Write(bw.body.Bytes()); err != nil {
			c.Error(err)
		}
	}
}
