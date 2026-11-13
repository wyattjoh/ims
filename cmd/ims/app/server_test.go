package app_test

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/wyattjoh/ims/cmd/ims/app"
)

func TestMountEndpoint(t *testing.T) {
	for _, tt := range []struct {
		name     string
		endpoint string
		path     string
		wantCode int
		wantCall bool
	}{
		{name: "exact route", endpoint: "/image", path: "/image", wantCode: http.StatusAccepted, wantCall: true},
		{name: "subtree", endpoint: "/images/", path: "/images/photo.png", wantCode: http.StatusAccepted, wantCall: true},
		{name: "root fallback", endpoint: "/", path: "/photo.png", wantCode: http.StatusAccepted, wantCall: true},
		{name: "unmatched", endpoint: "/image", path: "/other", wantCode: http.StatusNotFound},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			request := httptest.NewRequest(http.MethodGet, tt.path+"?width=20", nil)
			calls := 0
			app.MountEndpoint(mux, tt.endpoint, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r != request {
					t.Error("mounted handler did not receive the original request")
				}
				w.Header().Set("X-Endpoint", tt.endpoint)
				w.WriteHeader(http.StatusAccepted)
				_, _ = w.Write([]byte("mounted"))
			}))

			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			if response.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", response.Code, tt.wantCode)
			}
			wantCalls := 0
			if tt.wantCall {
				wantCalls = 1
				if response.Header().Get("X-Endpoint") != tt.endpoint || response.Body.String() != "mounted" {
					t.Errorf("handler response was not preserved: %v %q", response.Header(), response.Body.String())
				}
			}
			if calls != wantCalls {
				t.Errorf("handler called %d times, want %d", calls, wantCalls)
			}
		})
	}
}

func TestServeProviderValidation(t *testing.T) {
	for _, tt := range []struct {
		name        string
		backends    []string
		secret      string
		includePath bool
		wantError   string
	}{
		{name: "no providers", wantError: "no provider selected"},
		{name: "blank backend", backends: []string{""}, wantError: "cannot parse the backend: cannot be blank"},
		{name: "too many backend fields", backends: []string{"one,two,three"}, wantError: "expected form <host>,<origin> OR <origin>"},
		{name: "duplicate host", backends: []string{"images.test,/first", "images.test,/second"}, wantError: "host images.test already has a provider attached"},
		{name: "malformed origin", backends: []string{"http://%"}, wantError: "cannot parse the origin url"},
		{name: "unsupported scheme", backends: []string{"ftp://images.test"}, wantError: "scheme could not be matched"},
		{name: "unsigned proxy", backends: []string{":proxy:"}, wantError: "a signing secret is required"},
		{name: "proxy without signed path", backends: []string{":proxy:"}, secret: "test-secret", wantError: "a signing with path is required"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := app.Serve(&app.ServerOpts{
				Addr: "127.0.0.1:65536", Backends: tt.backends,
				SigningSecret: tt.secret, IncludePath: tt.includePath, DisableMetrics: true,
			})
			if err == nil || !strings.Contains(err.Error(), "cannot create providers") || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("Serve() error = %v, want provider error containing %q", err, tt.wantError)
			}
		})
	}
}

func TestServeListenFailure(t *testing.T) {
	for _, tt := range []struct {
		name string
		opts app.ServerOpts
	}{
		{name: "minimal", opts: app.ServerOpts{DisableMetrics: true}},
		{name: "metrics", opts: app.ServerOpts{}},
		{name: "debug signing cache and cors", opts: app.ServerOpts{
			Debug: true, DisableMetrics: true, SigningSecret: "test-secret", IncludePath: true,
			CacheTimeout: time.Minute, CORSDomains: []string{"https://images.test"},
		}},
		{name: "signed proxy", opts: app.ServerOpts{
			DisableMetrics: true, Backends: []string{":proxy:"}, SigningSecret: "test-secret", IncludePath: true, OriginCache: ":memory:",
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Isolate registrations from the process-wide Prometheus registry.
			oldRegisterer, oldGatherer := prometheus.DefaultRegisterer, prometheus.DefaultGatherer
			registry := prometheus.NewRegistry()
			prometheus.DefaultRegisterer, prometheus.DefaultGatherer = registry, registry
			t.Cleanup(func() {
				prometheus.DefaultRegisterer, prometheus.DefaultGatherer = oldRegisterer, oldGatherer
			})

			opts := tt.opts
			// An out-of-range numeric port fails before any socket can bind.
			opts.Addr = "127.0.0.1:65536"
			if len(opts.Backends) == 0 {
				opts.Backends = []string{t.TempDir()}
			}
			err := app.Serve(&opts)
			if err == nil || !strings.Contains(err.Error(), "could not listen on the address for http traffic") {
				t.Fatalf("Serve() error = %v, want wrapped listen failure", err)
			}
			var addressError *net.AddrError
			if !errors.As(err, &addressError) {
				t.Errorf("Serve() error = %v, want underlying *net.AddrError", err)
			}
			families, gatherErr := registry.Gather()
			if gatherErr != nil {
				t.Fatal(gatherErr)
			}
			if opts.DisableMetrics && len(families) != 0 {
				t.Errorf("metrics disabled but registered %d metric families", len(families))
			}
			if !opts.DisableMetrics && len(families) == 0 {
				t.Error("metrics enabled but no metric families registered")
			}
		})
	}
}
