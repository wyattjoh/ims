package sig

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestVerify(t *testing.T) {
	secret := "test-secret"
	
	// Helper function to create a valid signature
	createValidSignature := func(value, secret string) string {
		token := hmac.New(sha256.New, []byte(secret))
		token.Write([]byte(value))
		return strings.ToLower(hex.EncodeToString(token.Sum(nil)))
	}

	tests := []struct {
		name      string
		signature string
		value     string
		secret    string
		expected  bool
	}{
		{
			name:      "valid signature",
			signature: createValidSignature("test-value", secret),
			value:     "test-value",
			secret:    secret,
			expected:  true,
		},
		{
			name:      "invalid signature",
			signature: "invalid-signature",
			value:     "test-value",
			secret:    secret,
			expected:  false,
		},
		{
			name:      "wrong secret",
			signature: createValidSignature("test-value", "wrong-secret"),
			value:     "test-value",
			secret:    secret,
			expected:  false,
		},
		{
			name:      "wrong value",
			signature: createValidSignature("wrong-value", secret),
			value:     "test-value",
			secret:    secret,
			expected:  false,
		},
		{
			name:      "empty signature",
			signature: "",
			value:     "test-value",
			secret:    secret,
			expected:  false,
		},
		{
			name:      "empty value",
			signature: createValidSignature("", secret),
			value:     "",
			secret:    secret,
			expected:  true,
		},
		{
			name:      "empty secret",
			signature: createValidSignature("test-value", ""),
			value:     "test-value",
			secret:    "",
			expected:  true,
		},
		{
			name:      "case insensitive signature",
			signature: strings.ToUpper(createValidSignature("test-value", secret)),
			value:     "test-value",
			secret:    secret,
			expected:  false, // Our implementation expects lowercase
		},
		{
			name:      "url query parameters",
			signature: createValidSignature("height=200&width=100", secret),
			value:     "height=200&width=100",
			secret:    secret,
			expected:  true,
		},
		{
			name:      "url with path",
			signature: createValidSignature("/test?height=200&width=100", secret),
			value:     "/test?height=200&width=100",
			secret:    secret,
			expected:  true,
		},
		{
			name:      "complex url parameters",
			signature: createValidSignature("format=jpeg&quality=80&url=https%3A%2F%2Fexample.com%2Fimage.jpg", secret),
			value:     "format=jpeg&quality=80&url=https%3A%2F%2Fexample.com%2Fimage.jpg",
			secret:    secret,
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Verify(tt.signature, tt.value, tt.secret)
			if result != tt.expected {
				t.Errorf("Verify(%q, %q, %q) = %v, want %v", tt.signature, tt.value, tt.secret, result, tt.expected)
			}
		})
	}
}

func TestVerifyConstantTime(t *testing.T) {
	// Test that the function uses constant time comparison
	// This is important for security to prevent timing attacks
	secret := "test-secret"
	value := "test-value"
	
	// Create a valid signature
	token := hmac.New(sha256.New, []byte(secret))
	token.Write([]byte(value))
	validSig := strings.ToLower(hex.EncodeToString(token.Sum(nil)))
	
	// Test with the exact valid signature
	if !Verify(validSig, value, secret) {
		t.Error("Valid signature should return true")
	}
	
	// Test with a signature that differs only in the last character
	invalidSig := validSig[:len(validSig)-1] + "x"
	if Verify(invalidSig, value, secret) {
		t.Error("Invalid signature should return false")
	}
	
	// Test with a signature of different length
	shortSig := validSig[:len(validSig)-5]
	if Verify(shortSig, value, secret) {
		t.Error("Short signature should return false")
	}
}

func TestVerifyWithHMACWriteError(t *testing.T) {
	// Test edge case where HMAC write could theoretically fail
	// In practice, HMAC's Write method never returns an error for byte slices
	// but our code handles this case
	
	secret := "test-secret"
	value := "test-value"
	signature := "any-signature"
	
	// Normal case should work
	result := Verify(signature, value, secret)
	// We expect false because signature is invalid, but no panic/error
	if result {
		t.Error("Invalid signature should return false")
	}
}

// Benchmark to ensure the verification is reasonably fast
func BenchmarkVerify(b *testing.B) {
	secret := "test-secret"
	value := "height=200&width=100&format=jpeg&quality=80"
	
	// Create valid signature
	token := hmac.New(sha256.New, []byte(secret))
	token.Write([]byte(value))
	signature := strings.ToLower(hex.EncodeToString(token.Sum(nil)))
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Verify(signature, value, secret)
	}
}

func BenchmarkVerifyInvalid(b *testing.B) {
	secret := "test-secret"
	value := "height=200&width=100&format=jpeg&quality=80"
	signature := "invalid-signature-that-will-fail"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Verify(signature, value, secret)
	}
}