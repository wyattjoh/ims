package main

import (
	"flag"

	"github.com/Sirupsen/logrus"
	ims "github.com/wyattjoh/ims/lib"
)

func main() {
	var debug = flag.Bool("debug", false, "enable debug mode")
	var listenAddr = flag.String("listen-addr", "0.0.0.0:8080", "the address to listen for new connections on")

	flag.Parse()

	if *debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	if err := ims.Serve(*listenAddr, *debug); err != nil {
		logrus.Fatalf("Could not serve: %s", err)
	}
}
