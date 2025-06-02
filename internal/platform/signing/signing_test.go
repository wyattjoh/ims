package signing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/wyattjoh/ims/internal/sig"
)

func TestGetValue(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		includePath bool
		expected    string
	}{
		{
			name:        "simple query parameters without path",
			url:         "/test?width=100&height=200&sig=abc123",
			includePath: false,
			expected:    "height=200&width=100",
		},
		{
			name:        "simple query parameters with path",
			url:         "/test?width=100&height=200&sig=abc123",
			includePath: true,
			expected:    "/test?height=200&width=100",
		},
		{
			name:        "no query parameters without path",
			url:         "/test?sig=abc123",
			includePath: false,
			expected:    "",
		},
		{
			name:        "no query parameters with path",
			url:         "/test?sig=abc123",
			includePath: true,
			expected:    "/test?",
		},
		{
			name:        "multiple parameters without sig",
			url:         "/image?format=jpeg&quality=80&resize=300x200",
			includePath: false,
			expected:    "format=jpeg&quality=80&resize=300x200",
		},
		{
			name:        "multiple parameters with sig that gets removed",
			url:         "/image?format=jpeg&sig=test&quality=80&resize=300x200",
			includePath: false,
			expected:    "format=jpeg&quality=80&resize=300x200",
		},
		{
			name:        "url encoded parameters",
			url:         "/image?url=https%3A%2F%2Fexample.com%2Fimage.jpg&sig=test",
			includePath: false,
			expected:    "url=https%3A%2F%2Fexample.com%2Fimage.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			result := getValue(req, tt.includePath)
			if result != tt.expected {
				t.Errorf("getValue() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestMiddleware(t *testing.T) {
	secret := "test-secret"

	tests := []struct {
		name           string
		url            string
		secret         string
		includePath    bool
		expectStatus   int
		expectResponse string
	}{
		{
			name:           "missing signature",
			url:            "/test?width=100&height=200",
			secret:         secret,
			includePath:    false,
			expectStatus:   http.StatusUnauthorized,
			expectResponse: "Signature invalid\n",
		},
		{
			name:           "empty signature",
			url:            "/test?width=100&height=200&sig=",
			secret:         secret,
			includePath:    false,
			expectStatus:   http.StatusUnauthorized,
			expectResponse: "Signature invalid\n",
		},
		{
			name:           "invalid signature",
			url:            "/test?width=100&height=200&sig=invalid",
			secret:         secret,
			includePath:    false,
			expectStatus:   http.StatusUnauthorized,
			expectResponse: "Signature invalid\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock next handler
			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success"))
			})

			// Create the middleware
			handler := Middleware(tt.secret, tt.includePath, next)

			// Create request and response recorder
			req := httptest.NewRequest("GET", tt.url, nil)
			rr := httptest.NewRecorder()

			// Execute the handler
			handler(rr, req)

			// Check status code
			if rr.Code != tt.expectStatus {
				t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, tt.expectStatus)
			}

			// Check response body
			if rr.Body.String() != tt.expectResponse {
				t.Errorf("handler returned unexpected body: got %q want %q", rr.Body.String(), tt.expectResponse)
			}

			// Check if next was called (should only be called for valid signatures)
			if tt.expectStatus == http.StatusOK && !nextCalled {
				t.Error("next handler was not called for valid request")
			}
			if tt.expectStatus != http.StatusOK && nextCalled {
				t.Error("next handler was called for invalid request")
			}
		})
	}
}

func TestMiddlewareWithValidSignature(t *testing.T) {
	secret := "test-secret"

	// Test with a scenario where we can control the signature validation
	// by creating a request with parameters that we know will pass
	tests := []struct {
		name        string
		includePath bool
		params      url.Values
	}{
		{
			name:        "valid signature without path",
			includePath: false,
			params: url.Values{
				"width":  []string{"100"},
				"height": []string{"200"},
			},
		},
		{
			name:        "valid signature with path",
			includePath: true,
			params: url.Values{
				"format":  []string{"jpeg"},
				"quality": []string{"80"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create the value that would be signed
			value := tt.params.Encode()
			if tt.includePath {
				value = "/test?" + value
			}

			// Generate a valid signature using the same logic as sig.Verify expects
			token := hmac.New(sha256.New, []byte(secret))
			token.Write([]byte(value))
			validSig := strings.ToLower(hex.EncodeToString(token.Sum(nil)))

			// Add the signature to params
			tt.params.Set("sig", validSig)

			// Build the URL
			testURL := "/test?" + tt.params.Encode()

			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success"))
			})

			handler := Middleware(secret, tt.includePath, next)
			req := httptest.NewRequest("GET", testURL, nil)
			rr := httptest.NewRecorder()

			handler(rr, req)

			// Verify the signature was processed correctly
			if !sig.Verify(validSig, value, secret) {
				t.Errorf("Generated signature should be valid according to sig.Verify")
			}

			// Check if the signature was valid
			if rr.Code != http.StatusOK {
				t.Errorf("Expected valid signature to return 200, got %d", rr.Code)
			}
			if !nextCalled {
				t.Error("Expected next handler to be called for valid signature")
			}
		})
	}
}
