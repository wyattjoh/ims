package main

import (
	"os"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
	"github.com/wyattjoh/ims/cmd/ims/app"
)

func main() {
	app := cli.NewApp()
	app.Name = "ims"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "listen-addr",
			Value: "0.0.0.0:8080",
			Usage: "the address to listen for new connections on",
		},
		cli.BoolFlag{
			Name:  "debug",
			Usage: "enable debug logging and pprof routes",
		},
		cli.StringFlag{
			Name:  "images-dir",
			Usage: "the location on the filesystem to load images from",
		},
		cli.StringFlag{
			Name:  "origin-url",
			Usage: "url for the origin server to pull images from",
		},
		cli.StringFlag{
			Name:  "origin-cache",
			Usage: "cache the origin resources based on their cache headers (:memory: for memory based cache, directory name for file based, not specified for disabled)",
		},
		cli.BoolFlag{
			Name:  "disable-metrics",
			Usage: "disable the prometheus metrics",
		},
		cli.DurationFlag{
			Name:  "timeout",
			Value: 15 * time.Minute,
			Usage: "used to set the cache control max age headers, set to 0 to disable",
		},
	}
	app.Action = ServeAction

	app.Run(os.Args)
}

// ServeAction starts the ims daemon.
func ServeAction(c *cli.Context) error {

	// We want to enable debug logging as soon as we know that we're in debug
	// mode.
	if c.Bool("debug") {
		logrus.SetLevel(logrus.DebugLevel)
	}

	// Setup the server options.
	opts := &app.ServerOpts{
		Addr:           c.String("listen-addr"),
		Debug:          c.Bool("debug"),
		DisableMetrics: c.Bool("disable-metrics"),
		Directory:      c.String("images-dir"),
		Origin:         c.String("origin-url"),
		OriginCache:    c.String("origin-cache"),
		CacheTimeout:   c.Duration("timeout"),
	}

	if err := app.Serve(opts); err != nil {
		logrus.WithError(err).Error("could not serve")
		return cli.NewExitError(err.Error(), 1)
	}

	return nil
}
