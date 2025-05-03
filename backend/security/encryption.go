package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"log"
)

var encryptionKey []byte

// InitializeEncryption sets up the encryption key from environment variable
func InitializeEncryption(key string) {
	// Pad the key to 32 bytes if needed
	if len(key) < 32 {
		padding := make([]byte, 32-len(key))
		key = key + string(padding)
	}
	encryptionKey = []byte(key[:32])
}

// Encrypt encrypts a string using AES-GCM
func Encrypt(plaintext string) (string, error) {
	if len(encryptionKey) == 0 {
		return "", errors.New("encryption key not initialized")
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a string using AES-GCM
func Decrypt(encrypted string) (string, error) {
	if len(encryptionKey) == 0 {
		return "", errors.New("encryption key not initialized")
	}

	log.Printf("Attempting to decrypt value (length: %d)", len(encrypted))

	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		log.Printf("Failed to decode base64: %v", err)
		return "", err
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		log.Printf("Failed to create cipher: %v", err)
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		log.Printf("Failed to create GCM: %v", err)
		return "", err
	}

	if len(ciphertext) < gcm.NonceSize() {
		log.Printf("Ciphertext too short: %d < %d", len(ciphertext), gcm.NonceSize())
		return "", errors.New("ciphertext too short")
	}

	nonce := ciphertext[:gcm.NonceSize()]
	ciphertext = ciphertext[gcm.NonceSize():]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		log.Printf("Failed to decrypt: %v", err)
		return "", err
	}

	result := string(plaintext)
	log.Printf("Successfully decrypted to: %s", result)
	return result, nil
}
