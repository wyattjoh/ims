package main

import (
	"flag"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/wyattjoh/ims/cmd/ims/app"
)

func main() {
	var (
		debug           = flag.Bool("debug", false, "enable debug logging and pprof routes")
		listenAddr      = flag.String("listen-addr", "0.0.0.0:8080", "the address to listen for new connections on")
		imagesDirectory = flag.String("images-dir", "images", "the location on the filesystem to load images from")
		originURL       = flag.String("origin-url", "", "url for the origin server to pull images from")
		originCache     = flag.String("origin-cache", "", "cache the origin resources based on their cache headers (:memory: for memory based cache, directory name for file based, not specified for disabled)")
		timeout         = flag.Duration("timeout", 15*time.Minute, "used to set the cache control max age headers, set to 0 to disable")
		disableMetrics  = flag.Bool("disable-metrics", false, "disable the prometheus metrics")
	)

	flag.Parse()

	// We want to enable debug logging as soon as we know that we're in debug
	// mode.
	if *debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	// Setup the server options.
	opts := &app.ServerOpts{
		Addr:           *listenAddr,
		Debug:          *debug,
		DisableMetrics: *disableMetrics,
		Directory:      *imagesDirectory,
		Origin:         *originURL,
		OriginCache:    *originCache,
		CacheTimeout:   *timeout,
	}

	if err := app.Serve(opts); err != nil {
		logrus.WithError(err).Fatalf("could not serve")
	}
}
