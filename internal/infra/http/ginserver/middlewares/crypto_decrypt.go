package middlewares

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/vshulcz/Golectra/internal/ports"
)

// DecryptPayload decrypts request bodies that carry the encryption header.
func DecryptPayload(dec ports.PayloadDecrypter) gin.HandlerFunc {
	if dec == nil {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	return func(c *gin.Context) {
		if !strings.EqualFold(c.GetHeader(dec.HeaderKey()), dec.HeaderValue()) {
			c.Next()
			return
		}

		raw, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "read body failed"})
			return
		}
		if err := c.Request.Body.Close(); err != nil {
			_ = c.Error(err)
		}
		plain, err := dec.Decrypt(raw)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "decrypt failed"})
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(plain))
		c.Request.ContentLength = int64(len(plain))
		c.Next()
	}
}
