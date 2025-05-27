package encryption

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

func TestValidateKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "valid 32-byte key",
			key:     "12345678901234567890123456789012",
			wantErr: false,
		},
		{
			name:    "empty key",
			key:     "",
			wantErr: true,
		},
		{
			name:    "too short key",
			key:     "short",
			wantErr: true,
		},
		{
			name:    "too long key",
			key:     "123456789012345678901234567890123", // 33 bytes
			wantErr: true,
		},
		{
			name:    "31-byte key",
			key:     "1234567890123456789012345678901",
			wantErr: true,
		},
		{
			name:    "33-byte key",
			key:     "123456789012345678901234567890123",
			wantErr: true,
		},
		{
			name:    "key with special characters (32 bytes)",
			key:     "!@#$%^&*()_+{}|:<>?[]\\;'\".,/1234",
			wantErr: false,
		},
		{
			name:    "key with unicode characters (too long)",
			key:     "αβγδεζηθικλμνξοπρστυφχψω12345678",
			wantErr: true, // Unicode characters are multi-byte in UTF-8, this is 56 bytes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateKey(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateKey() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && err != nil {
				if !strings.Contains(err.Error(), "32 bytes") {
					t.Errorf("ValidateKey() error message should mention 32 bytes, got: %v", err)
				}
			}
		})
	}
}

func TestEncrypt(t *testing.T) {
	validKey := "12345678901234567890123456789012" // 32 bytes

	tests := []struct {
		name    string
		data    []byte
		key     string
		wantErr bool
	}{
		{
			name:    "valid encryption with simple data",
			data:    []byte("hello world"),
			key:     validKey,
			wantErr: false,
		},
		{
			name:    "valid encryption with empty data",
			data:    []byte(""),
			key:     validKey,
			wantErr: false,
		},
		{
			name:    "valid encryption with binary data",
			data:    []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD},
			key:     validKey,
			wantErr: false,
		},
		{
			name:    "valid encryption with large data",
			data:    make([]byte, 10000), // 10KB of zeros
			key:     validKey,
			wantErr: false,
		},
		{
			name:    "invalid key - too short",
			data:    []byte("test data"),
			key:     "short",
			wantErr: true,
		},
		{
			name:    "invalid key - too long",
			data:    []byte("test data"),
			key:     "123456789012345678901234567890123", // 33 bytes
			wantErr: true,
		},
		{
			name:    "invalid key - empty",
			data:    []byte("test data"),
			key:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encrypted, err := Encrypt(tt.data, tt.key)

			if (err != nil) != tt.wantErr {
				t.Errorf("Encrypt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if err != nil && !strings.Contains(err.Error(), "32 bytes") {
					t.Errorf("Encrypt() error should mention key length requirement, got: %v", err)
				}
				return
			}

			// For successful encryption, verify the output
			if encrypted == "" {
				t.Errorf("Encrypt() returned empty string for valid input")
				return
			}

			// Verify it's valid base64
			decoded, err := base64.StdEncoding.DecodeString(encrypted)
			if err != nil {
				t.Errorf("Encrypt() output is not valid base64: %v", err)
				return
			}

			// Verify minimum length (12 bytes nonce + 16 bytes auth tag + data)
			expectedMinLength := 12 + 16 + len(tt.data)
			if len(decoded) < expectedMinLength {
				t.Errorf("Encrypt() output too short: got %d bytes, want at least %d", len(decoded), expectedMinLength)
			}

			// Verify that encrypting the same data twice produces different results (due to random nonce)
			encrypted2, err := Encrypt(tt.data, tt.key)
			if err != nil {
				t.Errorf("Encrypt() second call failed: %v", err)
				return
			}

			if encrypted == encrypted2 {
				t.Errorf("Encrypt() produced identical output for same input (nonce not random)")
			}
		})
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	// Note: This test requires a decrypt function which doesn't exist in the current code
	// But we can test that the encryption format is consistent
	validKey := "12345678901234567890123456789012"

	testData := [][]byte{
		[]byte("hello world"),
		[]byte(""),
		[]byte("The quick brown fox jumps over the lazy dog"),
		[]byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD},
		make([]byte, 1000), // 1KB of zeros
	}

	for i, data := range testData {
		t.Run(string(rune('A'+i)), func(t *testing.T) {
			encrypted, err := Encrypt(data, validKey)
			if err != nil {
				t.Fatalf("Encrypt() failed: %v", err)
			}

			// Verify the encrypted data is different from original (unless empty)
			if len(data) > 0 && encrypted == string(data) {
				t.Errorf("Encrypt() output identical to input")
			}

			// Verify it's valid base64
			decoded, err := base64.StdEncoding.DecodeString(encrypted)
			if err != nil {
				t.Errorf("Encrypt() output is not valid base64: %v", err)
			}

			// Verify structure: nonce (12 bytes) + ciphertext + auth tag (16 bytes)
			if len(decoded) < 28 { // 12 + 16 minimum
				t.Errorf("Encrypted data too short: %d bytes", len(decoded))
			}
		})
	}
}

func TestEncryptWithDifferentKeys(t *testing.T) {
	data := []byte("test data for encryption")
	key1 := "12345678901234567890123456789012"
	key2 := "abcdefghijklmnopqrstuvwxyz123456"

	encrypted1, err := Encrypt(data, key1)
	if err != nil {
		t.Fatalf("Encrypt() with key1 failed: %v", err)
	}

	encrypted2, err := Encrypt(data, key2)
	if err != nil {
		t.Fatalf("Encrypt() with key2 failed: %v", err)
	}

	// Different keys should produce different encrypted output
	if encrypted1 == encrypted2 {
		t.Errorf("Encrypt() with different keys produced identical output")
	}
}

