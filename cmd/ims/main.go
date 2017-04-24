package main

import (
	"flag"
	"net/http"
	"net/http/pprof"
	"net/url"
	"time"

	"github.com/Sirupsen/logrus"
	negronilogrus "github.com/meatballhat/negroni-logrus"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/urfave/negroni"
	"github.com/wyattjoh/ims/cmd/ims/routes"
	"github.com/wyattjoh/ims/internal/image/provider"
)

// GetProvider gets the image provider to use for the resize handler. If the
// origin is not provided, it defaults to the filesysytem provider with the
// specified directory.
func GetProvider(directory, origin string) (provider.Provider, error) {

	// By default, we'll try to use the directory resize, otherwise, if the origin
	// url is provided, use it.
	if origin == "" {
		logrus.WithField("directory", directory).Debug("serving from the filesystem")

		return provider.Filesystem{Dir: http.Dir(directory)}, nil
	}

	logrus.WithField("origin", origin).Debug("serving from the origin")

	originURL, err := url.Parse(origin)
	if err != nil {
		return nil, errors.Wrap(err, "can't parse the origin url")
	}

	return provider.Origin{URL: originURL}, nil

}

// ServerOpts is the options for starting a new Server,
type ServerOpts struct {

	// Addr is the address to listen for http requests on.
	Addr string

	// Debug enables pprof endpoints and debug logs.
	Debug bool

	// DisableMetrics disables Prometheus endpoints.
	DisableMetrics bool

	// Directory is the folder in which images are served out of.
	Directory string

	// Origin is the url that is the base url for images and will act as the
	// provider.
	Origin string

	// CacheTimeout is the time that images will have cache headers for when
	// writing them out to the http response.
	CacheTimeout time.Duration
}

// Serve creates and starts a new server to provide image resizing services.
func Serve(opts *ServerOpts) error {
	mux := http.NewServeMux()

	if opts.CacheTimeout != 0 {
		logrus.WithField("timeout", opts.CacheTimeout.String()).Debug("cache headers enabled")
	} else {
		logrus.Debug("cache headers disabled")
	}

	// When debug mode is enabled, mount the debug handlers on this router.
	if opts.Debug {
		mux.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
		mux.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
		mux.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
		mux.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
		mux.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))
	}

	// Get the image provider.
	p, err := GetProvider(opts.Directory, opts.Origin)
	if err != nil {
		return err
	}

	if !opts.DisableMetrics {

		// Mount the resize handler on the mux with the instrumentation wrapped on
		// the handler.
		mux.HandleFunc("/", prometheus.InstrumentHandler("image", routes.Resize(opts.CacheTimeout, p)))

		// Register the prometheus metrics handler.
		mux.Handle("/metrics", prometheus.Handler())
	} else {

		// Mount the resize handler on the mux.
		mux.HandleFunc("/", routes.Resize(opts.CacheTimeout, p))
	}

	// Create the negroni middleware bundle.
	n := negroni.New(negroni.NewRecovery(), negronilogrus.NewMiddleware())

	// Attach the mux.
	n.UseHandler(mux)

	logrus.WithField("address", opts.Addr).Info("Now listening")
	return http.ListenAndServe(opts.Addr, n)
}

func main() {
	var (
		debug           = flag.Bool("debug", false, "enable debug logging and pprof routes")
		listenAddr      = flag.String("listen-addr", "0.0.0.0:8080", "the address to listen for new connections on")
		imagesDirectory = flag.String("images-dir", "images", "the location on the filesystem to load images from")
		originURL       = flag.String("origin-url", "", "url for the origin server to pull images from")
		timeout         = flag.Duration("timeout", 15*time.Minute, "used to set the cache control max age headers, set to 0 to disable")
		disableMetrics  = flag.Bool("disable-metrics", false, "disable the prometheus metrics")
	)

	flag.Parse()

	if *debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	// Setup the server options.
	opts := &ServerOpts{
		Addr:           *listenAddr,
		Debug:          *debug,
		DisableMetrics: *disableMetrics,
		Directory:      *imagesDirectory,
		Origin:         *originURL,
		CacheTimeout:   *timeout,
	}

	if err := Serve(opts); err != nil {
		logrus.Fatalf("Could not serve: %s", err)
	}
}
