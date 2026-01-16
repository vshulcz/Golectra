// Package rsaenvelope provides RSA-OAEP + AES-GCM envelope encryption helpers.
package rsaenvelope

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
)

const (
	headerKey   = "X-Encrypted"
	headerValue = "rsa"

	aesKeySize    = 32
	nonceSize     = 12
	keyLenFieldSz = 2
)

type cipherEnvelope struct {
	pub  *rsa.PublicKey
	priv *rsa.PrivateKey
}

// NewEncrypter returns an encrypter for RSA-OAEP + AES-GCM envelopes.
func NewEncrypter(pub *rsa.PublicKey) *cipherEnvelope {
	if pub == nil {
		return nil
	}
	return &cipherEnvelope{pub: pub}
}

// NewDecrypter returns a decrypter for RSA-OAEP + AES-GCM envelopes.
func NewDecrypter(priv *rsa.PrivateKey) *cipherEnvelope {
	if priv == nil {
		return nil
	}
	return &cipherEnvelope{priv: priv}
}

func (c *cipherEnvelope) HeaderKey() string {
	return headerKey
}

func (c *cipherEnvelope) HeaderValue() string {
	return headerValue
}

func (c *cipherEnvelope) Encrypt(plain []byte) ([]byte, error) {
	if c == nil || c.pub == nil {
		return nil, errors.New("encrypt: nil public key")
	}

	key := make([]byte, aesKeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("rand key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("rand nonce: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, plain, nil)

	encKey, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, c.pub, key, nil)
	if err != nil {
		return nil, fmt.Errorf("encrypt key: %w", err)
	}
	keyLen, err := safeUint16(len(encKey))
	if err != nil {
		return nil, errors.New("encrypt key: envelope too large")
	}
	out := make([]byte, keyLenFieldSz+len(encKey)+len(nonce)+len(ciphertext))
	binary.BigEndian.PutUint16(out, keyLen)
	offset := keyLenFieldSz
	copy(out[offset:], encKey)
	offset += len(encKey)
	copy(out[offset:], nonce)
	offset += len(nonce)
	copy(out[offset:], ciphertext)
	return out, nil
}

func safeUint16(n int) (uint16, error) {
	if n < 0 || n > math.MaxUint16 {
		return 0, fmt.Errorf("out of uint16 range: %d", n)
	}
	return uint16(n), nil
}

func readKeyFile(path string) ([]byte, error) {
	if path == "" {
		return nil, errors.New("empty path")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(abs)
	name := filepath.Base(abs)
	fsys := os.DirFS(dir)
	if !fs.ValidPath(name) {
		return nil, fmt.Errorf("invalid key filename: %q", name)
	}
	return fs.ReadFile(fsys, name)
}

func (c *cipherEnvelope) Decrypt(envelope []byte) ([]byte, error) {
	if c == nil || c.priv == nil {
		return nil, errors.New("decrypt: nil private key")
	}
	if len(envelope) < keyLenFieldSz+nonceSize {
		return nil, errors.New("decrypt: envelope too short")
	}

	keyLen := int(binary.BigEndian.Uint16(envelope[:keyLenFieldSz]))
	offset := keyLenFieldSz
	if keyLen == 0 || len(envelope) < offset+keyLen+nonceSize {
		return nil, errors.New("decrypt: invalid key length")
	}
	encKey := envelope[offset : offset+keyLen]
	offset += keyLen
	nonce := envelope[offset : offset+nonceSize]
	offset += nonceSize
	ciphertext := envelope[offset:]
	if len(ciphertext) == 0 {
		return nil, errors.New("decrypt: empty ciphertext")
	}

	key, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, c.priv, encKey, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	if gcm.NonceSize() != nonceSize {
		return nil, errors.New("decrypt: unexpected nonce size")
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt payload: %w", err)
	}
	return plain, nil
}

// LoadPublicKey loads an RSA public key from a PEM file (PKIX or PKCS1).
func LoadPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := readKeyFile(path)
	if err != nil {
		return nil, fmt.Errorf("read public key: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("decode public key PEM: no block found")
	}

	ifc, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err == nil {
		key, ok := ifc.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("public key: not RSA")
		}
		return key, nil
	}
	key, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	return key, nil
}

// LoadPrivateKey loads an RSA private key from a PEM file (PKCS1 or PKCS8).
func LoadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := readKeyFile(path)
	if err != nil {
		return nil, fmt.Errorf("read private key: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("decode private key PEM: no block found")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err == nil {
		return key, nil
	}
	ifc, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	priv, ok := ifc.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key: not RSA")
	}
	return priv, nil
}
