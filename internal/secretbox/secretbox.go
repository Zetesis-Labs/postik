// Package secretbox seals the tokens of the networks with the deployment key
// (constitution §8).
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
)

// Box seals with AES-256-GCM. The associated data binds a sealed value to
// where it is stored, so it cannot be moved to another row or column.
type Box struct {
	aead cipher.AEAD
}

func New(key []byte) (*Box, error) {
	if len(key) != 32 {
		return nil, errors.New("the key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

// Seal returns the nonce followed by the ciphertext.
func (b *Box) Seal(plaintext, associated []byte) ([]byte, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return b.aead.Seal(nonce, nonce, plaintext, associated), nil
}

func (b *Box) Open(sealed, associated []byte) ([]byte, error) {
	size := b.aead.NonceSize()
	if len(sealed) < size {
		return nil, errors.New("sealed value too short")
	}
	return b.aead.Open(nil, sealed[:size], sealed[size:], associated)
}
