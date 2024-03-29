package app

import (
	"context"
	"net/http"
	"time"

	negronilogrus "github.com/meatballhat/negroni-logrus"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/negroni"
	"github.com/wyattjoh/ims/cmd/ims/handlers"
	"github.com/wyattjoh/ims/internal/platform/providers"
	"github.com/wyattjoh/ims/internal/platform/signing"
)

// MountEndpoint mounts an endpoint on the mux and logs out the action.
func MountEndpoint(mux *http.ServeMux, endpoint string, handler http.Handler) {
	mux.Handle(endpoint, handler)
	logrus.WithField("endpoint", endpoint).Debug("mounting endpoint")
}

// ServerOpts is the options for starting a new Server.
type ServerOpts struct {
	// Addr is the address to listen for http requests on.
	Addr string

	// Debug enables pprof endpoints and debug logs.
	Debug bool

	// DisableMetrics disables Prometheus endpoints.
	DisableMetrics bool

	// Backends is the comma separated <host>,<origin> where <origin> is a pathname
	// or a url (with scheme) to load images from.
	Backends []string

	// OriginCache is the reference to the cache source for origin based
	// backends.
	OriginCache string

	// CacheTimeout is the time that images will have cache headers for when
	// writing them out to the http response.
	CacheTimeout time.Duration

	// CORSDomains are the permitted domains that will be permitted to make
	// CORS requests from.
	CORSDomains []string

	// SigningSecret is used to mount a signing middleware on the image
	// processing domain to only allow signed requests through.
	SigningSecret string

	// IncludePath when true will add the path component to the signing value
	// when request signing has been enabled.
	IncludePath bool
}

// Serve creates and starts a new server to provide image resizing services.
func Serve(opts *ServerOpts) error {
	// Create the context that will manage the state for the request.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mux := http.NewServeMux()

	if opts.CacheTimeout != 0 {
		logrus.WithField("timeout", opts.CacheTimeout.String()).Debug("cache headers enabled")
	} else {
		logrus.Debug("cache headers disabled")
	}

	// When debug mode is enabled, mount the debug handlers on this router.
	if opts.Debug {
		MountDebug(mux)
	}

	// Get the image provider map.
	p, err := providers.New(ctx, opts.Addr, opts.Backends, opts.OriginCache, opts.SigningSecret, opts.IncludePath)
	if err != nil {
		return errors.Wrap(err, "cannot create providers")
	}

	// Wrap the handler with the image handler and the providers.
	handler := providers.Middleware(p, handlers.Image(opts.CacheTimeout))

	if opts.SigningSecret != "" {
		// Wrap the handler with the signing middleware when we have a secret
		// for signing provided.
		handler = signing.Middleware(opts.SigningSecret, opts.IncludePath, handler)

		logrus.WithField("withPath", opts.IncludePath).Debug("signing middleware enabled")
	} else {
		logrus.Debug("signing middleware disabled, --signing-secret not provided")
	}

	if opts.DisableMetrics {
		// Mount the resize handler on the mux.
		MountEndpoint(mux, "/", handler)

		logrus.Debug("prometheus metrics disabled")
	} else {
		// Mount the resize handler on the mux with the instrumentation wrapped on
		// the handler.
		MountEndpoint(mux, "/", promhttp.InstrumentMetricHandler(prometheus.DefaultRegisterer, handler))

		// Register the prometheus metrics handler.
		MountEndpoint(mux, "/metrics", promhttp.Handler())

		logrus.Debug("prometheus metrics enabled")
	}

	// Create the negroni middleware bundle.
	n := negroni.New(
		negroni.NewRecovery(),
		negroni.HandlerFunc(Tracer),
		negronilogrus.NewMiddleware(),
	)

	if len(opts.CORSDomains) > 0 {
		// Mount the CORS middleware if it was enabled.
		n.Use(cors.New(cors.Options{
			AllowedOrigins: opts.CORSDomains,
			AllowedMethods: []string{"GET"},
		}))
	}

	// Attach the mux to the middleware handler.
	n.UseHandler(mux)

	logrus.WithField("address", opts.Addr).Info("now listening")

	if err := http.ListenAndServe(opts.Addr, n); err != nil {
		return errors.Wrap(err, "could not listen on the address for http traffic")
	}

	return nil
}
