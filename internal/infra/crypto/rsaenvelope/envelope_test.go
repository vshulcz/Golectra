package rsaenvelope

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	encrypter := NewEncrypter(&priv.PublicKey)
	decrypter := NewDecrypter(priv)

	plain := []byte("hello encrypted world")
	enc, err := encrypter.Encrypt(plain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	dec, err := decrypter.Decrypt(enc)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(dec, plain) {
		t.Fatalf("decrypt mismatch: got %q want %q", dec, plain)
	}

	if encrypter.HeaderKey() != headerKey || encrypter.HeaderValue() != headerValue {
		t.Fatalf("header mismatch: %s=%s", encrypter.HeaderKey(), encrypter.HeaderValue())
	}
}

func TestLoadKeys(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal pub: %v", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	dir := t.TempDir()
	pubPath := filepath.Join(dir, "pub.pem")
	privPath := filepath.Join(dir, "priv.pem")

	if err := os.WriteFile(pubPath, pubPEM, 0o600); err != nil {
		t.Fatalf("write pub: %v", err)
	}
	if err := os.WriteFile(privPath, privPEM, 0o600); err != nil {
		t.Fatalf("write priv: %v", err)
	}

	gotPub, err := LoadPublicKey(pubPath)
	if err != nil {
		t.Fatalf("load pub: %v", err)
	}
	gotPriv, err := LoadPrivateKey(privPath)
	if err != nil {
		t.Fatalf("load priv: %v", err)
	}

	if gotPub.N.Cmp(priv.N) != 0 {
		t.Fatal("public key mismatch")
	}
	if gotPriv.N.Cmp(priv.N) != 0 {
		t.Fatal("private key mismatch")
	}
}

func TestReadKeyFile_Errors(t *testing.T) {
	if _, err := readKeyFile(""); err == nil {
		t.Fatal("expected error for empty path")
	}
	if _, err := readKeyFile("../"); err == nil {
		t.Fatal("expected error for invalid filename")
	}
	if _, err := readKeyFile(filepath.Join(t.TempDir(), "missing.pem")); err == nil {
		t.Fatal("expected error for missing file")
	}
}
