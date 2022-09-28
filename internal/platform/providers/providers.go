package providers

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/gregjones/httpcache"
	"github.com/gregjones/httpcache/diskcache"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/wyattjoh/ims/internal/image/provider"
)

// Providers provide other Provider's based on the host name of the request.
type Providers struct {
	providers map[string]provider.Provider
}

// Get will return a provider.
func (p *Providers) Get(host string) provider.Provider {
	provider := p.providers[host]
	return provider
}

// NewProviders will return the Providers wrapped.
func NewProviders(providers map[string]provider.Provider) *Providers {
	return &Providers{
		providers: providers,
	}
}

// =============================================================================

// GetUnderlyingTransport checks the url as some clients have specific
// requirements of the underlying transport.
func GetUnderlyingTransport(ctx context.Context, originURL *url.URL) (http.RoundTripper, error) {
	switch originURL.Scheme {
	case "gs":
		transport, err := provider.NewGCSTransport(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "could not create transport for scheme")
		}

		return transport, nil
	default:
		return http.DefaultTransport, nil
	}
}

// WrapCacheRoundTripper gets the roundtripper if the provider is a remote
// backend type.
func WrapCacheRoundTripper(ctx context.Context, underlyingTransport http.RoundTripper, originCache string) (http.RoundTripper, error) {
	switch originCache {
	case ":memory:":
		// Create the memory cache transport, and add the underlying transport to
		// it.
		mct := httpcache.NewMemoryCacheTransport()
		mct.Transport = underlyingTransport

		logrus.WithField("transport", ":memory:").Debug("origin cache enabled")

		return mct, nil

	case "":
		// No cache was specified, fall back to the underlying transport.
		logrus.Debug("origin cache disabled")

		return underlyingTransport, nil

	default:
		// Create a new disk transport cache.
		ct := httpcache.NewTransport(diskcache.New(originCache))
		ct.Transport = underlyingTransport

		logrus.WithField("transport", originCache).Debug("origin cache enabled")

		return ct, nil
	}
}

// GetRemoteProviderClient gets the remote provider client or errors.
func GetRemoteProviderClient(ctx context.Context, originURL *url.URL, transport http.RoundTripper) (provider.Provider, error) {
	switch originURL.Scheme {
	case "gs":
		transport, err := provider.NewGCS(ctx, originURL.Host, transport)
		if err != nil {
			return nil, errors.Wrap(err, "could not create provider for the gs scheme")
		}

		return transport, nil
	case "s3":
		transport, err := provider.NewS3(originURL.Host, transport)
		if err != nil {
			return nil, errors.Wrap(err, "could not create provider for the s3 scheme")
		}

		return transport, nil
	case "http", "https":
		return provider.NewOrigin(originURL, transport), nil
	default:
		return nil, errors.New("invalid origin url provided, scheme could not be matched to an available provider")
	}
}

// GetRemoteBackendProvider will get the backend provider based on the scheme of
// the url.
func GetRemoteBackendProvider(ctx context.Context, origin, originCache string) (provider.Provider, error) {
	originURL, err := url.Parse(origin)
	if err != nil {
		return nil, errors.Wrap(err, "cannot parse the origin url")
	}

	// Get the underlying transport to use to fetch the original resource.
	underlyingTransport, err := GetUnderlyingTransport(ctx, originURL)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get the underlying transport")
	}

	transport, err := WrapCacheRoundTripper(ctx, underlyingTransport, originCache)
	if err != nil {
		return nil, errors.Wrap(err, "can't get the origin round tripper")
	}

	// Get the remote provider client.
	return GetRemoteProviderClient(ctx, originURL, transport)
}

// GetProxyBackendProvider will create a new proxy provider.
func GetProxyBackendProvider(ctx context.Context, originCache string) (provider.Provider, error) {
	transport, err := WrapCacheRoundTripper(ctx, http.DefaultTransport, originCache)
	if err != nil {
		return nil, errors.Wrap(err, "can't get the origin round tripper")
	}

	// Get the remote provider client.
	return provider.NewProxy(transport), nil
}

// ParseBackend parses the backend using the following formats:
//
//	<host>,<origin> OR <origin>
//
// Where if the host is not specified, it falls back to the defaultHost.
func ParseBackend(defaultHost, backend string) (string, string, error) {
	if backend == "" || backend == "," || strings.HasPrefix(backend, ",") || strings.HasSuffix(backend, ",") {
		return "", "", errors.New("cannot be blank")
	}

	backendSplit := strings.Split(backend, ",")

	if len(backendSplit) == 2 {
		return backendSplit[0], backendSplit[1], nil
	} else if len(backendSplit) == 1 {
		return defaultHost, backendSplit[0], nil
	}

	return "", "", errors.New("expected form <host>,<origin> OR <origin>")
}

// New loops over the origins provided, parsing with the specified providers,
// and returns the providers keyed by host and optionally wrapped with an origin
// cache. This will error if the same backend host is extracted more than once.
func New(ctx context.Context, defaultHost string, backends []string, originCache, signingSecret string, signingWithPath bool) (*Providers, error) {
	if len(backends) == 0 {
		return nil, errors.New("no provider selected")
	}

	// Collect all the providers to the map of Host -> provider.Provider.
	providers := make(map[string]provider.Provider)

	for _, backend := range backends {
		host, origin, err := ParseBackend(defaultHost, backend)
		if err != nil {
			return nil, errors.Wrap(err, "cannot parse the backend")
		}

		// Check to see if we've already attached this host.
		if _, ok := providers[host]; ok {
			return nil, errors.Errorf("host %s already has a provider attached to it", host)
		}

		// If the origin contains the "://" then it must be a remote provider, so
		// attempt to create a origin provider.
		if strings.Contains(origin, "://") {
			// This looks a little weird because we aren't passing a constructed
			// origin cache down to the provider that is shared, but each provider
			// will have a different http.RoundTripper anyways, so no need to reuse
			// the cache in the same way.
			p, err := GetRemoteBackendProvider(ctx, origin, originCache)
			if err != nil {
				return nil, errors.Wrap(err, "cannot get the origin provider")
			}

			logrus.WithFields(logrus.Fields{
				"host":   host,
				"origin": origin,
			}).Debug("serving from the origin")

			providers[host] = p
		} else if origin == ":proxy:" {
			// This looks like a proxy! Let's create the provider.
			p, err := GetProxyBackendProvider(ctx, originCache)
			if err != nil {
				return nil, errors.Wrap(err, "cannot get the proxy provider")
			}

			// Ensure that if the proxy backend is enabled, that the signing config is
			// also provided.
			if signingSecret == "" {
				return nil, errors.New("when proxy mode is enabled, a signing secret is required")
			}

			if !signingWithPath {
				return nil, errors.New("when proxy mode is enabled, a signing with path is required")
			}

			logrus.WithFields(logrus.Fields{
				"host": host,
			}).Debug("serving with proxy mode")
			providers[host] = p
		} else {
			logrus.WithFields(logrus.Fields{
				"host":      host,
				"directory": origin,
			}).Debug("serving from the filesystem")
			providers[host] = &provider.Filesystem{Dir: http.Dir(origin)}
		}
	}

	return NewProviders(providers), nil
}
