package provider

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
)

type cloudRoundTripper func(*http.Request) (*http.Response, error)

func (f cloudRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func cloudResponse(r *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode:    status,
		Status:        http.StatusText(status),
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       r,
	}
}

type cloudTrackedBody struct {
	io.Reader
	closed chan struct{}
	once   sync.Once
}

func (b *cloudTrackedBody) Close() error {
	b.once.Do(func() { close(b.closed) })
	return nil
}

func TestNewGCSTransportCredentialErrors(t *testing.T) {
	for _, tt := range []struct {
		name    string
		content string
		missing bool
	}{
		{name: "missing file", missing: true},
		{name: "invalid JSON", content: "not JSON"},
		{name: "unsupported credential type", content: `{"type":"not-a-credential"}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			credentials := filepath.Join(t.TempDir(), "credentials.json")
			if !tt.missing {
				if err := os.WriteFile(credentials, []byte(tt.content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", credentials)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			transport, err := NewGCSTransport(ctx)
			if transport != nil || err == nil || !strings.Contains(err.Error(), "cannot create token source") {
				t.Fatalf("NewGCSTransport() = (%v, %v), want nil transport and wrapped credential error", transport, err)
			}
		})
	}
}

func TestNewGCSTransportAuthorizedRequest(t *testing.T) {
	credentials := filepath.Join(t.TempDir(), "credentials.json")
	if err := os.WriteFile(credentials, []byte(`{"type":"authorized_user","client_id":"test-client","client_secret":"test-secret","refresh_token":"test-refresh-token"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", credentials)
	var tokenCalls, objectCalls int
	base := cloudRoundTripper(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodPost {
			tokenCalls++
			if err := r.ParseForm(); err != nil {
				return nil, err
			}
			if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != "test-refresh-token" {
				t.Errorf("unexpected token request form: %v", r.Form)
			}
			response := cloudResponse(r, http.StatusOK, `{"access_token":"test-access-token","token_type":"Bearer","expires_in":3600}`)
			response.Header.Set("Content-Type", "application/json")
			return response, nil
		}
		objectCalls++
		if got := r.Header.Get("Authorization"); got != "Bearer test-access-token" {
			t.Errorf("Authorization = %q, want test bearer token", got)
		}
		return cloudResponse(r, http.StatusOK, "image"), nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, oauth2.HTTPClient, &http.Client{Transport: base, Timeout: 2 * time.Second})
	transport, err := NewGCSTransport(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if tokenCalls != 0 {
		t.Fatal("creating the transport should not fetch a token")
	}
	for range 2 {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://storage.example.test/bucket/image", nil)
		if err != nil {
			t.Fatal(err)
		}
		response, err := transport.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
	}
	if tokenCalls != 1 || objectCalls != 2 {
		t.Errorf("token/object calls = %d/%d, want 1/2 (cached token)", tokenCalls, objectCalls)
	}
}

func TestNewGCSInvalidEmulatorEndpoint(t *testing.T) {
	t.Setenv("STORAGE_EMULATOR_HOST", "http://%invalid")
	p, err := NewGCS(context.Background(), "test-bucket", cloudRoundTripper(func(*http.Request) (*http.Response, error) {
		t.Error("invalid configuration should not make requests")
		return nil, errors.New("unexpected request")
	}))
	if p != nil || err == nil || !strings.Contains(err.Error(), "cannot create storage client") {
		t.Fatalf("NewGCS() = (%v, %v), want nil provider and wrapped client error", p, err)
	}
}

func TestGCSProvideResponses(t *testing.T) {
	for _, tt := range []struct {
		name   string
		status int
		body   string
	}{
		{name: "image", status: http.StatusOK, body: "image bytes\x00\xff"},
		{name: "empty object", status: http.StatusOK},
		{name: "missing object", status: http.StatusNotFound, body: "missing"},
		{name: "access denied", status: http.StatusForbidden, body: "denied"},
		{name: "upstream failure", status: http.StatusServiceUnavailable, body: "unavailable"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Emulator mode avoids ADC lookup; the injected transport handles every request.
			t.Setenv("STORAGE_EMULATOR_HOST", "https://storage.example.test")
			body := &cloudTrackedBody{Reader: strings.NewReader(tt.body), closed: make(chan struct{})}
			calls := 0
			p, err := NewGCS(context.Background(), "test-bucket", cloudRoundTripper(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != http.MethodGet || r.URL.Host != "storage.example.test" || r.URL.Path != "/test-bucket/folder/image photo.png" {
					t.Errorf("unexpected object request: %s %s", r.Method, r.URL)
				}
				response := cloudResponse(r, tt.status, tt.body)
				response.Body = body
				return response, nil
			}))
			if err != nil {
				t.Fatal(err)
			}
			// Exercise upstream errors without SDK retry delays.
			p.bucket = p.bucket.Retryer(storage.WithPolicy(storage.RetryNever))
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			reader, err := p.Provide(ctx, "folder/image photo.png")
			switch tt.status {
			case http.StatusOK:
				if err != nil {
					t.Fatal(err)
				}
				defer reader.Close()
				got, readErr := io.ReadAll(reader)
				if readErr != nil || string(got) != tt.body {
					t.Errorf("ReadAll() = (%q, %v), want %q", got, readErr, tt.body)
				}
				if err := reader.Close(); err != nil {
					t.Error(err)
				}
			case http.StatusNotFound:
				if reader != nil || !errors.Is(err, ErrNotFound) {
					t.Fatalf("Provide() = (%v, %v), want nil reader and ErrNotFound", reader, err)
				}
			default:
				var apiError *googleapi.Error
				if reader != nil || !errors.As(err, &apiError) || apiError.Code != tt.status || !strings.Contains(err.Error(), "cannot get file from provider") {
					t.Fatalf("Provide() = (%v, %v), want wrapped HTTP %d error", reader, err, tt.status)
				}
			}
			if calls != 1 {
				t.Errorf("transport calls = %d, want 1", calls)
			}
			select {
			case <-body.closed:
			case <-time.After(time.Second):
				t.Error("response body was not closed")
			}
		})
	}
}

func TestGCSProvideTransportAndContextErrors(t *testing.T) {
	t.Setenv("STORAGE_EMULATOR_HOST", "https://storage.example.test")
	transportError := errors.New("test transport failure")
	calls := 0
	p, err := NewGCS(context.Background(), "test-bucket", cloudRoundTripper(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, transportError
	}))
	if err != nil {
		t.Fatal(err)
	}
	p.bucket = p.bucket.Retryer(storage.WithPolicy(storage.RetryNever))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	reader, err := p.Provide(ctx, "image.png")
	if reader != nil || !errors.Is(err, transportError) || !strings.Contains(err.Error(), "cannot get file from provider") {
		t.Fatalf("Provide() = (%v, %v), want wrapped transport error", reader, err)
	}
	if calls != 1 {
		t.Fatalf("transport calls = %d, want 1", calls)
	}
	cancel()
	reader, err = p.Provide(ctx, "image.png")
	if reader != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("Provide(canceled) = (%v, %v), want context.Canceled", reader, err)
	}
	if calls != 1 {
		t.Errorf("canceled request reached transport: calls = %d", calls)
	}
}
