package middlewares

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/vshulcz/Golectra/internal/infra/crypto/rsaenvelope"
)

func TestDecryptPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	encrypter := rsaenvelope.NewEncrypter(&priv.PublicKey)
	decrypter := rsaenvelope.NewDecrypter(priv)

	r := gin.New()
	r.Use(DecryptPayload(decrypter))
	r.POST("/decrypt", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		c.Data(http.StatusOK, "text/plain", body)
	})

	plain := []byte("payload")
	enc, err := encrypter.Encrypt(plain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/decrypt", bytes.NewReader(enc))
	req.Header.Set(encrypter.HeaderKey(), encrypter.HeaderValue())
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusOK)
	}
	if !bytes.Equal(rec.Body.Bytes(), plain) {
		t.Fatalf("body=%q want %q", rec.Body.Bytes(), plain)
	}
}

func TestDecryptPayload_NoHeaderPassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	decrypter := rsaenvelope.NewDecrypter(priv)

	r := gin.New()
	r.Use(DecryptPayload(decrypter))
	r.POST("/plain", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		c.Data(http.StatusOK, "text/plain", body)
	})

	req := httptest.NewRequest(http.MethodPost, "/plain", bytes.NewReader([]byte("plain")))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != "plain" {
		t.Fatalf("body=%q want %q", got, "plain")
	}
}

func TestDecryptPayload_InvalidCiphertext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	decrypter := rsaenvelope.NewDecrypter(priv)

	r := gin.New()
	r.Use(DecryptPayload(decrypter))
	r.POST("/decrypt", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/decrypt", bytes.NewReader([]byte("bad")))
	req.Header.Set(decrypter.HeaderKey(), decrypter.HeaderValue())
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want %d", rec.Code, http.StatusBadRequest)
	}
}
