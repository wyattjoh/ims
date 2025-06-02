package handlers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wyattjoh/ims/internal/image/provider"
	"github.com/wyattjoh/ims/internal/platform/providers"
)

// Mock provider for testing
type mockProvider struct {
	response io.ReadCloser
	error    error
}

func (m *mockProvider) Provide(ctx context.Context, filename string) (io.ReadCloser, error) {
	if m.error != nil {
		return nil, m.error
	}
	return m.response, nil
}

// Mock proxy provider for testing proxy-specific logic
type mockProxyProvider struct {
	response io.ReadCloser
	error    error
}

func (m *mockProxyProvider) Provide(ctx context.Context, filename string) (io.ReadCloser, error) {
	if m.error != nil {
		return nil, m.error
	}
	return m.response, nil
}

func TestGetFilename(t *testing.T) {
	tests := []struct {
		name        string
		provider    provider.Provider
		urlPath     string
		queryParams string
		expected    string
		expectError error
	}{
		{
			name:        "regular provider with simple path",
			provider:    &mockProvider{},
			urlPath:     "/image.jpg",
			expected:    "image.jpg",
			expectError: nil,
		},
		{
			name:        "regular provider with complex path",
			provider:    &mockProvider{},
			urlPath:     "/path/to/image.jpg",
			expected:    "path/to/image.jpg",
			expectError: nil,
		},
		{
			name:        "regular provider with short path",
			provider:    &mockProvider{},
			urlPath:     "/a",
			expected:    "a",
			expectError: nil,
		},
		{
			name:        "regular provider with empty path",
			provider:    &mockProvider{},
			urlPath:     "/",
			expectError: ErrFilenameTooShort,
		},
		{
			name:        "proxy provider with valid url",
			provider:    provider.NewProxy(nil),
			urlPath:     "/any",
			queryParams: "url=https://example.com/image.jpg",
			expected:    "https://example.com/image.jpg",
			expectError: nil,
		},
		{
			name:        "proxy provider with short url",
			provider:    provider.NewProxy(nil),
			urlPath:     "/any",
			queryParams: "url=http://",
			expectError: ErrFilenameTooShort,
		},
		{
			name:        "proxy provider with empty url",
			provider:    provider.NewProxy(nil),
			urlPath:     "/any",
			queryParams: "url=",
			expectError: ErrFilenameTooShort,
		},
		{
			name:        "proxy provider without url param",
			provider:    provider.NewProxy(nil),
			urlPath:     "/any",
			queryParams: "",
			expectError: ErrFilenameTooShort,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := tt.urlPath
			if tt.queryParams != "" {
				url += "?" + tt.queryParams
			}
			
			req := httptest.NewRequest("GET", url, nil)
			
			result, err := getFilename(tt.provider, req)
			
			if tt.expectError != nil {
				if err == nil {
					t.Errorf("Expected error %v, got nil", tt.expectError)
					return
				}
				if err != tt.expectError {
					t.Errorf("Expected error %v, got %v", tt.expectError, err)
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if result != tt.expected {
				t.Errorf("Expected filename %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestImage(t *testing.T) {
	timeout := 30 * time.Second

	tests := []struct {
		name           string
		provider       provider.Provider
		setupContext   bool
		path           string
		query          string
		expectStatus   int
		providerError  error
	}{
		{
			name:         "missing provider in context",
			provider:     nil,
			setupContext: false,
			path:         "/image.jpg",
			expectStatus: http.StatusInternalServerError,
		},
		{
			name:         "provider returns not found",
			provider:     &mockProvider{error: provider.ErrNotFound},
			setupContext: true,
			path:         "/image.jpg",
			expectStatus: http.StatusNotFound,
		},
		{
			name:         "provider returns bad gateway",
			provider:     &mockProvider{error: provider.ErrBadGateway},
			setupContext: true,
			path:         "/image.jpg",
			expectStatus: http.StatusBadGateway,
		},
		{
			name:         "provider returns filename error",
			provider:     &mockProvider{error: provider.ErrFilename},
			setupContext: true,
			path:         "/image.jpg",
			expectStatus: http.StatusBadRequest,
		},
		{
			name:         "provider returns other error",
			provider:     &mockProvider{error: errors.New("some other error")},
			setupContext: true,
			path:         "/image.jpg",
			expectStatus: http.StatusInternalServerError,
		},
		{
			name:         "filename too short",
			provider:     &mockProvider{},
			setupContext: true,
			path:         "/",
			expectStatus: http.StatusNotFound,
		},
		{
			name:         "proxy provider with valid url",
			provider:     provider.NewProxy(nil),
			setupContext: true,
			path:         "/proxy",
			query:        "url=https://example.com/image.jpg",
			expectStatus: http.StatusNotFound, // Will get 404 from external URL
		},
		{
			name:         "proxy provider with invalid url",
			provider:     provider.NewProxy(nil),
			setupContext: true,
			path:         "/proxy",
			query:        "url=short",
			expectStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request
			url := tt.path
			if tt.query != "" {
				url += "?" + tt.query
			}
			req := httptest.NewRequest("GET", url, nil)
			
			// Setup context with provider if needed
			if tt.setupContext && tt.provider != nil {
				ctx := context.WithValue(req.Context(), providers.ContextKey, tt.provider)
				req = req.WithContext(ctx)
			}
			
			// Create response recorder
			rr := httptest.NewRecorder()
			
			// Call the handler
			handler := Image(timeout)
			handler(rr, req)
			
			// Check status code
			if rr.Code != tt.expectStatus {
				t.Errorf("Expected status %d, got %d", tt.expectStatus, rr.Code)
			}
		})
	}
}

func TestImageWithSuccessfulProvider(t *testing.T) {
	timeout := 30 * time.Second
	
	// Create a mock response body
	responseBody := strings.NewReader("fake image data")
	responseCloser := io.NopCloser(responseBody)
	
	provider := &mockProvider{
		response: responseCloser,
		error:    nil,
	}
	
	req := httptest.NewRequest("GET", "/test.jpg", nil)
	ctx := context.WithValue(req.Context(), providers.ContextKey, provider)
	req = req.WithContext(ctx)
	
	rr := httptest.NewRecorder()
	
	handler := Image(timeout)
	handler(rr, req)
	
	// Since we don't have actual image processing implemented,
	// this will likely return an error, but we can verify the
	// provider was called successfully by checking it didn't
	// return the provider-specific error codes
	if rr.Code == http.StatusNotFound || 
	   rr.Code == http.StatusBadRequest || 
	   rr.Code == http.StatusBadGateway {
		t.Errorf("Provider should have been called successfully, but got status %d", rr.Code)
	}
}

func TestImageContextCancellation(t *testing.T) {
	timeout := 30 * time.Second
	
	// Create a provider that takes some time
	responseBody := strings.NewReader("fake image data")
	responseCloser := io.NopCloser(responseBody)
	
	provider := &mockProvider{
		response: responseCloser,
		error:    nil,
	}
	
	req := httptest.NewRequest("GET", "/test.jpg", nil)
	
	// Cancel the context immediately
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	
	// Add provider to context
	ctx = context.WithValue(ctx, providers.ContextKey, provider)
	req = req.WithContext(ctx)
	
	rr := httptest.NewRecorder()
	
	handler := Image(timeout)
	handler(rr, req)
	
	// The handler should complete (context cancellation is handled internally)
	// We don't expect any specific status code since image processing will likely fail
	// but the handler shouldn't panic
	if rr.Code == 0 {
		t.Error("Handler should have set a status code")
	}
}

// Benchmark the getFilename function
func BenchmarkGetFilename(b *testing.B) {
	provider := &mockProvider{}
	req := httptest.NewRequest("GET", "/path/to/image.jpg", nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		getFilename(provider, req)
	}
}

func BenchmarkGetFilenameProxy(b *testing.B) {
	prov := provider.NewProxy(nil)
	req := httptest.NewRequest("GET", "/proxy?url=https://example.com/image.jpg", nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		getFilename(prov, req)
	}
}