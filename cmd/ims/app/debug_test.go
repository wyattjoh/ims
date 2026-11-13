package app_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wyattjoh/ims/cmd/ims/app"
)

func TestMountDebugRoutes(t *testing.T) {
	mux := http.NewServeMux()
	app.MountDebug(mux)
	for _, path := range []string{
		"/debug/pprof/", "/debug/pprof/cmdline", "/debug/pprof/profile",
		"/debug/pprof/symbol", "/debug/pprof/trace",
	} {
		t.Run(path, func(t *testing.T) {
			// Inspect registration only: invoking profile/trace would wait and
			// change process-wide runtime profiling state.
			_, pattern := mux.Handler(httptest.NewRequest(http.MethodGet, path, nil))
			if pattern != path {
				t.Errorf("registered pattern = %q, want %q", pattern, path)
			}
		})
	}
}

func TestMountDebugResponses(t *testing.T) {
	mux := http.NewServeMux()
	app.MountDebug(mux)
	for _, tt := range []struct {
		path     string
		wantCode int
		wantBody string
	}{
		{path: "/debug/pprof/", wantCode: http.StatusOK, wantBody: "Types of profiles available"},
		{path: "/debug/pprof/cmdline", wantCode: http.StatusOK},
		{path: "/debug/pprof/symbol", wantCode: http.StatusOK, wantBody: "num_symbols: 1"},
		{path: "/unregistered", wantCode: http.StatusNotFound, wantBody: "404 page not found"},
	} {
		t.Run(tt.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, tt.path, nil))
			if response.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", response.Code, tt.wantCode)
			}
			if !strings.Contains(response.Body.String(), tt.wantBody) {
				t.Errorf("body = %q, want substring %q", response.Body.String(), tt.wantBody)
			}
			if tt.path == "/debug/pprof/cmdline" && response.Body.Len() == 0 {
				t.Error("cmdline handler returned an empty body")
			}
		})
	}
}
