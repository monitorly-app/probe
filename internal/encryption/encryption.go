package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// Encrypt encrypts data using AES-256-GCM with the provided key
// The key must be exactly 32 bytes long (for AES-256)
// Returns base64-encoded encrypted data
func Encrypt(data []byte, key string) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("encryption key must be exactly 32 bytes long")
	}

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Never use more than 2^32 random nonces with a given key
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Encrypt and append nonce
	ciphertext := aesgcm.Seal(nil, nonce, data, nil)
	encrypted := append(nonce, ciphertext...)

	// Encode as base64 for safe transmission
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// ValidateKey checks if the encryption key is valid (32 bytes)
func ValidateKey(key string) error {
	if len(key) != 32 {
		return fmt.Errorf("encryption key must be exactly 32 bytes long")
	}
	return nil
}
