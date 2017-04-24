package provider

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/gregjones/httpcache"
	"github.com/gregjones/httpcache/diskcache"
	"github.com/pkg/errors"
	"github.com/wyattjoh/ims/internal/image/provider"
)

// GetUnderlyingTransport checks the url as some clients have specific
// requirements of the underlying transport.
func GetUnderlyingTransport(ctx context.Context, originURL *url.URL) (http.RoundTripper, error) {
	switch originURL.Scheme {
	case "gs":
		return provider.NewGCSTransport(ctx)
	default:
		return http.DefaultTransport, nil
	}
}

// GetOriginRoundTripper gets the roundtripper if the provider is a remote
// origin type.
func GetOriginRoundTripper(ctx context.Context, underlyingTrasport http.RoundTripper, originCache string) (http.RoundTripper, error) {
	switch originCache {
	case ":memory:":

		// Create the memory cache transport, and add the underlying transport to
		// it.
		mct := httpcache.NewMemoryCacheTransport()
		mct.Transport = underlyingTrasport

		logrus.WithField("transport", ":memory:").Debug("origin cache enabled")
		return mct, nil
	case "":
		logrus.Debug("origin cache disabled")
		return underlyingTrasport, nil
	default:

		// Create a new disk transport cache.
		ct := httpcache.NewTransport(diskcache.New(originCache))
		ct.Transport = underlyingTrasport

		logrus.WithField("transport", originCache).Debug("origin cache enabled")
		return ct, nil
	}
}

// GetOriginProvider will get the origin provider based on the scheme of the
// url.
func GetOriginProvider(ctx context.Context, origin, originCache string) (provider.Provider, error) {
	originURL, err := url.Parse(origin)
	if err != nil {
		return nil, errors.Wrap(err, "cannot parse the origin url")
	}

	underlyingTrasport, err := GetUnderlyingTransport(ctx, originURL)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get the underlying transport")
	}

	transport, err := GetOriginRoundTripper(ctx, underlyingTrasport, originCache)
	if err != nil {
		return nil, errors.Wrap(err, "can't get the origin round tripper")
	}

	switch originURL.Scheme {
	case "gs":
		return provider.NewGCS(ctx, originURL.Host, transport)
	case "s3":
		return provider.NewS3(originURL.Host, transport)
	case "http", "https":
		return provider.NewOrigin(originURL, transport), nil
	default:
		return nil, errors.New("invalid origin url provided, scheme could not be matched to an available provider")
	}
}

// New loops over the origins provided, parsing with the specified providers,
// and returns the providers keyed by host and optionally wrapped with an origin
// cache.
func New(ctx context.Context, defaultHost string, backends []string, originCache string) (map[string]provider.Provider, error) {
	if len(backends) == 0 {
		return nil, errors.New("no provider selected")
	}

	// Collect all the providers to the map of Host -> provider.Provider.
	providers := make(map[string]provider.Provider)
	for _, originString := range backends {
		originSplit := strings.Split(originString, ",")

		var host, origin string

		// We always expect that the origin should arrive in the form of:
		//   <host>,<origin> OR <origin>
		// So if we don't get it, ensure that we do error out.
		if len(originSplit) == 2 {
			host = originSplit[0]
			origin = originSplit[1]
		} else if len(originSplit) == 1 {
			host = defaultHost
			origin = originSplit[0]
		} else {
			return nil, errors.New("origin format invalid, expected form <host>,<origin> OR <origin>")
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
			p, err := GetOriginProvider(ctx, origin, originCache)
			if err != nil {
				return nil, errors.Wrap(err, "cannot get the origin provider")
			}

			logrus.WithFields(logrus.Fields{
				"host":   host,
				"origin": origin,
			}).Debug("serving from the origin")
			providers[host] = p

		} else {

			logrus.WithFields(logrus.Fields{
				"host":      host,
				"directory": origin,
			}).Debug("serving from the filesystem")
			providers[host] = &provider.Filesystem{Dir: http.Dir(origin)}

		}
	}

	return providers, nil
}
