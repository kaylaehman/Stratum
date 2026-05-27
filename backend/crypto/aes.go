// Package crypto provides the authenticated-encryption primitive used by the
// secrets vault and encrypted node credentials. It exposes a single opaque
// blob API (Seal/Open) so callers can never mishandle the nonce.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

// KeySize is the required AES-256 key length in bytes.
const KeySize = 32

// nonceSize is the standard GCM nonce length in bytes. It is prepended to
// every sealed blob so Open can recover it without the caller tracking it.
const nonceSize = 12

// ErrInvalidKey is returned when the key is not exactly KeySize bytes.
var ErrInvalidKey = errors.New("crypto: key must be 32 bytes (AES-256)")

// ErrMalformedBlob is returned when a blob is too short to contain a nonce.
var ErrMalformedBlob = errors.New("crypto: blob too short")

// Cipher seals and opens values with AES-256-GCM under a fixed key.
type Cipher struct {
	aead cipher.AEAD
}

// New builds a Cipher from a 32-byte key. The key typically comes from the
// ENCRYPTION_KEY environment variable (32 bytes, hex-decoded by the caller).
func New(key []byte) (*Cipher, error) {
	if len(key) != KeySize {
		return nil, ErrInvalidKey
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: new cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: new gcm: %w", err)
	}
	return &Cipher{aead: aead}, nil
}

// Seal generates a fresh random nonce, encrypts plaintext, and returns
// nonce || ciphertext as a single blob. The nonce is never exposed separately.
func (c *Cipher) Seal(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("crypto: read nonce: %w", err)
	}
	// Seal appends ciphertext to nonce, yielding nonce||ciphertext in one slice.
	return c.aead.Seal(nonce, nonce, plaintext, nil), nil
}

// Open splits the leading nonce off the blob and decrypts the remainder.
// A tampered blob or wrong key fails authentication and returns an error —
// never silent corruption.
func (c *Cipher) Open(blob []byte) ([]byte, error) {
	if len(blob) < nonceSize {
		return nil, ErrMalformedBlob
	}
	nonce, ciphertext := blob[:nonceSize], blob[nonceSize:]
	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("crypto: open: %w", err)
	}
	return plaintext, nil
}
