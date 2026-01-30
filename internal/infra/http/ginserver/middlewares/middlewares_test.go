package middlewares

import (
	"bytes"
	"compress/gzip"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHashSHA256_ValidatesRequestAndSetsResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(HashSHA256("secret"))
	router.POST("/hash", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		c.Data(http.StatusOK, "text/plain", body)
	})

	plain := []byte("payload")
	req := httptest.NewRequest(http.MethodPost, "/hash", bytes.NewReader(plain))
	req.Header.Set("HashSHA256", sumSHA256(plain, "secret"))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("HashSHA256"); got == "" {
		t.Fatal("expected response HashSHA256 header")
	}
	if !bytes.Equal(rec.Body.Bytes(), plain) {
		t.Fatalf("body=%q want %q", rec.Body.Bytes(), plain)
	}
}

func TestHashSHA256_InvalidRejects(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(HashSHA256("secret"))
	router.POST("/hash", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/hash", strings.NewReader("payload"))
	req.Header.Set("HashSHA256", "bad")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHashSHA256_EmptyKey_IsNoop(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(HashSHA256("  "))
	router.POST("/noop", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/plain", []byte("ok"))
	})

	req := httptest.NewRequest(http.MethodPost, "/noop", strings.NewReader("payload"))
	req.Header.Set("HashSHA256", "bad")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("HashSHA256"); got != "" {
		t.Fatalf("unexpected HashSHA256 header: %q", got)
	}
}

func TestHashSHA256_EmptyBody_SkipsValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(HashSHA256("secret"))
	router.POST("/empty", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/plain", []byte("ok"))
	})

	req := httptest.NewRequest(http.MethodPost, "/empty", http.NoBody)
	req.Header.Set("HashSHA256", "bad")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusOK)
	}
}

func TestGzipRequest_Decompresses(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(GzipRequest())
	router.POST("/gzip", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		c.Data(http.StatusOK, "text/plain", body)
	})

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte("hello")); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/gzip", bytes.NewReader(buf.Bytes()))
	req.Header.Set("Content-Encoding", "gzip")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != "hello" {
		t.Fatalf("body=%q want %q", got, "hello")
	}
}

func TestTrustedSubnet_AllowsWhenUnset(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(TrustedSubnet(nil))
	router.GET("/ok", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusOK)
	}
}

func TestTrustedSubnet_RejectsOutside(t *testing.T) {
	gin.SetMode(gin.TestMode)

	_, subnet, err := net.ParseCIDR("10.0.0.0/24")
	if err != nil {
		t.Fatalf("parse cidr: %v", err)
	}

	router := gin.New()
	router.Use(TrustedSubnet(subnet))
	router.GET("/guard", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/guard", nil)
	req.Header.Set("X-Real-IP", "192.168.1.10")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusForbidden)
	}
}

func TestTrustedSubnet_AllowsInside(t *testing.T) {
	gin.SetMode(gin.TestMode)

	_, subnet, err := net.ParseCIDR("10.0.0.0/24")
	if err != nil {
		t.Fatalf("parse cidr: %v", err)
	}

	router := gin.New()
	router.Use(TrustedSubnet(subnet))
	router.GET("/guard", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/guard", nil)
	req.Header.Set("X-Real-IP", "10.0.0.10")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusOK)
	}
}

func TestTrustedSubnet_RejectsInvalidHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	_, subnet, err := net.ParseCIDR("10.0.0.0/24")
	if err != nil {
		t.Fatalf("parse cidr: %v", err)
	}

	router := gin.New()
	router.Use(TrustedSubnet(subnet))
	router.GET("/guard", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/guard", nil)
	req.Header.Set("X-Real-IP", "not-an-ip")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusForbidden)
	}
}

func TestTrustedSubnet_RejectsMissingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	_, subnet, err := net.ParseCIDR("10.0.0.0/24")
	if err != nil {
		t.Fatalf("parse cidr: %v", err)
	}

	router := gin.New()
	router.Use(TrustedSubnet(subnet))
	router.GET("/guard", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/guard", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusForbidden)
	}
}

func TestGzipRequest_BadGzip(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(GzipRequest())
	router.POST("/gzip", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/gzip", strings.NewReader("not-gzip"))
	req.Header.Set("Content-Encoding", "gzip")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGzipResponse_CompressesJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(GzipResponse())
	router.GET("/gzip", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json", []byte(`{"ok":true}`))
	})

	req := httptest.NewRequest(http.MethodGet, "/gzip", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusOK)
	}
	if enc := rec.Header().Get("Content-Encoding"); enc != "gzip" {
		t.Fatalf("encoding=%q want gzip", enc)
	}

	gr, err := gzip.NewReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer func() { _ = gr.Close() }()
	out, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("read gz body: %v", err)
	}
	if string(out) != `{"ok":true}` {
		t.Fatalf("body=%q want %q", out, `{"ok":true}`)
	}
}

func TestGzipResponse_SkipsWhenNoGzipAccept(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(GzipResponse())
	router.GET("/plain", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json", []byte(`{"ok":true}`))
	})

	req := httptest.NewRequest(http.MethodGet, "/plain", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusOK)
	}
	if enc := rec.Header().Get("Content-Encoding"); enc != "" {
		t.Fatalf("expected no Content-Encoding, got %q", enc)
	}
}

func TestGzipResponse_SkipsNonCompressibleType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(GzipResponse())
	router.GET("/bin", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/octet-stream", []byte{0x01, 0x02})
	})

	req := httptest.NewRequest(http.MethodGet, "/bin", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusOK)
	}
	if enc := rec.Header().Get("Content-Encoding"); enc != "" {
		t.Fatalf("expected no Content-Encoding, got %q", enc)
	}
}