func TestEncryptErrorHandling(t *testing.T) {
	// Test with various invalid inputs to ensure proper error handling
	tests := []struct {
		name        string
		data        []byte
		key         string
		expectError string
	}{
		{
			name:        "nil data with invalid key",
			data:        nil,
			key:         "short",
			expectError: "32 bytes",
		},
		{
			name:        "large data with invalid key",
			data:        make([]byte, 1000000), // 1MB
			key:         "invalid",
			expectError: "32 bytes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Encrypt(tt.data, tt.key)
			if err == nil {
				t.Errorf("Encrypt() expected error but got none")
				return
			}

			if !strings.Contains(err.Error(), tt.expectError) {
				t.Errorf("Encrypt() error = %v, want error containing %v", err, tt.expectError)
			}
		})
	}
}

// BenchmarkEncrypt benchmarks the encryption function
func BenchmarkEncrypt(b *testing.B) {
	key := "12345678901234567890123456789012"
	data := []byte("This is some test data for benchmarking encryption performance")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Encrypt(data, key)
		if err != nil {
			b.Fatalf("Encrypt() failed: %v", err)
		}
	}
}

// BenchmarkEncryptLargeData benchmarks encryption with larger data
func BenchmarkEncryptLargeData(b *testing.B) {
	key := "12345678901234567890123456789012"
	data := make([]byte, 10240) // 10KB
	rand.Read(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Encrypt(data, key)
		if err != nil {
			b.Fatalf("Encrypt() failed: %v", err)
		}
	}
}

// BenchmarkValidateKey benchmarks the key validation function
func BenchmarkValidateKey(b *testing.B) {
	key := "12345678901234567890123456789012"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateKey(key)
	}
}

// TestEncryptWithMockedFailures tests error conditions that are hard to trigger naturally
func TestEncryptWithMockedFailures(t *testing.T) {
	// Test with a key that would cause cipher creation to fail
	// This is difficult to test naturally since aes.NewCipher rarely fails with valid key lengths
	// But we can test the error path by using reflection or other techniques

	// For now, we'll test the documented behavior with invalid key lengths
	// which we already cover in other tests

	// Test with nil data to ensure it doesn't panic
	validKey := "12345678901234567890123456789012"
	_, err := Encrypt(nil, validKey)
	if err != nil {
		t.Errorf("Encrypt() with nil data should not fail, got: %v", err)
	}
}

// TestEncryptConsistency tests that encryption is consistent in format
func TestEncryptConsistency(t *testing.T) {
	validKey := "12345678901234567890123456789012"
	testData := []byte("consistency test data")

	// Encrypt multiple times
	results := make([]string, 10)
	for i := 0; i < 10; i++ {
		encrypted, err := Encrypt(testData, validKey)
		if err != nil {
			t.Fatalf("Encrypt() failed on iteration %d: %v", i, err)
		}
		results[i] = encrypted
	}

	// Verify all results are different (due to random nonce)
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i] == results[j] {
				t.Errorf("Encrypt() produced identical results at indices %d and %d", i, j)
			}
		}
	}

	// Verify all results decode to the same structure
	for i, encrypted := range results {
		decoded, err := base64.StdEncoding.DecodeString(encrypted)
		if err != nil {
			t.Errorf("Result %d is not valid base64: %v", i, err)
			continue
		}

		// Check minimum length (12 bytes nonce + 16 bytes auth tag + data length)
		expectedMinLength := 12 + 16 + len(testData)
		if len(decoded) < expectedMinLength {
			t.Errorf("Result %d too short: got %d bytes, want at least %d", i, len(decoded), expectedMinLength)
		}
	}
}

// TestValidateKeyEdgeCases tests edge cases for key validation
func TestValidateKeyEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		wantErr     bool
		description string
	}{
		{
			name:        "exactly 32 bytes with null characters",
			key:         "123456789012345678901234567890\x00\x00",
			wantErr:     false,
			description: "null characters should be valid",
		},
		{
			name:        "32 bytes with high unicode",
			key:         "1234567890123456789012345678901\xFF",
			wantErr:     false,
			description: "high byte values should be valid",
		},
		{
			name:        "31 bytes exactly",
			key:         "1234567890123456789012345678901",
			wantErr:     true,
			description: "one byte short should fail",
		},
		{
			name:        "33 bytes exactly",
			key:         "123456789012345678901234567890123",
			wantErr:     true,
			description: "one byte over should fail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateKey(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateKey() error = %v, wantErr %v (%s)", err, tt.wantErr, tt.description)
			}
		})
	}
}

// TestEncryptLargeDataSizes tests encryption with various data sizes
func TestEncryptLargeDataSizes(t *testing.T) {
	validKey := "12345678901234567890123456789012"

	// Test various data sizes
	sizes := []int{0, 1, 16, 1024, 10240, 65536} // 0B, 1B, 16B, 1KB, 10KB, 64KB

	for _, size := range sizes {
		t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
			data := make([]byte, size)
			// Fill with some pattern
			for i := range data {
				data[i] = byte(i % 256)
			}

			encrypted, err := Encrypt(data, validKey)
			if err != nil {
				t.Errorf("Encrypt() failed for size %d: %v", size, err)
				return
			}

			// Verify the encrypted data is valid base64
			decoded, err := base64.StdEncoding.DecodeString(encrypted)
			if err != nil {
				t.Errorf("Encrypted data for size %d is not valid base64: %v", size, err)
				return
			}

			// Verify minimum expected size
			expectedMinSize := 12 + 16 + size // nonce + auth tag + data
			if len(decoded) < expectedMinSize {
				t.Errorf("Encrypted data for size %d too small: got %d, want at least %d", size, len(decoded), expectedMinSize)
			}
		})
	}
}
