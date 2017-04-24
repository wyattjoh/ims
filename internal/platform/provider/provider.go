package provider

import (
	"context"
	"net/http"
	"net/url"

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

	// If the cache is enabled, we need to switch in the cache'd transport.
	transport := http.DefaultTransport
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
		return transport, nil
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
		return nil, errors.Wrap(err, "can't parse the origin url")
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
	default:
		return provider.NewOrigin(originURL, transport), nil
	}
}

// Get gets the image provider to use for the resize handler. If the origin is
// not provided, it defaults to the filesysytem provider with the specified
// directory.
func Get(ctx context.Context, directory, origin, originCache string) (provider.Provider, error) {
	if origin == "" && directory == "" {
		return nil, errors.New("no provider selected")
	}

	// By default, we'll try to use the directory resize, otherwise, if the origin
	// url is provided, use it.
	if origin == "" {
		logrus.WithField("directory", directory).Debug("serving from the filesystem")

		return &provider.Filesystem{Dir: http.Dir(directory)}, nil
	}

	logrus.WithField("origin", origin).Debug("serving from the origin")
	return GetOriginProvider(ctx, origin, originCache)
}
