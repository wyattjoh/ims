package provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// Mock transport for testing
type mockTransport struct {
	response *http.Response
	error    error
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.error != nil {
		return nil, m.error
	}
	return m.response, nil
}

func TestNewProxy(t *testing.T) {
	transport := &mockTransport{}
	proxy := NewProxy(transport)

	if proxy == nil {
		t.Fatal("NewProxy() returned nil")
	}

	if proxy.client == nil {
		t.Fatal("NewProxy() created proxy with nil client")
	}

	if proxy.client.Transport != transport {
		t.Error("NewProxy() did not set the transport correctly")
	}
}

func TestProxy_Provide(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		filename    string
		expectError error
	}{
		{
			name:        "valid http url",
			filename:    "http://example.com/image.jpg",
			expectError: nil,
		},
		{
			name:        "valid https url", 
			filename:    "https://example.com/image.jpg",
			expectError: nil,
		},
		{
			name:        "invalid url",
			filename:    ":\\invalid-url",
			expectError: ErrFilename,
		},
		{
			name:        "url with special characters",
			filename:    "https://example.com/path with spaces/image.jpg",
			expectError: nil,
		},
		{
			name:        "empty filename",
			filename:    "",
			expectError: nil, // url.Parse("") returns a valid empty URL
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock response
			response := &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("test content")),
				Header:     make(http.Header),
			}

			transport := &mockTransport{response: response}
			proxy := NewProxy(transport)

			reader, err := proxy.Provide(ctx, tt.filename)

			if tt.expectError != nil {
				if err == nil {
					t.Errorf("Expected error %v, got nil", tt.expectError)
					if reader != nil {
						reader.Close()
					}
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

			if reader != nil {
				reader.Close()
			}
		})
	}
}

func TestProxy_Handle(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		statusCode     int
		responseBody   string
		expectError    error
		expectContent  string
		transportError error
	}{
		{
			name:          "successful response",
			statusCode:    200,
			responseBody:  "image content",
			expectError:   nil,
			expectContent: "image content",
		},
		{
			name:        "not found response",
			statusCode:  404,
			expectError: ErrNotFound,
		},
		{
			name:        "bad gateway response",
			statusCode:  500,
			expectError: ErrBadGateway,
		},
		{
			name:        "unauthorized response",
			statusCode:  401,
			expectError: ErrBadGateway,
		},
		{
			name:           "transport error",
			transportError: io.EOF,
			expectError:    io.EOF, // Should wrap the error, but for testing we check the underlying
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var transport *mockTransport

			if tt.transportError != nil {
				transport = &mockTransport{error: tt.transportError}
			} else {
				response := &http.Response{
					StatusCode: tt.statusCode,
					Body:       io.NopCloser(strings.NewReader(tt.responseBody)),
					Header:     make(http.Header),
				}
				transport = &mockTransport{response: response}
			}

			proxy := NewProxy(transport)
			fileURL, _ := url.Parse("http://example.com/test.jpg")

			reader, err := proxy.Handle(ctx, fileURL)

			if tt.expectError != nil {
				if err == nil {
					t.Errorf("Expected error containing %v, got nil", tt.expectError)
					if reader != nil {
						reader.Close()
					}
					return
				}
				// For transport errors, check that the error is wrapped
				if tt.transportError != nil {
					if !strings.Contains(err.Error(), "cannot get file from provider") {
						t.Errorf("Expected wrapped transport error, got %v", err)
					}
				} else {
					// For HTTP status errors, check exact match
					if err != tt.expectError {
						t.Errorf("Expected error %v, got %v", tt.expectError, err)
					}
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if reader == nil {
				t.Error("Expected reader, got nil")
				return
			}
			defer reader.Close()

			// Read and verify content
			if tt.expectContent != "" {
				buf := make([]byte, len(tt.expectContent))
				n, err := reader.Read(buf)
				if err != nil && err != io.EOF {
					t.Errorf("Error reading content: %v", err)
					return
				}

				if string(buf[:n]) != tt.expectContent {
					t.Errorf("Expected content %q, got %q", tt.expectContent, string(buf[:n]))
				}
			}
		})
	}
}

func TestProxy_HandleWithContext(t *testing.T) {
	// Test that context cancellation is properly handled
	ctx, cancel := context.WithCancel(context.Background())

	// Create a test server that hangs
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if the request context was cancelled
		select {
		case <-r.Context().Done():
			// Request was cancelled, this is expected
			return
		default:
			// Write a response
			w.WriteHeader(200)
			w.Write([]byte("content"))
		}
	}))
	defer server.Close()

	proxy := NewProxy(nil) // Use default transport
	fileURL, _ := url.Parse(server.URL + "/test.jpg")

	// Cancel the context before making the request
	cancel()

	reader, err := proxy.Handle(ctx, fileURL)

	// Should get an error due to context cancellation
	if err == nil {
		t.Error("Expected error due to context cancellation, got nil")
		if reader != nil {
			reader.Close()
		}
		return
	}

	// Error should indicate context was cancelled
	if !strings.Contains(err.Error(), "context canceled") && 
	   !strings.Contains(err.Error(), "operation was canceled") {
		t.Errorf("Expected context cancellation error, got: %v", err)
	}
}

func TestProxy_HandleRealServer(t *testing.T) {
	// Test with a real HTTP server
	expectedContent := "test image data"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/test.jpg":
			w.WriteHeader(200)
			w.Write([]byte(expectedContent))
		case "/notfound.jpg":
			w.WriteHeader(404)
		case "/error.jpg":
			w.WriteHeader(500)
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	proxy := NewProxy(nil) // Use default transport
	ctx := context.Background()

	tests := []struct {
		name          string
		path          string
		expectError   error
		expectContent string
	}{
		{
			name:          "successful request",
			path:          "/test.jpg",
			expectContent: expectedContent,
		},
		{
			name:        "not found request",
			path:        "/notfound.jpg",
			expectError: ErrNotFound,
		},
		{
			name:        "server error request",
			path:        "/error.jpg",
			expectError: ErrBadGateway,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileURL, _ := url.Parse(server.URL + tt.path)
			reader, err := proxy.Handle(ctx, fileURL)

			if tt.expectError != nil {
				if err != tt.expectError {
					t.Errorf("Expected error %v, got %v", tt.expectError, err)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if reader == nil {
				t.Error("Expected reader, got nil")
				return
			}
			defer reader.Close()

			// Read content
			content, err := io.ReadAll(reader)
			if err != nil {
				t.Errorf("Error reading content: %v", err)
				return
			}

			if string(content) != tt.expectContent {
				t.Errorf("Expected content %q, got %q", tt.expectContent, string(content))
			}
		})
	}
}