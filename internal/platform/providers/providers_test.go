package providers_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/gregjones/httpcache"
	"github.com/wyattjoh/ims/internal/image/provider"
	"github.com/wyattjoh/ims/internal/platform/providers"
)

func TestMiddleware(t *testing.T) {
	tableData := map[string]provider.Provider{
		"1.com": provider.NewOrigin(nil, nil),
		"2.com": provider.NewOrigin(nil, nil),
	}

	// We need to check that the middleware is setting the right provider for the
	// given host.
	for host, tableCaseProvider := range tableData {
		providers.Middleware(tableData, func(w http.ResponseWriter, r *http.Request) {
			p, ok := r.Context().Value(providers.ContextKey).(provider.Provider)
			if !ok {
				t.Fatalf("Expected case %s to find a provider, could not", host)
			}

			if p != tableCaseProvider {
				t.Errorf("Expected case %s to have a provider %v, it had %v instead", host, tableCaseProvider, p)
			} else {
				t.Logf("Expected case %s to have a provider %v, it did", host, tableCaseProvider)
			}
		})(nil, &http.Request{
			Host: host,
		})
	}
}

func TestWrapCacheRoundTripper(t *testing.T) {
	ut := http.DefaultTransport
	tableData := []struct {
		OriginCache      string
		DefaultTransport bool
		CacheTransport   bool
	}{
		{
			OriginCache:    ":memory:",
			CacheTransport: true,
		},
		{
			DefaultTransport: true,
		},
		{
			OriginCache:    "cache",
			CacheTransport: true,
		},
	}

	ctx := context.Background()
	for i, tableCase := range tableData {
		rt, err := providers.WrapCacheRoundTripper(ctx, ut, tableCase.OriginCache)
		if err != nil {
			t.Fatalf("Expected no error from case %d, got %s", i, err.Error())
		}

		if tableCase.DefaultTransport {
			if rt != ut {
				t.Errorf("Expected case %d to match default transport, it did not", i)
			} else {
				t.Logf("Expected case %d to match default transport, it did", i)
			}
		} else {
			if rt == ut {
				t.Errorf("Expected case %d to not match default transport, it did", i)
			} else {
				t.Logf("Expected case %d to not match default transport, it did not", i)
			}
		}

		if tableCase.CacheTransport {
			if _, ok := rt.(*httpcache.Transport); !ok {
				t.Errorf("Expected case %d to match *httpcache.Transport, it did not", i)
			} else {
				t.Logf("Expected case %d to match *httpcache.Transport, it did", i)
			}
		} else {
			if _, ok := rt.(*httpcache.Transport); ok {
				t.Errorf("Expected case %d to not match *httpcache.Transport, it did", i)
			} else {
				t.Logf("Expected case %d to not match *httpcache.Transport, it did not", i)
			}
		}
	}
}

func TestParseBackend(t *testing.T) {
	defaultHost := "127.0.0.1:8080"
	tableData := []struct {
		Backend      string
		Error        bool
		Host, Origin string
	}{
		{
			Backend: "s3://super-cool-bucket",
			Host:    defaultHost,
			Origin:  "s3://super-cool-bucket",
		},
		{
			Backend: "host,origin",
			Host:    "host",
			Origin:  "origin",
		},
		{
			Backend: "",
			Error:   true,
		},
		{
			Backend: "a,",
			Error:   true,
		},
		{
			Backend: ",a",
			Error:   true,
		},
		{
			Backend: "host,origin,somethingelse?",
			Error:   true,
		},
	}

	for i, tableCase := range tableData {
		host, origin, err := providers.ParseBackend(defaultHost, tableCase.Backend)
		if err != nil {
			if tableCase.Error {
				t.Logf("Expected case %d to error, it did", i)
				continue
			} else {
				t.Errorf("Expected case %d to not error, it did: %s", i, err.Error())
				continue
			}
		} else {
			if tableCase.Error {
				t.Errorf("Expected case %d to error, it did not", i)
				continue
			} else {
				t.Logf("Expected case %d to not error, it did not", i)
			}
		}

		if host != tableCase.Host {
			t.Errorf("Expected case %d to have host %s, got %s", i, tableCase.Host, host)
		} else {
			t.Logf("Expected case %d to have host %s, it did", i, tableCase.Host)
		}

		if origin != tableCase.Origin {
			t.Errorf("Expected case %d to have origin %s, got %s", i, tableCase.Origin, origin)
		} else {
			t.Logf("Expected case %d to have origin %s, it did", i, tableCase.Origin)
		}
	}
}
