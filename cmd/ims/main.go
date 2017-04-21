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
	"github.com/urfave/negroni"
	"github.com/wyattjoh/ims/cmd/ims/routes"
)

// Serve creates and starts a new server to provide image resizing services.
func Serve(addr string, debug bool, directory, origin string, timeout time.Duration) error {
	mux := http.NewServeMux()

	if timeout != 0 {
		logrus.WithField("timeout", timeout.String()).Debug("cache headers enabled")
	} else {
		logrus.Debug("cache headers disabled")
	}

	// When debug mode is enabled, mount the debug handlers on this router.
	if debug {
		mux.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
		mux.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
		mux.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
		mux.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
		mux.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))
	}

	// By default, we'll try to use the directory resize, otherwise, if the origin
	// url is provided, use it.
	if origin == "" {
		logrus.WithField("directory", directory).Debug("serving from the filesystem")
		mux.HandleFunc("/", routes.FileSystemResize(timeout, http.Dir(directory)))
	} else {
		logrus.WithField("origin", origin).Debug("serving from the origin")

		originURL, err := url.Parse(origin)
		if err != nil {
			return errors.Wrap(err, "can't parse the origin url")
		}

		mux.HandleFunc("/", routes.OriginResize(timeout, originURL))
	}

	// Create the negroni middleware bundle.
	n := negroni.New(negroni.NewRecovery(), negronilogrus.NewMiddleware())

	// Attach the mux.
	n.UseHandler(mux)

	logrus.WithField("address", addr).Info("Now listening")
	return http.ListenAndServe(addr, n)
}

func main() {
	var (
		debug           = flag.Bool("debug", false, "enable debug logging and pprof routes")
		listenAddr      = flag.String("listen-addr", "0.0.0.0:8080", "the address to listen for new connections on")
		imagesDirectory = flag.String("images-dir", "images", "the location on the filesystem to load images from")
		originURL       = flag.String("origin-url", "", "url for the origin server to pull images from")
		timeout         = flag.Duration("timeout", 15*time.Minute, "used to set the cache control max age headers, set to 0 to disable")
	)

	flag.Parse()

	if *debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	if err := Serve(*listenAddr, *debug, *imagesDirectory, *originURL, *timeout); err != nil {
		logrus.Fatalf("Could not serve: %s", err)
	}
}
