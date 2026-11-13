package provider

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go"
)

func configureS3Test(t *testing.T) {
	t.Helper()
	t.Setenv("S3_ENDPOINT", "s3.example.test")
	t.Setenv("S3_ACCESS_KEY_ID", "test-access-key")
	t.Setenv("S3_ACCESS_KEY_SECRET", "test-secret-key")
	t.Setenv("S3_DONT_USE_SSL", "")
	// No retries or backoff in upstream-failure tests. These tests are not parallel.
	previous := minio.MaxRetry
	minio.MaxRetry = 1
	t.Cleanup(func() { minio.MaxRetry = previous })
}

func TestNewS3InvalidEndpoint(t *testing.T) {
	configureS3Test(t)
	for _, endpoint := range []string{"", "https://s3.example.test", "s3.example.test/path", "bad endpoint"} {
		t.Run(endpoint, func(t *testing.T) {
			t.Setenv("S3_ENDPOINT", endpoint)
			p, err := NewS3("test-bucket", cloudRoundTripper(func(*http.Request) (*http.Response, error) {
				t.Error("invalid endpoint should not make requests")
				return nil, errors.New("unexpected request")
			}))
			if p != nil || err == nil || !strings.Contains(err.Error(), "cannot create the s3 client") {
				t.Fatalf("NewS3() = (%v, %v), want nil provider and wrapped configuration error", p, err)
			}
		})
	}
}

func TestS3ProvideResponses(t *testing.T) {
	configureS3Test(t)
	for _, tt := range []struct {
		name      string
		status    int
		body      string
		errorCode string
	}{
		{name: "image", status: http.StatusOK, body: "image bytes\x00\xff"},
		{name: "empty object", status: http.StatusOK},
		{name: "missing object", status: http.StatusNotFound, errorCode: "NoSuchKey"},
		{name: "missing bucket", status: http.StatusNotFound, errorCode: "NoSuchBucket"},
		{name: "invalid credentials", status: http.StatusForbidden, errorCode: "InvalidAccessKeyId"},
		{name: "access denied", status: http.StatusForbidden, errorCode: "AccessDenied"},
		{name: "upstream failure", status: http.StatusServiceUnavailable, errorCode: "ServiceUnavailable"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var locationCalls, objectCalls int
			payload := tt.body
			if tt.errorCode != "" {
				payload = "<Error><Code>" + tt.errorCode + "</Code><Message>test failure</Message></Error>"
			}
			body := &cloudTrackedBody{Reader: strings.NewReader(payload), closed: make(chan struct{})}
			p, err := NewS3("test-bucket", cloudRoundTripper(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodGet || r.URL.Host != "s3.example.test" || r.URL.Scheme != "https" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				if !strings.Contains(r.Header.Get("Authorization"), "Credential=test-access-key/") {
					t.Errorf("request not signed with configured access key: %q", r.Header.Get("Authorization"))
				}
				if r.URL.Query().Has("location") {
					locationCalls++
					if r.URL.Path != "/test-bucket/" && r.URL.Path != "/test-bucket" {
						t.Errorf("bucket location path = %q", r.URL.Path)
					}
					return cloudResponse(r, http.StatusOK, `<LocationConstraint>us-east-1</LocationConstraint>`), nil
				}
				objectCalls++
				if r.URL.Path != "/test-bucket/folder/image photo.png" {
					t.Errorf("object path = %q", r.URL.Path)
				}
				response := cloudResponse(r, tt.status, payload)
				response.Header.Set("Last-Modified", "Mon, 02 Jan 2006 15:04:05 GMT")
				response.Header.Set("Content-Type", "application/octet-stream")
				response.Body = body
				return response, nil
			}))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			reader, err := p.Provide(ctx, "folder/image photo.png")
			switch {
			case tt.status == http.StatusOK:
				if err != nil {
					t.Fatal(err)
				}
				defer reader.Close()
				got, readErr := io.ReadAll(reader)
				if readErr != nil || string(got) != tt.body {
					t.Errorf("ReadAll() = (%q, %v), want %q", got, readErr, tt.body)
				}
			case tt.errorCode == "NoSuchKey":
				if reader != nil || !errors.Is(err, ErrNotFound) {
					t.Fatalf("Provide() = (%v, %v), want nil reader and ErrNotFound", reader, err)
				}
			default:
				if reader != nil || err == nil || !strings.Contains(err.Error(), "could not copy into the buffer") {
					t.Fatalf("Provide() = (%v, %v), want wrapped read error", reader, err)
				}
				if errors.Is(err, ErrNotFound) {
					t.Fatal("only NoSuchKey should be translated to ErrNotFound")
				}
				var responseError minio.ErrorResponse
				if !errors.As(err, &responseError) || responseError.Code != tt.errorCode {
					t.Errorf("error = %v, want S3 error code %q", err, tt.errorCode)
				}
			}
			if locationCalls != 1 || objectCalls != 1 {
				t.Errorf("location/object calls = %d/%d, want 1/1", locationCalls, objectCalls)
			}
			// Minio closes the HTTP reader from its worker goroutine after Object.Close.
			select {
			case <-body.closed:
			case <-time.After(time.Second):
				t.Error("upstream body was not closed after buffering")
			}
		})
	}
}

func TestS3ProvideInvalidNames(t *testing.T) {
	configureS3Test(t)
	for _, tt := range []struct {
		name, bucket, filename string
	}{
		{name: "invalid bucket", bucket: "x", filename: "image.png"},
		{name: "empty object", bucket: "test-bucket"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewS3(tt.bucket, cloudRoundTripper(func(*http.Request) (*http.Response, error) {
				t.Error("invalid object or bucket should fail before any request")
				return nil, errors.New("unexpected request")
			}))
			if err != nil {
				t.Fatal(err)
			}
			reader, err := p.Provide(context.Background(), tt.filename)
			if reader != nil || err == nil || !strings.Contains(err.Error(), "cannot get object from provider") {
				t.Fatalf("Provide() = (%v, %v), want nil reader and wrapped validation error", reader, err)
			}
		})
	}
}

