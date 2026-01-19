package ports

// PayloadEncrypter encrypts outbound payloads and advertises its header metadata.
type PayloadEncrypter interface {
	Encrypt([]byte) ([]byte, error)
	HeaderKey() string
	HeaderValue() string
}

// PayloadDecrypter decrypts inbound payloads and advertises its header metadata.
type PayloadDecrypter interface {
	Decrypt([]byte) ([]byte, error)
	HeaderKey() string
	HeaderValue() string
}
