package middlewares

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type gzipReadCloser struct {
	gz  *gzip.Reader
	raw io.Closer
}

func (g *gzipReadCloser) Read(p []byte) (int, error) {
	return g.gz.Read(p)
}

func (g *gzipReadCloser) Close() error {
	if err := g.gz.Close(); err != nil {
		return err
	}
	if g.raw != nil {
		return g.raw.Close()
	}
	return nil
}

func GzipRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		if enc := strings.ToLower(c.GetHeader("Content-Encoding")); strings.Contains(enc, "gzip") {
			gr, err := gzip.NewReader(c.Request.Body)
			if err != nil {
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
			c.Request.Body = &gzipReadCloser{gz: gr, raw: c.Request.Body}
			c.Request.Header.Del("Content-Length")
		}
		c.Next()
	}
}

type gzipResponseWriter struct {
	gin.ResponseWriter
	gzw        *gzip.Writer
	compress   bool
	decided    bool
	acceptGzip bool
}

func (w *gzipResponseWriter) decide() {
	if w.decided {
		return
	}
	w.decided = true

	if !w.acceptGzip {
		return
	}
	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") && !strings.HasPrefix(ct, "text/html") {
		return
	}
	status := w.Status()
	if status == 204 || status < 200 {
		return
	}

	w.Header().Del("Content-Length")
	w.Header().Set("Content-Encoding", "gzip")
	w.gzw = gzip.NewWriter(w.ResponseWriter)
	w.compress = true
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	w.ResponseWriter.WriteHeader(code)
}

func (w *gzipResponseWriter) Write(p []byte) (int, error) {
	if !w.decided {
		w.decide()
	}
	if w.compress {
		return w.gzw.Write(p)
	}
	return w.ResponseWriter.Write(p)
}

func (w *gzipResponseWriter) Close() error {
	if w.gzw != nil {
		return w.gzw.Close()
	}
	return nil
}

func GzipResponse() gin.HandlerFunc {
	return func(c *gin.Context) {
		accept := strings.Contains(strings.ToLower(c.GetHeader("Accept-Encoding")), "gzip")
		if !accept {
			c.Next()
			return
		}
		grw := &gzipResponseWriter{ResponseWriter: c.Writer, acceptGzip: true}
		c.Writer = grw
		c.Next()
		if err := grw.Close(); err != nil {
			_ = c.Error(err)
		}
	}
}
