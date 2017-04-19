package main

import (
	"flag"
	"time"

	"github.com/Sirupsen/logrus"
	ims "github.com/wyattjoh/ims/lib"
)

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

	if err := ims.Serve(*listenAddr, *debug, *imagesDirectory, *originURL, *timeout); err != nil {
		logrus.Fatalf("Could not serve: %s", err)
	}
}
