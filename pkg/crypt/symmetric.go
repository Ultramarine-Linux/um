package crypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
)

// These functions allow for symmetric encryption and decryption using AES-GCM.
// They are provided as a convenience for encrypting and decrypting data, in a sane and secure way.

func NewKey() ([]byte, error) {
	key := make([]byte, 32) // 32 for AES-256

	_, err := rand.Read(key)
	if err != nil {
		return nil, err
	}

	return key, nil
}

func NewNonce() ([]byte, error) {
	nonce := make([]byte, 12)

	_, err := rand.Read(nonce)
	if err != nil {
		return nil, err
	}

	return nonce, nil
}

// SECURITY: NEVER reuse nonces!
func Encrypt(key []byte, nonce []byte, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	ciphertext := aesgcm.Seal(nil, nonce, data, nil)

	return ciphertext, nil
}

func Decrypt(key []byte, nonce []byte, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plaintext, err := aesgcm.Open(nil, nonce, data, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}
