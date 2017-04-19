package main

import (
	"flag"

	"github.com/Sirupsen/logrus"
	ims "github.com/wyattjoh/ims/lib"
)

func main() {
	var (
		debug           = flag.Bool("debug", false, "enable debug mode")
		listenAddr      = flag.String("listen-addr", "0.0.0.0:8080", "the address to listen for new connections on")
		imagesDirectory = flag.String("images-dir", "images", "the location on the filesystem to load images from")
		originURL       = flag.String("origin-url", "", "url for the origin server to pull images from")
	)

	flag.Parse()

	if *debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	if err := ims.Serve(*listenAddr, *debug, *imagesDirectory, *originURL); err != nil {
		logrus.Fatalf("Could not serve: %s", err)
	}
}
