package providers_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wyattjoh/ims/internal/image/provider"
	"github.com/wyattjoh/ims/internal/platform/providers"
)

type cacheTestRoundTripper func(*http.Request) (*http.Response, error)

func (f cacheTestRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestMiddlewareRejectsUnmappedHosts(t *testing.T) {
	p := providers.NewProviders(map[string]provider.Provider{
		"known.example:8080": &provider.Filesystem{Dir: http.Dir(t.TempDir())},
		"nil.example":        nil,
	})
	for _, host := range []string{"unknown.example", "known.example", "known.example:9090", "", "nil.example"} {
		t.Run(host, func(t *testing.T) {
			called := false
			handler := providers.Middleware(p, func(http.ResponseWriter, *http.Request) { called = true })
			req := httptest.NewRequest(http.MethodGet, "http://known.example:8080/image.png", nil)
			req.Host = host
			response := httptest.NewRecorder()
			handler(response, req)
			if called {
				t.Error("next handler was called for unmapped host")
			}
			if response.Code != http.StatusBadRequest || response.Body.String() != "No such host: "+host+"\n" {
				t.Errorf("response = %d %q, want 400 with rejected host", response.Code, response.Body.String())
			}
			if req.Context().Value(providers.ContextKey) != nil {
				t.Error("middleware mutated the original request context")
			}
		})
	}
}

func TestMiddlewarePreservesContextAndOriginalRequest(t *testing.T) {
	type contextKey struct{}
	ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), contextKey{}, "request-value"), 2*time.Second)
	defer cancel()
	p := &provider.Filesystem{Dir: http.Dir(t.TempDir())}
	original := httptest.NewRequest(http.MethodGet, "http://known.example/image.png", nil).WithContext(ctx)
	deadline, _ := ctx.Deadline()
	called := false
	handler := providers.Middleware(providers.NewProviders(map[string]provider.Provider{"known.example": p}), func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r == original {
			t.Error("middleware should clone the request before adding context")
		}
		if r.Context().Value(providers.ContextKey) != p || r.Context().Value(contextKey{}) != "request-value" {
			t.Error("provider or existing context value was lost")
		}
		if got, ok := r.Context().Deadline(); !ok || !got.Equal(deadline) {
			t.Errorf("deadline = (%v, %v), want %v", got, ok, deadline)
		}
		cancel()
		if !errors.Is(r.Context().Err(), context.Canceled) {
			t.Error("parent cancellation was not propagated")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	response := httptest.NewRecorder()
	handler(response, original)
	if !called || response.Code != http.StatusNoContent {
		t.Fatalf("handler called = %v, status = %d", called, response.Code)
	}
	if original.Context() != ctx || original.Context().Value(providers.ContextKey) != nil {
		t.Error("original request context was modified")
	}
}

func TestCacheRoundTripperBehavior(t *testing.T) {
	for _, cacheKind := range []string{"disabled", "memory", "disk"} {
		t.Run(cacheKind, func(t *testing.T) {
			cache := ""
			if cacheKind == "memory" {
				cache = ":memory:"
			} else if cacheKind == "disk" {
				cache = t.TempDir()
			}
			calls := make(map[string]int)
			underlying := cacheTestRoundTripper(func(r *http.Request) (*http.Response, error) {
				key := r.URL.RequestURI()
				calls[key]++
				if r.URL.Path == "/transport-error" {
					return nil, io.ErrClosedPipe
				}
				status := http.StatusOK
				cacheControl := "public, max-age=3600"
				if r.URL.Path == "/no-store" {
					cacheControl = "no-store"
				}
				if r.URL.Path == "/unavailable" {
					status = http.StatusServiceUnavailable
					cacheControl = "no-store"
				}
				return &http.Response{
					StatusCode: status,
					Header: http.Header{
						"Cache-Control": {cacheControl},
						"Date":          {time.Now().UTC().Format(http.TimeFormat)},
					},
					Body:          io.NopCloser(strings.NewReader("content:" + key)),
					ContentLength: int64(len("content:" + key)),
					Request:       r,
				}, nil
			})
			transport, err := providers.WrapCacheRoundTripper(context.Background(), underlying, cache)
			if err != nil {
				t.Fatal(err)
			}
			client := &http.Client{Transport: transport, Timeout: 2 * time.Second}
			for _, path := range []string{"/image?v=1", "/image?v=2", "/no-store", "/unavailable", "/transport-error"} {
				for range 2 {
					response, err := client.Get("http://origin.example.test" + path)
					if path == "/transport-error" {
						if !errors.Is(err, io.ErrClosedPipe) {
							t.Fatalf("Get(%q) error = %v, want transport error", path, err)
						}
						continue
					}
					if err != nil {
						t.Fatal(err)
					}
					body, readErr := io.ReadAll(response.Body)
					response.Body.Close()
					if readErr != nil || string(body) != "content:"+path {
						t.Errorf("Get(%q) body = (%q, %v)", path, body, readErr)
					}
					wantStatus := http.StatusOK
					if path == "/unavailable" {
						wantStatus = http.StatusServiceUnavailable
					}
					if response.StatusCode != wantStatus {
						t.Errorf("Get(%q) status = %d, want %d", path, response.StatusCode, wantStatus)
					}
				}
				wantCalls := 2
				if cacheKind != "disabled" && strings.HasPrefix(path, "/image?") {
					wantCalls = 1
				}
				if calls[path] != wantCalls {
					t.Errorf("upstream calls for %q = %d, want %d", path, calls[path], wantCalls)
				}
			}
		})
	}
}

func TestCacheRevalidationAndDiskPersistence(t *testing.T) {
	for _, cacheKind := range []string{"memory", "disk"} {
		t.Run(cacheKind, func(t *testing.T) {
			cache := ":memory:"
			if cacheKind == "disk" {
				cache = t.TempDir()
			}
			calls := 0
			underlying := cacheTestRoundTripper(func(r *http.Request) (*http.Response, error) {
				calls++
				response := &http.Response{
					StatusCode: http.StatusOK,
					Header: http.Header{
						"Cache-Control": {"no-cache"},
						"Etag":          {`"image-version-1"`},
						"Last-Modified": {"Mon, 02 Jan 2006 15:04:05 GMT"},
						"Date":          {time.Now().UTC().Format(http.TimeFormat)},
					},
					Body:          io.NopCloser(strings.NewReader("cached image")),
					ContentLength: int64(len("cached image")),
					Request:       r,
				}
				if calls == 1 {
					if r.Header.Get("If-None-Match") != "" {
						t.Error("first request unexpectedly has a cache validator")
					}
					return response, nil
				}
				if r.Header.Get("If-None-Match") != `"image-version-1"` || r.Header.Get("If-Modified-Since") != "Mon, 02 Jan 2006 15:04:05 GMT" {
					t.Errorf("revalidation headers = %v, want cached ETag and modification date", r.Header)
				}
				response.StatusCode = http.StatusNotModified
				response.Header.Set("Cache-Control", "public, max-age=3600")
				response.Body = http.NoBody
				response.ContentLength = 0
				return response, nil
			})
			transport, err := providers.WrapCacheRoundTripper(context.Background(), underlying, cache)
			if err != nil {
				t.Fatal(err)
			}
			client := &http.Client{Transport: transport, Timeout: 2 * time.Second}
			for attempt := range 3 {
				if cacheKind == "disk" && attempt == 1 {
					// A fresh wrapper must recover the cached response from the same directory.
					client.Transport, err = providers.WrapCacheRoundTripper(context.Background(), underlying, cache)
					if err != nil {
						t.Fatal(err)
					}
				}
				response, err := client.Get("http://origin.example.test/image.png")
				if err != nil {
					t.Fatal(err)
				}
				body, readErr := io.ReadAll(response.Body)
				response.Body.Close()
				if readErr != nil || response.StatusCode != http.StatusOK || string(body) != "cached image" {
					t.Fatalf("attempt %d response = %d %q (%v), want original 200 body", attempt, response.StatusCode, body, readErr)
				}
			}
			if calls != 2 {
				t.Errorf("upstream calls = %d, want initial response + revalidation (then fresh cache hit)", calls)
			}
		})
	}
}

func TestNewMiddlewareRoutingAndCacheIntegration(t *testing.T) {
	for _, cacheKind := range []string{"disabled", "memory", "disk"} {
		t.Run(cacheKind, func(t *testing.T) {
			var calls atomic.Int32
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Cache-Control", "public, max-age=3600")
				_, _ = io.WriteString(w, "origin:"+r.URL.Path)
			}))
			defer origin.Close()
			directory := t.TempDir()
			if err := os.WriteFile(filepath.Join(directory, "image.png"), []byte("local image"), 0o600); err != nil {
				t.Fatal(err)
			}
			cache := ""
			if cacheKind == "memory" {
				cache = ":memory:"
			} else if cacheKind == "disk" {
				cache = t.TempDir()
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			p, err := providers.New(ctx, "files.example:8080", []string{
				directory,
				"first.example," + origin.URL + "/first/",
				"second.example," + origin.URL + "/second/",
				"proxy.example,:proxy:",
			}, cache, "test-signing-secret", true)
			if err != nil {
				t.Fatal(err)
			}
			handler := providers.Middleware(p, func(w http.ResponseWriter, r *http.Request) {
				selected, ok := r.Context().Value(providers.ContextKey).(provider.Provider)
				if !ok {
					t.Error("middleware did not attach a provider")
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				filename := strings.TrimPrefix(r.URL.Path, "/")
				if r.Host == "proxy.example" {
					filename = origin.URL + "/proxy/image.png"
				}
				reader, err := selected.Provide(r.Context(), filename)
				if err != nil {
					t.Errorf("Provide(%q): %v", filename, err)
					w.WriteHeader(http.StatusBadGateway)
					return
				}
				defer reader.Close()
				if _, err := io.Copy(w, reader); err != nil {
					t.Error(err)
				}
			})
			for _, tt := range []struct {
				host, wantBody string
			}{
				{host: "files.example:8080", wantBody: "local image"},
				{host: "first.example", wantBody: "origin:/first/image.png"},
				{host: "second.example", wantBody: "origin:/second/image.png"},
				{host: "proxy.example", wantBody: "origin:/proxy/image.png"},
			} {
				for range 2 {
					req := httptest.NewRequest(http.MethodGet, "http://"+tt.host+"/image.png", nil).WithContext(ctx)
					response := httptest.NewRecorder()
					handler(response, req)
					if response.Code != http.StatusOK || response.Body.String() != tt.wantBody {
						t.Errorf("host %s response = %d %q, want 200 %q", tt.host, response.Code, response.Body.String(), tt.wantBody)
					}
				}
			}
			wantCalls := int32(3)
			if cacheKind == "disabled" {
				wantCalls = 6
			}
			if got := calls.Load(); got != wantCalls {
				t.Errorf("origin requests = %d, want %d", got, wantCalls)
			}
		})
	}
}