type cloudErrorReader struct{ err error }

func (r cloudErrorReader) Read([]byte) (int, error) { return 0, r.err }

func TestS3ProvideTransportAndBodyErrors(t *testing.T) {
	configureS3Test(t)
	for _, stage := range []string{"location transport", "object transport", "object body"} {
		t.Run(stage, func(t *testing.T) {
			failure := errors.New("test upstream failure")
			p, err := NewS3("test-bucket", cloudRoundTripper(func(r *http.Request) (*http.Response, error) {
				if r.URL.Query().Has("location") {
					if stage == "location transport" {
						return nil, failure
					}
					return cloudResponse(r, http.StatusOK, `<LocationConstraint>us-east-1</LocationConstraint>`), nil
				}
				if stage == "object transport" {
					return nil, failure
				}
				response := cloudResponse(r, http.StatusOK, "placeholder")
				response.Header.Set("Last-Modified", "Mon, 02 Jan 2006 15:04:05 GMT")
				response.Body = io.NopCloser(cloudErrorReader{err: failure})
				return response, nil
			}))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			reader, err := p.Provide(ctx, "image.png")
			if reader != nil || !errors.Is(err, failure) || !strings.Contains(err.Error(), "could not copy into the buffer") {
				t.Fatalf("Provide() = (%v, %v), want wrapped upstream failure", reader, err)
			}
		})
	}
}

func TestS3ProvideCallerCancellationAndDeadline(t *testing.T) {
	configureS3Test(t)
	for _, tt := range []struct {
		name      string
		cancel    bool
		wantError error
	}{
		{name: "canceled during request", cancel: true, wantError: context.Canceled},
		{name: "deadline during request", wantError: context.DeadlineExceeded},
	} {
		t.Run(tt.name, func(t *testing.T) {
			type contextKey struct{}
			ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), contextKey{}, "caller-value"), time.Second)
			defer cancel()
			deadline, _ := ctx.Deadline()
			started := make(chan context.Context, 1)
			p, err := NewS3("test-bucket", cloudRoundTripper(func(r *http.Request) (*http.Response, error) {
				if r.URL.Query().Has("location") {
					return cloudResponse(r, http.StatusOK, `<LocationConstraint>us-east-1</LocationConstraint>`), nil
				}
				requestContext := r.Context()
				started <- requestContext
				if requestContext.Value(contextKey{}) != "caller-value" {
					return nil, errors.New("caller context did not reach object transport")
				}
				watchdog := time.NewTimer(3 * time.Second)
				defer watchdog.Stop()
				select {
				case <-requestContext.Done():
					return nil, requestContext.Err()
				case <-watchdog.C:
					return nil, errors.New("object transport did not observe caller cancellation")
				}
			}))
			if err != nil {
				t.Fatal(err)
			}
			completed := make(chan error, 1)
			go func() {
				reader, err := p.Provide(ctx, "image.png")
				if reader != nil {
					reader.Close()
				}
				completed <- err
			}()
			select {
			case requestContext := <-started:
				if got := requestContext.Value(contextKey{}); got != "caller-value" {
					t.Errorf("object request context value = %v, want caller-value", got)
				}
				if got, ok := requestContext.Deadline(); !ok || !got.Equal(deadline) {
					t.Errorf("object request deadline = (%v, %v), want %v", got, ok, deadline)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("object request did not reach fake transport before timeout")
			}
			if tt.cancel {
				cancel()
			}
			select {
			case err := <-completed:
				if !errors.Is(err, tt.wantError) || !strings.Contains(err.Error(), "could not copy into the buffer") {
					t.Errorf("Provide() error = %v, want wrapped %v", err, tt.wantError)
				}
			case <-time.After(4 * time.Second):
				t.Fatal("Provide did not return after caller cancellation/deadline")
			}
		})
	}
}

func TestS3SSLConfigurationAndAnonymousAccess(t *testing.T) {
	configureS3Test(t)
	t.Setenv("S3_ACCESS_KEY_ID", "")
	t.Setenv("S3_ACCESS_KEY_SECRET", "")
	for _, tt := range []struct {
		setting, scheme string
	}{
		{setting: "TRUE", scheme: "http"},
		{setting: "true", scheme: "https"},
		{setting: "", scheme: "https"},
	} {
		t.Run("SSL setting="+tt.setting, func(t *testing.T) {
			t.Setenv("S3_DONT_USE_SSL", tt.setting)
			calls := 0
			p, err := NewS3("test-bucket", cloudRoundTripper(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.URL.Scheme != tt.scheme || r.Header.Get("Authorization") != "" {
					t.Errorf("request = %s, Authorization = %q; want scheme %s and anonymous request", r.URL, r.Header.Get("Authorization"), tt.scheme)
				}
				if r.URL.Query().Has("location") {
					return cloudResponse(r, http.StatusOK, `<LocationConstraint>us-east-1</LocationConstraint>`), nil
				}
				response := cloudResponse(r, http.StatusOK, "image")
				response.Header.Set("Last-Modified", "Mon, 02 Jan 2006 15:04:05 GMT")
				return response, nil
			}))
			if err != nil {
				t.Fatal(err)
			}
			for range 2 {
				reader, err := p.Provide(context.Background(), "image.png")
				if err != nil {
					t.Fatal(err)
				}
				reader.Close()
			}
			if calls != 3 {
				t.Errorf("requests = %d, want 1 location + 2 objects (bucket location reused)", calls)
			}
		})
	}
}
