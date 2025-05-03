package security

import (
	"testing"
)

// TestMain sets up the encryption key for all tests and cleans up after
func TestMain(m *testing.M) {
	// Initialize with a test key
	testKey := "test-encryption-key-12345678901234"
	InitializeEncryption(testKey)

	// Run tests
	m.Run()

	// Clean up by resetting the encryption key
	encryptionKey = nil
}

func TestEncryptionKeyInitialization(t *testing.T) {
	// Test with a short key (should be padded)
	shortKey := "short-key"
	InitializeEncryption(shortKey)

	// Key should be padded to 32 bytes
	if len(encryptionKey) != 32 {
		t.Errorf("Expected padded key length of 32, got %d", len(encryptionKey))
	}

	// Test with exactly 32 bytes
	exactKey := "12345678901234567890123456789012" // 32 bytes
	InitializeEncryption(exactKey)

	// Key should remain 32 bytes
	if len(encryptionKey) != 32 {
		t.Errorf("Expected key length of 32, got %d", len(encryptionKey))
	}

	// Test with a longer key (should be truncated)
	longKey := "this-is-a-very-long-key-that-exceeds-32-bytes-by-quite-a-lot"
	InitializeEncryption(longKey)

	// Key should be truncated to 32 bytes
	if len(encryptionKey) != 32 {
		t.Errorf("Expected truncated key length of 32, got %d", len(encryptionKey))
	}

	// Re-initialize with the test key for remaining tests
	testKey := "test-encryption-key-12345678901234"
	InitializeEncryption(testKey)
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	// Test values to encrypt and decrypt
	testCases := []struct {
		name  string
		value string
	}{
		{"Simple text", "Hello, world!"},
		{"Empty string", ""},
		{"Special characters", "!@#$%^&*()_+{}|:<>?~"},
		{"Long text", "This is a longer text to encrypt and decrypt to ensure that our encryption works correctly with various lengths of input data."},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Encrypt the value
			encrypted, err := Encrypt(tc.value)
			if err != nil {
				t.Fatalf("Error encrypting '%s': %v", tc.value, err)
			}

			// Make sure the encrypted value is different from the original
			if encrypted == tc.value && tc.value != "" {
				t.Errorf("Encrypted value '%s' is the same as the original", encrypted)
			}

			// Decrypt the value
			decrypted, err := Decrypt(encrypted)
			if err != nil {
				t.Fatalf("Error decrypting '%s': %v", encrypted, err)
			}

			// Make sure the decrypted value matches the original
			if decrypted != tc.value {
				t.Errorf("Expected decrypted value '%s', got '%s'", tc.value, decrypted)
			}
		})
	}
}

func TestEncryptWithUninitializedKey(t *testing.T) {
	// Temporarily set the encryption key to nil
	originalKey := encryptionKey
	encryptionKey = nil
	defer func() { encryptionKey = originalKey }()

	// Try to encrypt a value
	_, err := Encrypt("test")
	if err == nil {
		t.Error("Expected error when encrypting with uninitialized key, got nil")
	}

	// Check error message
	expectedError := "encryption key not initialized"
	if err.Error() != expectedError {
		t.Errorf("Expected error message '%s', got '%s'", expectedError, err.Error())
	}
}

func TestDecryptWithUninitializedKey(t *testing.T) {
	// Temporarily set the encryption key to nil
	originalKey := encryptionKey
	encryptionKey = nil
	defer func() { encryptionKey = originalKey }()

	// Try to decrypt a value
	_, err := Decrypt("test")
	if err == nil {
		t.Error("Expected error when decrypting with uninitialized key, got nil")
	}

	// Check error message
	expectedError := "encryption key not initialized"
	if err.Error() != expectedError {
		t.Errorf("Expected error message '%s', got '%s'", expectedError, err.Error())
	}
}

func TestDecryptInvalidData(t *testing.T) {
	// Test decrypting invalid base64 data
	_, err := Decrypt("not-base64")
	if err == nil {
		t.Error("Expected error when decrypting invalid base64 data, got nil")
	}

	// Test decrypting valid base64 but invalid ciphertext
	_, err = Decrypt("aGVsbG8=") // "hello" in base64
	if err == nil {
		t.Error("Expected error when decrypting invalid ciphertext, got nil")
	}
}
