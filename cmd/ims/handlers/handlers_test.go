package handlers

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
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
	ctx      context.Context
	filename string
	calls    int
}

func (m *mockProvider) Provide(ctx context.Context, filename string) (io.ReadCloser, error) {
	m.ctx = ctx
	m.filename = filename
	m.calls++
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
		name          string
		provider      provider.Provider
		setupContext  bool
		path          string
		query         string
		expectStatus  int
		providerError error
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

type trackedReadCloser struct {
	io.Reader
	closes int
}

func (r *trackedReadCloser) Close() error {
	r.closes++
	return nil
}

func TestImageWithSuccessfulProvider(t *testing.T) {
	var input bytes.Buffer
	if err := png.Encode(&input, image.NewNRGBA(image.Rect(0, 0, 4, 2))); err != nil {
		t.Fatal(err)
	}
	body := &trackedReadCloser{Reader: &input}
	p := &mockProvider{response: body}
	req := httptest.NewRequest(http.MethodGet, "/nested/test.png?width=2", nil)
	req = req.WithContext(context.WithValue(req.Context(), providers.ContextKey, p))
	rr := httptest.NewRecorder()

	Image(30*time.Second)(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q; want 200", rr.Code, rr.Body.String())
	}
	if p.calls != 1 || p.filename != "nested/test.png" {
		t.Errorf("provider calls=%d filename=%q, want 1 and nested/test.png", p.calls, p.filename)
	}
	if body.closes != 1 {
		t.Errorf("body closed %d times, want 1", body.closes)
	}
	if got := rr.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", got)
	}
	if got := rr.Header().Get("Cache-Control"); got != "public, max-age=30" {
		t.Errorf("Cache-Control = %q", got)
	}
	decoded, err := png.Decode(rr.Body)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds().Size() != image.Pt(2, 1) {
		t.Errorf("image size = %v, want (2,1)", decoded.Bounds().Size())
	}
}

func TestImageContextCancellation(t *testing.T) {
	p := &mockProvider{error: context.Canceled}
	req := httptest.NewRequest(http.MethodGet, "/test.jpg", nil)
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(context.WithValue(ctx, providers.ContextKey, p))
	rr := httptest.NewRecorder()

	Image(30*time.Second)(rr, req)

	if p.calls != 1 || p.ctx == nil || !errors.Is(p.ctx.Err(), context.Canceled) {
		t.Fatalf("provider did not receive cancellation: calls=%d context=%v", p.calls, p.ctx)
	}
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}

func TestImageClosesProviderResponseOnDecodeError(t *testing.T) {
	body := &trackedReadCloser{Reader: strings.NewReader("not an image")}
	p := &mockProvider{response: body}
	req := httptest.NewRequest(http.MethodGet, "/test.png", nil)
	req = req.WithContext(context.WithValue(req.Context(), providers.ContextKey, p))
	rr := httptest.NewRecorder()

	Image(0)(rr, req)

	if rr.Code != http.StatusInternalServerError || body.closes != 1 {
		t.Fatalf("status=%d closes=%d, want 500 and 1", rr.Code, body.closes)
	}
	if !strings.Contains(rr.Body.String(), "can't decode the image") {
		t.Errorf("body = %q, want decode error", rr.Body.String())
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
